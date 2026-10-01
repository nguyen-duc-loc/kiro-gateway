package kiro

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"

	"kiro-gateway/internal/jsonobject"
)

const probeErrorBodyLimit = 16 << 10
const probeErrorScratch = 64*probeErrorBodyLimit + (64 << 10)

func probeServiceErrorTypes() map[string]string {
	return map[string]string{
		"AccessDeniedError": "access_denied", "AccessDeniedException": "access_denied",
		"MissingAuthenticationTokenException": "missing_authentication_token",
		"InternalServerError":                 "internal_server_error",
		"ServiceQuotaExceededError":           "service_quota_exceeded",
		"ThrottlingError":                     "throttling",
		"ServiceUnavailableException":         "service_unavailable",
	}
}

func probeNormalizeError(value string) string {
	if len(value) == 0 || len(value) > 256 {
		return ""
	}
	for i := range len(value) {
		if value[i] < 33 || value[i] > 126 {
			return ""
		}
	}
	value, _, _ = strings.Cut(value, ":")
	if _, tail, found := strings.Cut(value, "#"); found {
		value = tail
	}
	return value
}

func probeServiceErrorLabel(value string) string {
	if label, known := probeServiceErrorTypes()[value]; known {
		return label
	}
	return "unknown"
}

func probeErrorFormat(headers http.Header) string {
	values := headers.Values("Content-Type")
	if len(values) == 0 {
		return "absent"
	}
	if len(values) != 1 {
		return "ambiguous"
	}
	media, _, err := mime.ParseMediaType(values[0])
	if err != nil {
		return "other"
	}
	switch media {
	case "application/json", "application/x-amz-json-1.0", "application/x-amz-json-1.1":
		return "json"
	case "text/html":
		return "html"
	default:
		return "other"
	}
}

// Classify only a finite discriminator. Raw headers, body bytes, and message
// fields are scoped to this call and never enter the result or an error string.
func probeServiceError(headers http.Header, body io.Reader) (label, format string, received int64) {
	format = probeErrorFormat(headers)
	values := headers.Values("X-Amzn-Errortype")
	if len(values) > 1 {
		return "ambiguous", format, 0
	}
	if len(values) == 1 {
		return probeServiceErrorLabel(probeNormalizeError(values[0])), format, 0
	}
	if format != "json" {
		return "absent", format, 0
	}
	b, err := io.ReadAll(io.LimitReader(body, probeErrorBodyLimit+1))
	received = int64(len(b))
	if err != nil {
		return "unavailable", format, received
	}
	if len(b) > probeErrorBodyLimit {
		return "oversized", format, received
	}
	o, err := jsonobject.Parse(b, probeErrorBodyLimit)
	if err != nil {
		return "unparseable", format, received
	}
	var selected string
	present := false
	for _, key := range []string{"code", "__type"} {
		raw, exists := o[key]
		if !exists {
			continue
		}
		var value *string
		if json.Unmarshal(raw, &value) != nil || value == nil {
			return "unparseable", format, received
		}
		normalized := probeNormalizeError(*value)
		if normalized == "" {
			return "unknown", format, received
		}
		if present && normalized != selected {
			return "ambiguous", format, received
		}
		selected, present = normalized, true
	}
	if !present {
		return "absent", format, received
	}
	return probeServiceErrorLabel(selected), format, received
}

// Raw network errors remain private. Only these fixed stage labels may leave
// the transport; Unwrap preserves timeout and cancellation classification.
type probeDialFailure struct {
	stage string
	cause error
}

func (e *probeDialFailure) Error() string { return "transport_failed" }
func (e *probeDialFailure) Unwrap() error { return e.cause }

func probeTransportLabel(err error, tlsFailed bool) string {
	var failure *probeDialFailure
	if errors.As(err, &failure) {
		switch failure.stage {
		case "dns", "connect", "destination_policy":
			return failure.stage
		}
	}
	if tlsFailed {
		return "tls"
	}
	var timeout net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &timeout) && timeout.Timeout() {
		return "timeout"
	}
	return "other"
}

func probeHTTPLabel(status int) string {
	switch status {
	case http.StatusOK:
		return "ok"
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusTooManyRequests:
		return "throttled"
	}
	if status >= 300 && status < 400 {
		return "redirect"
	}
	if status >= 500 && status < 600 {
		return "server_error"
	}
	return "other"
}

func probeDiagnosticPolicy() map[string]any {
	return map[string]any{
		"failure_stages":         []string{"pre_dispatch", "source", "request_build", "transport", "http_status", "response_headers", "stream", "cleanup"},
		"transport_failures":     []string{"dns", "connect", "destination_policy", "tls", "timeout", "other"},
		"http_status_categories": []string{"ok", "bad_request", "unauthorized", "forbidden", "not_found", "throttled", "redirect", "server_error", "other"},
		"error_body_policy":      "bounded_json_error_type_only",
		"error_body_limit":       probeErrorBodyLimit,
		"error_type_header":      "X-Amzn-Errortype",
		"error_type_body_fields": []string{"code", "__type"},
		"known_error_types":      probeServiceErrorTypes(),
		"error_type_fallbacks":   []string{"absent", "unknown", "ambiguous", "unparseable", "oversized", "unavailable"},
		"error_response_formats": []string{"json", "html", "other", "absent", "ambiguous"},
	}
}
