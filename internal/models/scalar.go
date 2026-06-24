package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

// StringOrNumber is a scalar that may be provided as either a YAML/JSON string
// or a number. It preserves the original representation so that v3 output can
// carry {{var}} placeholder strings while numeric values still round-trip as
// numbers. The zero value is an empty (omittable) scalar.
//
// Internally the raw decoded value is kept as one of: nil, string, int64 or
// float64. Marshaling re-emits that underlying value, so a numeric input stays
// a JSON number and a string input (including "{{var}}") stays a JSON string.
type StringOrNumber struct {
	value any // nil | string | int64 | float64
}

// String returns the scalar rendered as a string. Numbers are formatted without
// a trailing ".0" for integral float values.
func (s StringOrNumber) String() string {
	switch v := s.value.(type) {
	case nil:
		return ""
	case string:
		return v
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		// Format integral floats without a decimal point (e.g. 2 not 2.0).
		return strconv.FormatFloat(v, 'g', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// IsZero reports whether the scalar holds no value, used for omitempty support.
func (s StringOrNumber) IsZero() bool {
	return s.value == nil
}

// IsString reports whether the underlying value was decoded as a string (which
// is the case for {{var}} placeholders).
func (s StringOrNumber) IsString() bool {
	_, ok := s.value.(string)
	return ok
}

// Value returns the underlying decoded value (nil, string, int64 or float64).
func (s StringOrNumber) Value() any {
	return s.value
}

func (s *StringOrNumber) UnmarshalYAML(node *yaml.Node) error {
	if node.Tag == "!!null" {
		s.value = nil
		return nil
	}
	switch node.Tag {
	case "!!int":
		var i int64
		if err := node.Decode(&i); err != nil {
			return err
		}
		s.value = i
	case "!!float":
		var f float64
		if err := node.Decode(&f); err != nil {
			return err
		}
		s.value = f
	default:
		var str string
		if err := node.Decode(&str); err != nil {
			return err
		}
		s.value = str
	}
	return nil
}

func (s StringOrNumber) MarshalYAML() (any, error) {
	return s.value, nil
}

func (s *StringOrNumber) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		s.value = nil
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		s.value = str
		return nil
	}
	// Numeric: keep integral numbers as int64, otherwise float64.
	var num json.Number
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&num); err != nil {
		return err
	}
	if i, err := num.Int64(); err == nil {
		s.value = i
		return nil
	}
	f, err := num.Float64()
	if err != nil {
		return err
	}
	s.value = f
	return nil
}

func (s StringOrNumber) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.value)
}
