package kiro

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

func wireClientFingerprint(hostname, username string) string {
	sum := sha256.Sum256([]byte(hostname + "-" + username + "-kiro-gateway"))
	return hex.EncodeToString(sum[:])
}

func wireUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", errNeedsEvidence
	}
	b[6], b[8] = (b[6]&0x0f)|0x40, (b[8]&0x3f)|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func wireHeaders(fingerprint, invocationID string) (http.Header, error) {
	if len(fingerprint) != 64 || strings.Trim(fingerprint, "0123456789abcdef") != "" ||
		len(invocationID) != 36 || invocationID[8] != '-' || invocationID[13] != '-' ||
		invocationID[18] != '-' || invocationID[23] != '-' || invocationID[14] != '4' ||
		!strings.ContainsRune("89ab", rune(invocationID[19])) {
		return nil, errPlanInvalid
	}
	compact := strings.ReplaceAll(invocationID, "-", "")
	if len(compact) != 32 || strings.Trim(compact, "0123456789abcdef") != "" {
		return nil, errPlanInvalid
	}
	h := make(http.Header)
	for name, value := range map[string]string{
		"Content-Type":                wireContentType,
		"X-Amz-Target":                wireTarget,
		"User-Agent":                  "aws-sdk-js/1.0.27 ua/2.1 os/win32#10.0.19044 lang/js md/nodejs#22.21.1 api/codewhispererstreaming#1.0.27 m/E KiroIDE-0.7.45-" + fingerprint,
		"X-Amz-User-Agent":            "aws-sdk-js/1.0.27 KiroIDE-0.7.45-" + fingerprint,
		"X-Amzn-Codewhisperer-Optout": "true",
		"X-Amzn-Kiro-Agent-Mode":      "vibe",
		"Amz-Sdk-Invocation-Id":       invocationID,
		"Amz-Sdk-Request":             "attempt=1; max=3",
		"Accept":                      "*/*",
		"Accept-Encoding":             "identity",
	} {
		h.Set(name, value)
	}
	return h, nil
}

// These values are part of the reviewed plan, not a promise of SDK retries.
func wireHeaderPolicy() map[string]any {
	h, _ := wireHeaders(strings.Repeat("0", 64), "00000000-0000-4000-8000-000000000001")
	examples := make(map[string]string)
	for name := range h {
		examples[name] = strings.ReplaceAll(h.Get(name), strings.Repeat("0", 64), "{{client_fingerprint}}")
	}
	examples["Amz-Sdk-Invocation-Id"] = "{{invocation_id}}"
	return map[string]any{
		"headers":            examples,
		"fingerprint_source": "SHA256(hostname + '-' + username + '-kiro-gateway'); username LOGNAME, USER, LNAME, USERNAME, then OS user; resolve only after live gates; errors stop",
		"invocation_source":  "crypto/rand UUID v4 per request; never retained in output",
		"identity_output":    "never_output",
		"sdk_retry_metadata": "literal reference metadata only; no retry or replay",
	}
}
