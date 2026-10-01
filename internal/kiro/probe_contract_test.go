package kiro

import (
	"encoding/json"
	"reflect"
)

// Plan values are checked against the implemented, finite experiment. Human
// approval still belongs to the workflow, never to a status field in this file.
func wirePlanParts() map[string]any {
	return map[string]any{
		"destination":    map[string]any{"scheme": "https", "port": 443, "method": "POST", "path": "/", "content_type": wireContentType, "target": wireTarget, "accept": "application/vnd.amazon.eventstream"},
		"region_rule":    wireDestinations(),
		"authentication": map[string]string{"header": "Authorization", "scheme": "Bearer", "token_source": "selected_snapshot.access_token", "profile_source": "selected_snapshot.profile_arn"},
		"request_schema": map[string]any{
			"conversation_id_source": "crypto/rand UUID v4, generated once per run",
			"history":                "prior synthetic user messages and observed assistant text or validated tool calls, cases 1 through 4 only",
			"tool_result":            "fixed lookup for alpha, paired with the exact observed tool ID",
			"examples":               "request_examples, with placeholders for the selected profile ARN, generated conversation ID, and observed tool ID",
			"framing":                "amazon_eventstream_crc32",
			"event_fields":           wireEventFields(), "usage_fields": wireUsageFields(),
			"unknown_policy": "count_and_stop", "exception_policy": "stop_before_tentative_completion",
		},
		"instruction_mapping": map[string]string{"policy": wireInstructionPolicy, "prefix": wireInstructions, "separator": "\n\n", "field": "conversationState.currentMessage.userInputMessage.content", "label": "instructions_in_user_content"},
		"controls":            map[string]any{"max_tokens": wireMaxTokens, "thinking_type": "disabled", "field": "additionalModelRequestFields", "observation": "metadataEvent.tokenUsage.outputTokens <= 1024 when present; absence is unknown"},
		"limits":              map[string]int{"attempts": probeMaxAttempts, "run_seconds": 600, "request_seconds": 120, "idle_seconds": 30, "cleanup_seconds": 5, "request_bytes": probeMaxRequest, "header_bytes": probeMaxHeaders, "response_bytes": probeMaxResponse, "event_bytes": probeMaxEvent, "retained_bytes": probeMaxRetained, "cutoff_bytes": fixtureCutoff},
		"observation_policy":  map[string]any{"raw_values": "never_output", "instruction_label": "instructions_in_user_content", "completion_label": wireCompletionPolicy, "assertions": []string{"incremental", "instruction_placement", "marker_match", "valid_arguments", "matching_tool_name", "matching_tool_id", "matching_model_identity", "usage_present", "observed_completion", "tentative_completion", "controls_requested", "output_within_limit", "reached_injection_trigger", "completed_cleanup"}},
		"completion":          map[string]any{"policy": wireCompletionPolicy, "requires": []string{"HTTP 200", "valid EventStream content type", "CRC checked frames", "clean HTTP body EOF", "case assertions met", "no errors or unknown fields", "local cleanup completed"}, "observed_completion": nil, "best_verdict": "limited_candidate_observed"},
	}
}

func wirePlanCase(index int) map[string]any {
	trigger := "none"
	if index == 4 {
		trigger = "cancel child after first nonempty text event"
	}
	if index == 5 {
		trigger = "interrupt after 256 response bytes"
	}
	return map[string]any{"id": fixtureCaseIDs[index], "prompt": wirePrompts[index], "trigger": trigger}
}

func samePlanValue(raw json.RawMessage, expected any) bool {
	b, err := json.Marshal(expected)
	if err != nil {
		return false
	}
	var got, want any
	return json.Unmarshal(raw, &got) == nil && json.Unmarshal(b, &want) == nil && reflect.DeepEqual(got, want)
}

func wireExamples() ([]json.RawMessage, error) {
	examples := make([]json.RawMessage, 0, 6)
	history := []wireMessage{}
	for i := range fixtureCaseIDs {
		selectedHistory := history
		if i >= 4 {
			selectedHistory = nil
		}
		body, current, err := wireBody(i, selectedHistory, "{{observed_tool_id}}", "{{conversation_id}}", "{{selected_profile_arn}}")
		if err != nil {
			return nil, err
		}
		examples = append(examples, json.RawMessage(body))
		assistant := &wireAssistant{Content: fixtureMarker}
		if i == 1 {
			assistant.Content = ""
			assistant.Tools = []wireToolUse{{ID: "{{observed_tool_id}}", Name: fixtureTool, Input: json.RawMessage(`{"key":"alpha"}`)}}
		}
		if i == 2 {
			assistant.Content = fixtureToolResult
		}
		if i == 3 {
			assistant.Content = fixtureFollowup
		}
		if i < 4 {
			history = append(history, current, wireMessage{Assistant: assistant})
		}
	}
	return examples, nil
}

func (p probePlan) liveReadiness() error {
	if p.Status != "prepared_limited" || len(p.MissingContract) != 0 || len(p.Cases) != 6 || len(p.Sources) == 0 {
		return errNeedsEvidence
	}
	parts := wirePlanParts()
	for name, raw := range map[string]json.RawMessage{"destination": p.Destination, "region_rule": p.RegionRule, "authentication": p.Authentication, "request_schema": p.RequestSchema, "instruction_mapping": p.InstructionMapping, "controls": p.Controls, "completion": p.Completion, "limits": p.Limits, "observation_policy": p.ObservationPolicy} {
		if !samePlanValue(raw, parts[name]) {
			return errPlanInvalid
		}
	}
	for i, raw := range p.Cases {
		if !samePlanValue(raw, wirePlanCase(i)) {
			return errPlanInvalid
		}
	}
	examples, err := wireExamples()
	if err != nil || len(p.RequestExamples) != len(examples) {
		return errPlanInvalid
	}
	for i, raw := range p.RequestExamples {
		if !samePlanValue(raw, examples[i]) {
			return errPlanInvalid
		}
	}
	return nil
}
