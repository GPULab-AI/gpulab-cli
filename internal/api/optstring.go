package api

import "encoding/json"

// OptString is a string field the API may leave out of a payload entirely.
//
// The serverless list endpoint sends a summary of each endpoint and omits the
// configuration block that only the detail endpoint carries — autoscaling
// policy, metrics config, CUDA version, command, notes. Decoding those into a
// plain string made "the API did not send this" indistinguishable from "the
// API sent an empty value", so `gpulab serverless list --json` reported every
// endpoint as having no autoscaling policy attached when all of them had one.
//
// Present records whether the key appeared at all. Callers must not print
// Value as fact unless Present is true; use Known and Or for that.
type OptString struct {
	Present bool
	Value   string
}

// NewOptString builds a known value, for tests and for locally derived data.
func NewOptString(value string) OptString {
	return OptString{Present: true, Value: value}
}

// Known reports whether the API sent this field, and therefore whether Value
// carries a real answer. An omitted field is unknown, not empty.
func (o OptString) Known() bool { return o.Present }

// Set reports whether the API sent this field AND it holds a value.
func (o OptString) Set() bool { return o.Present && o.Value != "" }

// String returns the value, which is empty for both an unknown and an empty
// field. Only use it after checking Known.
func (o OptString) String() string { return o.Value }

// Or returns Value when the API sent one, and the given placeholder otherwise,
// so human output can say "unknown" instead of showing a blank.
func (o OptString) Or(unknown string) string {
	if !o.Present {
		return unknown
	}
	return o.Value
}

func (o *OptString) UnmarshalJSON(data []byte) error {
	o.Present = true
	if string(data) == "null" {
		o.Value = ""
		return nil
	}
	return json.Unmarshal(data, &o.Value)
}

// MarshalJSON writes the value the API sent, or null when it sent nothing, so
// --json consumers can tell "no policy attached" from "not reported here".
func (o OptString) MarshalJSON() ([]byte, error) {
	if !o.Present {
		return []byte("null"), nil
	}
	return json.Marshal(o.Value)
}
