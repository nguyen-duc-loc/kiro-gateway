//go:build schemaprobe

package kiro

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
)

const schemaBodyLimit = 1 << 20

func schemaPaths() []string {
	return []string{"system", "system_prompt", "messages", "max_tokens", "output_config.effort", "reasoning.effort", "thinking.type", "thinking.display"}
}

type schemaField struct {
	Path              string   `json:"path"`
	Present           *bool    `json:"present"`
	Shape             string   `json:"shape"`
	Type              *string  `json:"type"`
	Minimum           *int64   `json:"minimum"`
	Maximum           *int64   `json:"maximum"`
	EnumState         *string  `json:"enum_state"`
	EnumValues        []string `json:"enum_values"`
	OtherEnumValues   *bool    `json:"other_enum_values"`
	DefaultValue      *string  `json:"default_value"`
	OtherDefaultValue *bool    `json:"other_default_value"`
}

type schemaReport struct {
	ReportVersion     int           `json:"report_version"`
	CodeCommit        *string       `json:"code_commit"`
	PlanDigest        *string       `json:"plan_digest"`
	RunID             *string       `json:"run_id"`
	ElapsedMillis     int64         `json:"elapsed_millis"`
	DispatchCount     int           `json:"dispatch_count"`
	Region            *string       `json:"region"`
	RequestedModel    string        `json:"requested_model"`
	Outcome           string        `json:"outcome"`
	FailureCategory   *string       `json:"failure_category"`
	CleanupOutcome    string        `json:"cleanup_outcome"`
	HTTPStatus        *int          `json:"http_status"`
	ModelFound        *bool         `json:"model_found"`
	SchemaPresent     *bool         `json:"schema_present"`
	MorePages         *bool         `json:"more_pages"`
	CatalogueComplete *bool         `json:"catalogue_complete"`
	Fields            []schemaField `json:"fields"`
}

func schemaPtr[T any](v T) *T { return &v }

func newSchemaReport() schemaReport {
	r := schemaReport{ReportVersion: 1, RequestedModel: probeModel, Outcome: "needs_evidence", CleanupOutcome: "complete"}
	for _, path := range schemaPaths() {
		r.Fields = append(r.Fields, schemaField{Path: path, Shape: "other"})
	}
	return r
}

func (r *schemaReport) fail(category string) {
	if r.FailureCategory == nil {
		r.FailureCategory = schemaPtr(category)
	}
	r.Outcome = "needs_evidence"
}

func extractSchema(data []byte, r *schemaReport) {
	root, err := schemaJSON(data, schemaBodyLimit)
	if err != nil {
		r.fail("invalid_response")
		return
	}
	models, ok := root["models"].([]any)
	if !ok || len(models) > 100 {
		r.fail("invalid_response")
		return
	}
	more := false
	if token := root["nextToken"]; token != nil {
		s, ok := token.(string)
		if !ok || s == "" {
			r.fail("invalid_response")
			return
		}
		more = true
	}
	var selected map[string]any
	matches := 0
	for _, value := range models {
		model, ok := value.(map[string]any)
		if !ok {
			r.fail("invalid_response")
			return
		}
		if id, _ := model["modelId"].(string); id == probeModel {
			selected = model
			matches++
		}
	}
	if matches > 1 {
		r.fail("invalid_response")
		return
	}
	r.MorePages, r.CatalogueComplete = schemaPtr(more), schemaPtr(!more)
	r.ModelFound = schemaPtr(matches == 1)
	if matches == 0 {
		r.fail("model_missing")
		return
	}
	r.SchemaPresent = schemaPtr(selected["additionalModelRequestFieldsSchema"] != nil)
	if !*r.SchemaPresent {
		r.fail("schema_missing")
		return
	}
	schema, ok := selected["additionalModelRequestFieldsSchema"].(map[string]any)
	if !ok {
		r.fail("schema_unsupported")
		return
	}
	fields := make([]schemaField, 0, 8)
	for _, path := range schemaPaths() {
		field, ok := inspectSchemaPath(schema, path)
		if !ok {
			r.fail("schema_unsupported")
			return
		}
		fields = append(fields, field)
	}
	r.Fields = fields
	// Only the runner can grant schema_observed, after transport cleanup.
}

func inspectSchemaPath(root map[string]any, path string) (schemaField, bool) {
	unknown := schemaField{Path: path, Shape: "other"}
	var node any = root
	for _, part := range strings.Split(path, ".") {
		object, ok := node.(map[string]any)
		if !ok {
			return unknown, true
		}
		if _, ref := object["$ref"]; ref {
			return unknown, true
		}
		properties, ok := object["properties"].(map[string]any)
		if !ok {
			return unknown, true
		}
		var exists bool
		node, exists = properties[part]
		if !exists {
			return schemaField{Path: path, Present: schemaPtr(false), Shape: "absent"}, true
		}
	}
	object, ok := node.(map[string]any)
	if !ok {
		unknown.Present = schemaPtr(true)
		return unknown, true
	}
	if _, ref := object["$ref"]; ref {
		return unknown, true
	}
	f := schemaField{Path: path, Present: schemaPtr(true), Shape: "object", Type: schemaPtr("unknown"), EnumState: schemaPtr("absent"), EnumValues: []string{}, OtherDefaultValue: schemaPtr(false)}
	if typ, ok := object["type"].(string); ok && slices.Contains([]string{"object", "array", "string", "integer", "number", "boolean", "null"}, typ) {
		f.Type = schemaPtr(typ)
	}
	f.Minimum, f.Maximum = schemaInteger(object["minimum"]), schemaInteger(object["maximum"])
	if value, exists := object["enum"]; exists {
		f.EnumState = schemaPtr("other")
		if values, ok := value.([]any); ok {
			if len(values) > 32 {
				return unknown, false
			}
			f.EnumState, f.OtherEnumValues = schemaPtr("array"), schemaPtr(false)
			for _, value := range values {
				if s, ok := schemaAllowedValue(value); ok {
					f.EnumValues = append(f.EnumValues, s)
				} else {
					*f.OtherEnumValues = true
				}
			}
			slices.Sort(f.EnumValues)
			f.EnumValues = slices.Compact(f.EnumValues)
		}
	}
	if value, exists := object["default"]; exists {
		if s, ok := schemaAllowedValue(value); ok {
			f.DefaultValue = schemaPtr(s)
		} else {
			*f.OtherDefaultValue = true
		}
	}
	return f, true
}

func schemaAllowedValue(value any) (string, bool) {
	s, ok := value.(string)
	return s, ok && slices.Contains([]string{"low", "medium", "high", "max", "minimal", "none", "xhigh", "disabled", "enabled", "adaptive", "summarized", "omitted"}, s)
}

func schemaInteger(value any) *int64 {
	n, ok := value.(json.Number)
	if !ok {
		return nil
	}
	// Normalize decimal digits without floating point rounding or allocating
	// a large power of ten for an attacker supplied exponent.
	s := string(n)
	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	mantissa, exponent, hasExponent := strings.Cut(strings.ToLower(s), "e")
	shift := 0
	if dot := strings.IndexByte(mantissa, '.'); dot >= 0 {
		shift -= len(mantissa) - dot - 1
		mantissa = mantissa[:dot] + mantissa[dot+1:]
	}
	mantissa = strings.TrimLeft(mantissa, "0")
	if mantissa == "" {
		return schemaPtr(int64(0))
	}
	if negative {
		return nil
	}
	if hasExponent {
		e, err := strconv.Atoi(exponent)
		if err != nil || e < -schemaBodyLimit || e > schemaBodyLimit {
			return nil
		}
		shift += e
	}
	trimmed := strings.TrimRight(mantissa, "0")
	shift += len(mantissa) - len(trimmed)
	if shift < 0 || len(trimmed)+shift > 10 {
		return nil
	}
	v, err := strconv.ParseInt(trimmed+strings.Repeat("0", shift), 10, 64)
	if err != nil || v < 0 || v > 2147483647 {
		return nil
	}
	return &v
}
