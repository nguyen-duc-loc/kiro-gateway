//go:build responsediscovery

package kiro

import (
	"encoding/binary"
	"unicode/utf8"
)

// The discovery grammar permits error and exception frames and a present empty
// event label. The production event grammar remains unchanged.
func responseEventHeaders(b []byte) (kind string, event *string, result error) {
	seen := map[string]bool{}
	for len(b) > 0 {
		n := int(b[0])
		b = b[1:]
		if n == 0 || len(b) < n+1 || !utf8.Valid(b[:n]) {
			return "", nil, errPlanInvalid
		}
		name := string(b[:n])
		typ := b[n]
		b = b[n+1:]
		if seen[name] {
			return "", nil, errPlanInvalid
		}
		seen[name] = true
		var size int
		switch typ {
		case 0, 1:
			size = 0
		case 2:
			size = 1
		case 3:
			size = 2
		case 4:
			size = 4
		case 5, 8:
			size = 8
		case 9:
			size = 16
		case 6, 7:
			if len(b) < 2 {
				return "", nil, errPlanInvalid
			}
			size = int(binary.BigEndian.Uint16(b[:2]))
			b = b[2:]
		default:
			return "", nil, errPlanInvalid
		}
		if len(b) < size || typ == 7 && !utf8.Valid(b[:size]) {
			return "", nil, errPlanInvalid
		}
		if name == ":message-type" || name == ":event-type" {
			if typ != 7 {
				return "", nil, errPlanInvalid
			}
			if name == ":message-type" {
				kind = string(b[:size])
			} else {
				event = responseLabel(string(b[:size]))
			}
		}
		b = b[size:]
	}
	if kind == "" || kind == "event" && event == nil {
		return "", nil, errPlanInvalid
	}
	return kind, event, nil
}
