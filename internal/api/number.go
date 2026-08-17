package api

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// FlexFloat is a numeric API field that arrives either as a JSON number (1.25)
// or as a quoted string ("1.25"). The API is not consistent about this: decimal
// columns come back quoted on some endpoints (/v1/gpus/available) and unquoted
// on others (/v1/gpus/types), so a plain float64 field breaks one of them with
// "cannot unmarshal string into Go value of type float64".
//
// Values that are absent, null, empty, or not a number at all (e.g. "N/A") leave
// Valid false rather than failing the whole response — a price we cannot read is
// not a reason to lose the GPU listing around it.
type FlexFloat struct {
	Value float64
	Valid bool
	// text keeps the original string for non-numeric values so JSON output does
	// not silently drop information the API sent.
	text string
}

// Float64 returns the value and whether it is a usable number.
func (f FlexFloat) Float64() (float64, bool) {
	return f.Value, f.Valid
}

// String returns the value as text: a plain number, the original string when the
// API sent something non-numeric, or "" when there was no value.
func (f FlexFloat) String() string {
	if f.Valid {
		return strconv.FormatFloat(f.Value, 'f', -1, 64)
	}
	return f.text
}

// UnmarshalJSON accepts a JSON number, a quoted number, or null.
func (f *FlexFloat) UnmarshalJSON(data []byte) error {
	*f = FlexFloat{}
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil
	}

	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return nil
		}
		value, err := strconv.ParseFloat(s, 64)
		if err != nil {
			f.text = s
			return nil
		}
		*f = FlexFloat{Value: value, Valid: true}
		return nil
	}

	var value float64
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*f = FlexFloat{Value: value, Valid: true}
	return nil
}

// MarshalJSON writes a plain JSON number so --json output has one stable shape
// whichever way the API sent the field. Non-numeric text is passed through as a
// string, and a missing value as null.
func (f FlexFloat) MarshalJSON() ([]byte, error) {
	switch {
	case f.Valid:
		return json.Marshal(f.Value)
	case f.text != "":
		return json.Marshal(f.text)
	default:
		return []byte("null"), nil
	}
}
