// Package jsonobject validates bounded JSON objects without ambiguous members.
package jsonobject

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

// Parse rejects invalid UTF 8, duplicate members at every depth, and trailing data.
// It returns only a fixed error, never any part of the input.
func Parse(data []byte, limit int) (map[string]json.RawMessage, error) {
	invalid := errors.New("invalid JSON object")
	if len(data) > limit || !utf8.Valid(data) {
		return nil, invalid
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := value(d, 0); err != nil {
		return nil, invalid
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, invalid
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		return nil, invalid
	}
	return object, nil
}

func value(d *json.Decoder, depth int) error {
	if depth > 1000 {
		return errors.New("JSON nesting limit")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]bool)
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return errors.New("duplicate JSON member")
			}
			seen[name] = true
			if err := value(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := value(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid JSON delimiter")
	}
	_, err = d.Token()
	return err
}
