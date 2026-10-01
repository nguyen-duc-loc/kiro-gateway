package kiro

import (
	"bytes"
	"encoding/json"
	"strings"
)

// Schema vocabulary supplies diagnostic hints only, never decoding permission.
// Values are deliberately excluded, including values under recognized names.
const wireDiagnosticNames = `additionalContext additionalModelRequestFields agentContinuationId agentMode agentTaskType assistantResponseEvent assistantResponseMessage bytes
cacheReadInputTokens cacheWriteInputTokens character chatTriggerType client clientId code codeReferenceEvent
completion completions content contentType contextUsageEvent contextUsagePercentage conversationId conversationState
currentMessage currentWorkingDirectory cursorState data description document documentChar documentChunk
documentCitationEvent documentIndex documentPage documentSymbols documents domain edit editorState
effort end envState environmentVariables error fileContext filePath fileUri
filename followupPrompt format generateAssistantResponseResponse history http httpError httpHeader
id ideCategory ideVersion images innerContext input inputSchema inputTokens
instructions json jsonName jsonrpc key languageName leftFileContent licenseName
line location lspVersion maxOutputTokens message messageId metadata metadataEvent
meteringEvent method model modelId mostRelevantMissingImports name normalizedTokenUsage operatingSystem
optOutPreference origin outputTokens parameters params pluginVersion position predictionTypes
predictions previousEditorStateMetadata previousResponseId product profileArn programmingLanguage range reason
reasoning reasoningContent reasoningContentEvent reasoningText recommendationContentSpan recommendationsWithReferences redactedContent referenceTrackerConfiguration
references relativeFilePath relevantDocuments repository result retryAfterSeconds rightFileContent searchResult
searchResultIndex server signature source sourceContent start statement status
stop stream streaming strict summary supplementalContexts temperature text
timeOffset timezoneOffset title tokenUsage toolChoice toolResult toolResultEvent toolResults
toolSpecification toolUseEvent toolUseId toolUses tools topP totalTokens truncation
type uncachedInputTokens unit unitPlural url usage useRelevantDocuments userContext
userInputMessage userInputMessageContext utteranceId value web workspaceFolders`
const wireMaxUnknownDetails = 4

type wireUnknownField struct {
	Event     string `json:"event"`
	Location  string `json:"location"`
	KnownName string `json:"known_name"`
	Kind      string `json:"kind"`
}

func wireUnknownPolicy() map[string]any {
	return map[string]any{
		"max_details":    wireMaxUnknownDetails,
		"events":         []string{"assistantResponseEvent", "toolUseEvent", "messageMetadataEvent", "metadataEvent", "contextUsageEvent", "meteringEvent"},
		"locations":      []string{"event", "token_usage"},
		"known_names":    strings.Fields(wireDiagnosticNames),
		"unlisted_label": "unlisted",
		"kinds":          []string{"string", "number", "boolean", "null", "object", "array", "unknown"},
		"values":         "never_output",
		"behavior":       "ignore_metadata_event_extensions_otherwise_stop",
	}
}

func (s *wireTurn) noteUnknownField(event, location, name string, raw json.RawMessage) {
	s.unknownFields++
	if len(s.unknownDetails) >= wireMaxUnknownDetails {
		return
	}
	label := "unlisted"
	for _, candidate := range strings.Fields(wireDiagnosticNames) {
		if name == candidate {
			label = candidate
			break
		}
	}
	raw = bytes.TrimSpace(raw)
	kind := "unknown"
	if len(raw) != 0 {
		switch raw[0] {
		case '"':
			kind = "string"
		case '{':
			kind = "object"
		case '[':
			kind = "array"
		case 't', 'f':
			kind = "boolean"
		case 'n':
			kind = "null"
		default:
			if raw[0] == '-' || raw[0] >= '0' && raw[0] <= '9' {
				kind = "number"
			}
		}
	}
	s.unknownDetails = append(s.unknownDetails, wireUnknownField{event, location, label, kind})
}
