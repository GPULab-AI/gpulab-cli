package api

import (
	"encoding/json"
	"testing"
)

func TestFlexFloatUnmarshal(t *testing.T) {
	cases := []struct {
		name  string
		input string
		value float64
		valid bool
		text  string
	}{
		{name: "number", input: `1.25`, value: 1.25, valid: true},
		{name: "integer", input: `2`, value: 2, valid: true},
		{name: "quoted number", input: `"1.25"`, value: 1.25, valid: true},
		{name: "quoted integer", input: `"0"`, value: 0, valid: true},
		{name: "padded string", input: `" 3.50 "`, value: 3.5, valid: true},
		{name: "null", input: `null`},
		{name: "empty string", input: `""`},
		{name: "non-numeric string", input: `"N/A"`, text: "N/A"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var f FlexFloat
			if err := json.Unmarshal([]byte(tc.input), &f); err != nil {
				t.Fatalf("unmarshal %s: %v", tc.input, err)
			}
			value, ok := f.Float64()
			if ok != tc.valid {
				t.Fatalf("valid = %v, want %v", ok, tc.valid)
			}
			if ok && value != tc.value {
				t.Fatalf("value = %v, want %v", value, tc.value)
			}
			if !ok && f.String() != tc.text {
				t.Fatalf("text = %q, want %q", f.String(), tc.text)
			}
		})
	}
}

func TestFlexFloatMarshal(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "quoted number becomes a number", input: `"1.25"`, want: `1.25`},
		{name: "number stays a number", input: `1.25`, want: `1.25`},
		{name: "null stays null", input: `null`, want: `null`},
		{name: "empty string becomes null", input: `""`, want: `null`},
		{name: "non-numeric text is kept", input: `"N/A"`, want: `"N/A"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var f FlexFloat
			if err := json.Unmarshal([]byte(tc.input), &f); err != nil {
				t.Fatalf("unmarshal %s: %v", tc.input, err)
			}
			out, err := json.Marshal(f)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(out) != tc.want {
				t.Fatalf("marshal = %s, want %s", out, tc.want)
			}
		})
	}
}

// TestAvailableGPUsResponseStringPrice covers the shape that broke
// `gpulab gpus available`: the API sends summary prices as strings.
func TestAvailableGPUsResponseStringPrice(t *testing.T) {
	payload := `{
		"total_available": 3,
		"summary": [
			{"count": 2, "gpu_name": "RTX 4090", "memory_mb": 24576, "price": "0.69"},
			{"count": 1, "gpu_name": "H100", "memory_mb": 81920, "price": null}
		],
		"gpus": []
	}`

	var resp AvailableGPUsResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("unmarshal available GPUs: %v", err)
	}
	if len(resp.Summary) != 2 {
		t.Fatalf("summary length = %d, want 2", len(resp.Summary))
	}
	if value, ok := resp.Summary[0].Price.Float64(); !ok || value != 0.69 {
		t.Fatalf("first price = %v (valid %v), want 0.69", value, ok)
	}
	if _, ok := resp.Summary[1].Price.Float64(); ok {
		t.Fatalf("null price should not be valid")
	}
}

// TestGPUTypeNumberPrice guards the other direction: /v1/gpus/types sends the
// same field unquoted.
func TestGPUTypeNumberPrice(t *testing.T) {
	var types []GPUType
	payload := `[{"gpu_name": "A100", "gpu_total_memory": 40960, "gpu_price": 1.5, "available_count": 4}]`
	if err := json.Unmarshal([]byte(payload), &types); err != nil {
		t.Fatalf("unmarshal GPU types: %v", err)
	}
	if value, ok := types[0].GPUPrice.Float64(); !ok || value != 1.5 {
		t.Fatalf("price = %v (valid %v), want 1.5", value, ok)
	}
}
