package commands

import (
	"testing"

	"github.com/GPULab-AI/gpulab-cli/internal/api"
)

func TestDetectLogIssues_LineCountingAndAttributeError(t *testing.T) {
	logs := "" +
		"AttributeError: 'NoneType' object has no attribute 'size'\n" +
		"AttributeError: 'NoneType' object has no attribute 'size'\n" +
		"all good\n"
	issues := detectLogIssues(logs)

	byCat := map[string]int{}
	for _, iss := range issues {
		byCat[iss.Category] = iss.Count
	}
	// The exact incident crash must be caught.
	if byCat["attribute-error"] != 2 {
		t.Fatalf("attribute-error count = %d, want 2", byCat["attribute-error"])
	}
	// Two overlapping patterns on the same line (NoneType twice) must count the
	// line once per category, not ×N.
	if byCat["nonetype"] != 2 {
		t.Fatalf("nonetype count = %d, want 2 (one per matching line)", byCat["nonetype"])
	}
}

func TestComputeRequestStats_ErrorRateNeverExceeds100(t *testing.T) {
	code := func(c int) *int { return &c }
	msg := "boom"
	reqs := []api.ServerlessRequestLog{
		// A 5xx that is also an error status + error message — must count once.
		{Status: "error", ResponseStatusCode: code(500), ErrorMessage: msg},
		{Status: "completed", ResponseStatusCode: code(200)},
	}
	s := computeRequestStats(reqs)
	if s.Errors != 1 {
		t.Fatalf("deduped Errors = %d, want 1", s.Errors)
	}
	if s.ErrorRate != 50 {
		t.Fatalf("ErrorRate = %v, want 50", s.ErrorRate)
	}
	if s.ErrorRate > 100 {
		t.Fatalf("ErrorRate exceeds 100: %v", s.ErrorRate)
	}
}
