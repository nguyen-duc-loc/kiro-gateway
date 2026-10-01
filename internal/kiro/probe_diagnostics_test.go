package kiro

import (
	"context"
	"errors"
	"net"
	"net/http"
)

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
		"error_body_policy":      "never_read_or_output",
	}
}
