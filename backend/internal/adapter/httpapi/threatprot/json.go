package threatprot

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

var (
	ErrJSONTooDeep             = errors.New("json structure too deep")
	ErrJSONStringTooLong       = errors.New("json string value too long")
	ErrJSONObjectNameTooLong   = errors.New("json object entry name too long")
	ErrJSONObjectEntriesExceed = errors.New("json object entry count exceeded")
	ErrJSONArrayElementsExceed = errors.New("json array element count exceeded")
)

type frameKind int

const (
	frameObject frameKind = iota
	frameArray
)

type parseFrame struct {
	kind      frameKind
	count     int
	expectKey bool
}

// ValidateJSONStructure rejects oversized depth, keys, object/array fan-out, and
// string tokens before decode. Mirrors Apigee JSON Threat Protection.
func ValidateJSONStructure(data []byte, maxDepth, maxStringLen int) error {
	if maxDepth <= 0 {
		maxDepth = MaxJSONContainerDepth
	}
	if maxStringLen <= 0 {
		maxStringLen = MaxJSONStringLength
	}
	maxNameLen := MaxJSONObjectEntryNameLength
	maxObjEntries := MaxJSONObjectEntryCount
	maxArrElems := MaxJSONArrayElementCount

	dec := json.NewDecoder(bytes.NewReader(data))
	stack := make([]parseFrame, 0, maxDepth+1)
	depth := 0

	noteArrayElement := func() error {
		if len(stack) == 0 {
			return nil
		}
		top := &stack[len(stack)-1]
		if top.kind != frameArray {
			return nil
		}
		top.count++
		if top.count > maxArrElems {
			return ErrJSONArrayElementsExceed
		}
		return nil
	}

	noteValueComplete := func() {
		if len(stack) == 0 {
			return
		}
		top := &stack[len(stack)-1]
		if top.kind == frameObject {
			top.expectKey = true
		}
	}

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
			case '{':
				if err := noteArrayElement(); err != nil {
					return err
				}
				depth++
				if depth > maxDepth {
					return ErrJSONTooDeep
				}
				stack = append(stack, parseFrame{kind: frameObject, expectKey: true})
			case '[':
				if err := noteArrayElement(); err != nil {
					return err
				}
				depth++
				if depth > maxDepth {
					return ErrJSONTooDeep
				}
				stack = append(stack, parseFrame{kind: frameArray})
			case '}', ']':
				if len(stack) == 0 {
					return errors.New("json delimiter mismatch")
				}
				stack = stack[:len(stack)-1]
				depth--
				noteValueComplete()
			}
		case string:
			if len(stack) > 0 {
				top := &stack[len(stack)-1]
				if top.kind == frameObject && top.expectKey {
					if utf8.RuneCountInString(v) > maxNameLen {
						return ErrJSONObjectNameTooLong
					}
					top.count++
					if top.count > maxObjEntries {
						return ErrJSONObjectEntriesExceed
					}
					top.expectKey = false
					continue
				}
			}
			if utf8.RuneCountInString(v) > maxStringLen {
				return ErrJSONStringTooLong
			}
			if err := noteArrayElement(); err != nil {
				return err
			}
			noteValueComplete()
		default:
			if err := noteArrayElement(); err != nil {
				return err
			}
			noteValueComplete()
		}
	}
}
