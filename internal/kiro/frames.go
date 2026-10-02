package kiro

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"kiro-gateway/internal/bridge"
	"unicode/utf8"
)

// readFrames accepts EOF only between complete checksummed frames.
func readFrames(input io.Reader, visit func(string, []byte) error) error {
	totalBytes := 0
	for {
		var prelude [12]byte
		n, err := io.ReadFull(input, prelude[:])
		if n == 0 && errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		total := int(binary.BigEndian.Uint32(prelude[:4]))
		headers := int(binary.BigEndian.Uint32(prelude[4:8]))
		if crc32.ChecksumIEEE(prelude[:8]) != binary.BigEndian.Uint32(prelude[8:]) || total < 16 || total > 1<<20 || headers > total-16 || headers > 16<<10 || totalBytes+total > 8<<20 {
			return bridge.ProtocolFailure()
		}
		totalBytes += total
		frame := make([]byte, total)
		copy(frame, prelude[:])
		if _, err := io.ReadFull(input, frame[12:]); err != nil {
			return err
		}
		if crc32.ChecksumIEEE(frame[:total-4]) != binary.BigEndian.Uint32(frame[total-4:]) {
			return bridge.ProtocolFailure()
		}
		kind, event, err := eventHeaders(frame[12 : 12+headers])
		if err != nil || kind != "event" {
			return bridge.ProtocolFailure()
		}
		if err := visit(event, frame[12+headers:total-4]); err != nil {
			return err
		}
	}
}

func eventHeaders(b []byte) (kind, event string, result error) {
	seen := map[string]bool{}
	for len(b) > 0 {
		n := int(b[0])
		b = b[1:]
		if n == 0 || len(b) < n+1 || !utf8.Valid(b[:n]) {
			return "", "", bridge.ProtocolFailure()
		}
		name := string(b[:n])
		typ := b[n]
		b = b[n+1:]
		if seen[name] {
			return "", "", bridge.ProtocolFailure()
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
				return "", "", bridge.ProtocolFailure()
			}
			size = int(binary.BigEndian.Uint16(b[:2]))
			b = b[2:]
		default:
			return "", "", bridge.ProtocolFailure()
		}
		if len(b) < size || typ == 7 && !utf8.Valid(b[:size]) {
			return "", "", bridge.ProtocolFailure()
		}
		if name == ":message-type" || name == ":event-type" {
			if typ != 7 {
				return "", "", bridge.ProtocolFailure()
			}
			if name == ":message-type" {
				kind = string(b[:size])
			} else {
				event = string(b[:size])
			}
		}
		b = b[size:]
	}
	if kind == "" || kind == "event" && event == "" {
		return "", "", bridge.ProtocolFailure()
	}
	return kind, event, nil
}
