package threatprot

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

var (
	ErrJSONTooDeep      = errors.New("json structure too deep")
	ErrJSONStringTooLong = errors.New("json string value too long")
)

// ValidateJSONStructure rejects oversized depth and string tokens before decode.
// Mirrors Apigee JSON Threat Protection depth/string limits.
func ValidateJSONStructure(data []byte, maxDepth, maxStringLen int) error {
	if maxDepth <= 0 {
		maxDepth = MaxJSONContainerDepth
	}
	if maxStringLen <= 0 {
		maxStringLen = MaxJSONStringLength
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	depth := 0
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		switch v := tok.(type) {
		case json.Delim:
			switch v {
			case '{', '[':
				depth++
				if depth > maxDepth {
					return ErrJSONTooDeep
				}
			case '}', ']':
				depth--
			}
		case string:
			if utf8.RuneCountInString(v) > maxStringLen {
				return ErrJSONStringTooLong
			}
		}
	}
}
