//go:build responsediscovery

package kiro

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"io"
	"mime"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"
)

const responseBodyLimit = 256 << 10
const responseReportLimit = 16 << 10
const responseHypothesis = "createresponse_awsjson_v1"
const responseTarget = "KiroRuntimeService.CreateResponse"

func responsePtr[T any](v T) *T { return &v }
func responsePaths() []string {
	return strings.Fields(`/type /status /model /output /usage /usage/input_tokens /usage/output_tokens /response /response/status /response/model /response/output /response/usage /response/usage/input_tokens /response/usage/output_tokens /error /error/type /error/code /code /__type`)
}
func responseLabels() []string {
	return strings.Fields(`message error response.created response.queued response.in_progress response.completed response.incomplete response.failed response.output_item.added response.output_item.done response.content_part.added response.content_part.done response.output_text.delta response.output_text.done response.function_call_arguments.delta response.function_call_arguments.done assistantResponseEvent toolUseEvent reasoningContentEvent metadataEvent messageMetadataEvent`)
}
func responseKinds() []string {
	return strings.Fields(`absent unreachable null boolean number string array object`)
}
func responseStatuses() []string {
	return strings.Fields(`queued in_progress completed incomplete failed cancelled other`)
}
func responseErrors() []string {
	return strings.Fields(`AccessDeniedException ValidationException UnknownOperationException ResourceNotFoundException ThrottlingException InternalServerException ServiceUnavailableException ServiceQuotaExceededException other`)
}

type responseRecord struct {
	Channel        string  `json:"channel"`
	MessageKind    *string `json:"message_kind"`
	TransportLabel *string `json:"transport_label"`
	PayloadType    *string `json:"payload_type"`
	DoneMarker     bool    `json:"done_marker"`
}
type responseSummary struct {
	RecordCount       int                 `json:"record_count"`
	JSONRecordCount   int                 `json:"json_record_count"`
	EventSequence     []responseRecord    `json:"event_sequence"`
	SequenceTruncated bool                `json:"sequence_truncated"`
	PathKinds         map[string][]string `json:"path_kinds"`
	StatusLabels      []string            `json:"status_labels"`
	ErrorLabels       []string            `json:"error_labels"`
	ModelComparisons  []string            `json:"model_comparisons"`
}
type responseReport struct {
	ReportVersion     int              `json:"report_version"`
	HypothesisID      string           `json:"hypothesis_id"`
	RequestedModel    string           `json:"requested_model"`
	CodeCommit        *string          `json:"code_commit"`
	PlanDigest        *string          `json:"plan_digest"`
	RunID             *string          `json:"run_id"`
	ElapsedMillis     int64            `json:"elapsed_millis"`
	DispatchCount     int              `json:"dispatch_count"`
	Region            *string          `json:"region"`
	HTTPStatus        *int             `json:"http_status"`
	MediaKind         *string          `json:"media_kind"`
	TransportComplete *bool            `json:"transport_complete"`
	DecodeComplete    *bool            `json:"decode_complete"`
	BodyFailure       *string          `json:"body_failure"`
	BodySummary       *responseSummary `json:"body_summary"`
	Outcome           string           `json:"outcome"`
	FailureCategory   *string          `json:"failure_category"`
	CleanupOutcome    string           `json:"cleanup_outcome"`
}

func newResponseReport() responseReport {
	return responseReport{ReportVersion: 1, HypothesisID: responseHypothesis, RequestedModel: probeModel, Outcome: "needs_evidence", CleanupOutcome: "complete"}
}
func (r *responseReport) fail(c string) {
	if c != "" && r.FailureCategory == nil {
		r.FailureCategory = &c
	}
	r.Outcome = "needs_evidence"
}
func (r *responseReport) bodyFail(c string) {
	r.BodySummary = nil
	r.BodyFailure = &c
	category := "invalid_response"
	switch c {
	case "informational_response", "invalid_headers", "unsupported_media":
		category = "response_format"
	case "limit":
		category = "response_limit"
	case "transport", "timeout", "canceled":
		category = c
	}
	r.fail(category)
}
func encodeResponseReport(r *responseReport) []byte {
	b, err := json.Marshal(r)
	if err != nil || len(b) > responseReportLimit {
		r.BodySummary = nil
		r.fail("report_limit")
		b, _ = json.Marshal(r)
	}
	return b
}
func responseLabel(s string) *string {
	if !slices.Contains(responseLabels(), s) {
		s = "other"
	}
	return &s
}
func responseSet(set []string, value string, order []string) []string {
	if !slices.Contains(set, value) {
		set = append(set, value)
		slices.SortFunc(set, func(a, b string) int { return slices.Index(order, a) - slices.Index(order, b) })
	}
	return set
}
func responsePath(root map[string]any, path string) (any, string) {
	var node any = root
	for _, key := range strings.Split(path[1:], "/") {
		m, ok := node.(map[string]any)
		if !ok {
			return nil, "unreachable"
		}
		v, exists := m[key]
		if !exists {
			return nil, "absent"
		}
		node = v
	}
	switch node.(type) {
	case nil:
		return node, "null"
	case bool:
		return node, "boolean"
	case json.Number:
		return node, "number"
	case string:
		return node, "string"
	case []any:
		return node, "array"
	default:
		return node, "object"
	}
}
func (s *responseSummary) record(ctx context.Context, data []byte, record responseRecord) string {
	if c := responseCancellation(ctx); c != "" {
		return c
	}
	s.RecordCount++
	if s.RecordCount > 256 {
		return "limit"
	}
	if !record.DoneMarker {
		if !utf8.Valid(data) {
			return "invalid_utf8"
		}
		root, err := schemaJSON(data, responseBodyLimit)
		if err != nil {
			return "invalid_json"
		}
		if c := responseCancellation(ctx); c != "" {
			return c
		}
		s.JSONRecordCount++
		if v, exists := root["type"]; exists {
			if text, ok := v.(string); ok {
				record.PayloadType = responseLabel(text)
			} else {
				record.PayloadType = responsePtr("invalid")
			}
		}
		for _, path := range responsePaths() {
			v, kind := responsePath(root, path)
			s.PathKinds[path] = responseSet(s.PathKinds[path], kind, responseKinds())
			text, ok := v.(string)
			if !ok {
				continue
			}
			switch path {
			case "/status", "/response/status":
				if !slices.Contains(responseStatuses(), text) {
					text = "other"
				}
				s.StatusLabels = responseSet(s.StatusLabels, text, responseStatuses())
			case "/model", "/response/model":
				comparison := "different"
				if text == probeModel {
					comparison = "match"
				}
				s.ModelComparisons = responseSet(s.ModelComparisons, comparison, []string{"match", "different"})
			case "/__type", "/code", "/error/type", "/error/code":
				text = text[strings.LastIndex(text, "#")+1:]
				if !slices.Contains(responseErrors(), text) {
					text = "other"
				}
				s.ErrorLabels = responseSet(s.ErrorLabels, text, responseErrors())
			}
		}
	}
	if len(s.EventSequence) < 32 {
		s.EventSequence = append(s.EventSequence, record)
	} else {
		s.SequenceTruncated = true
	}
	return ""
}
func observeResponse(ctx context.Context, data []byte, media string) (*responseSummary, string) {
	s := &responseSummary{EventSequence: []responseRecord{}, PathKinds: map[string][]string{}, StatusLabels: []string{}, ErrorLabels: []string{}, ModelComparisons: []string{}}
	for _, p := range responsePaths() {
		s.PathKinds[p] = []string{}
	}
	if len(data) > responseBodyLimit {
		return nil, "limit"
	}
	if len(data) == 0 {
		return nil, "empty_body"
	}
	var failure string
	switch media {
	case "json":
		failure = s.record(ctx, data, responseRecord{Channel: "json"})
	case "sse":
		failure = s.sse(ctx, data)
	case "eventstream":
		failure = s.frames(ctx, data)
	default:
		failure = "unsupported_media"
	}
	if failure != "" {
		return nil, failure
	}
	if s.JSONRecordCount == 0 {
		return nil, "empty_body"
	}
	return s, ""
}
func (s *responseSummary) sse(ctx context.Context, data []byte) string {
	if !utf8.Valid(data) {
		return "invalid_utf8"
	}
	var event *string
	var joined []byte
	hasData := false
	for len(data) > 0 {
		if c := responseCancellation(ctx); c != "" {
			return c
		}
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			if len(data) > 16<<10 {
				return "limit"
			}
			return "invalid_sse"
		}
		line := data[:i]
		data = data[i+1:]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if len(line) > 16<<10 {
			return "limit"
		}
		if bytes.ContainsRune(line, '\r') {
			return "invalid_sse"
		}
		if len(line) == 0 {
			if hasData {
				if f := s.record(ctx, joined, responseRecord{Channel: "sse", TransportLabel: event, DoneMarker: bytes.Equal(joined, []byte("[DONE]"))}); f != "" {
					return f
				}
			}
			event = nil
			joined = nil
			hasData = false
			continue
		}
		field, value, _ := bytes.Cut(line, []byte(":"))
		value = bytes.TrimPrefix(value, []byte(" "))
		switch string(field) {
		case "event":
			if event != nil {
				return "invalid_sse"
			}
			event = responseLabel(string(value))
		case "data":
			if hasData {
				joined = append(joined, '\n')
			}
			if len(joined)+len(value) > 64<<10 {
				return "limit"
			}
			joined = append(joined, value...)
			hasData = true
		}
	}
	if event != nil || hasData {
		return "invalid_sse"
	}
	return ""
}
func (s *responseSummary) frames(ctx context.Context, data []byte) string {
	for len(data) > 0 {
		if c := responseCancellation(ctx); c != "" {
			return c
		}
		if len(data) < 12 {
			return "invalid_eventstream"
		}
		total := int(binary.BigEndian.Uint32(data[:4]))
		headers := int(binary.BigEndian.Uint32(data[4:8]))
		if crc32.ChecksumIEEE(data[:8]) != binary.BigEndian.Uint32(data[8:12]) || total < 16 || headers > total-16 {
			return "invalid_eventstream"
		}
		if total > 64<<10 || headers > 8<<10 {
			return "limit"
		}
		if total > len(data) {
			return "invalid_eventstream"
		}
		frame := data[:total]
		data = data[total:]
		if crc32.ChecksumIEEE(frame[:total-4]) != binary.BigEndian.Uint32(frame[total-4:]) {
			return "invalid_eventstream"
		}
		kind, label, err := responseEventHeaders(frame[12 : 12+headers])
		if err != nil || !slices.Contains([]string{"event", "error", "exception"}, kind) {
			return "invalid_eventstream"
		}
		if f := s.record(ctx, frame[12+headers:total-4], responseRecord{Channel: "eventstream", MessageKind: &kind, TransportLabel: label}); f != "" {
			return f
		}
	}
	return ""
}

func consumeResponse(ctx context.Context, resp *http.Response, r *responseReport) {
	if c := responseCancellation(ctx); c != "" {
		r.fail(c)
		return
	}
	if resp.StatusCode >= 100 && resp.StatusCode < 200 {
		r.bodyFail("informational_response")
		return
	}
	r.HTTPStatus = responsePtr(resp.StatusCode)
	if resp.StatusCode != 200 {
		r.fail("http_status")
	}
	media, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	enc := resp.Header.Values("Content-Encoding")
	if err != nil || len(resp.Header.Values("Content-Type")) != 1 || len(enc) > 1 || len(enc) == 1 && !strings.EqualFold(enc[0], "identity") {
		r.bodyFail("invalid_headers")
		return
	}
	kind := "other"
	switch media {
	case "application/json", "application/x-amz-json-1.0":
		kind = "json"
	case "text/event-stream":
		kind = "sse"
	case "application/vnd.amazon.eventstream":
		kind = "eventstream"
	}
	r.MediaKind = &kind
	if len(params) > 0 && (kind == "eventstream" || len(params) != 1 || !strings.EqualFold(params["charset"], "utf-8")) {
		r.bodyFail("invalid_headers")
		return
	}
	if resp.ContentLength > responseBodyLimit {
		r.bodyFail("limit")
		return
	}
	if kind == "other" {
		r.bodyFail("unsupported_media")
		return
	}
	r.TransportComplete = responsePtr(false)
	buf := make([]byte, responseBodyLimit+1)
	used := 0
	for {
		if c := responseCancellation(ctx); c != "" {
			r.bodyFail(c)
			return
		}
		n, e := resp.Body.Read(buf[used:])
		used += n
		if c := responseCancellation(ctx); c != "" {
			r.bodyFail(c)
			return
		}
		if used > responseBodyLimit {
			r.bodyFail("limit")
			return
		}
		if e == io.EOF {
			*r.TransportComplete = true
			break
		}
		if e != nil {
			r.bodyFail("transport")
			return
		}
	}
	r.DecodeComplete = responsePtr(false)
	summary, failure := observeResponse(ctx, buf[:used], kind)
	if c := responseCancellation(ctx); c != "" {
		failure = c
	}
	if failure != "" {
		r.bodyFail(failure)
		return
	}
	*r.DecodeComplete = true
	r.BodySummary = summary
}
