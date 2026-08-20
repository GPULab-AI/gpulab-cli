package api

import (
	"encoding/json"
	"testing"
)

// The available-GPUs endpoint groups its summary by GPU name alone and labels
// each group with the memory of whichever row it saw first, so free 20GB and
// 24GB RTX 3090s arrive merged under a single size.
func TestSummaryByTypeSplitsMemorySizes(t *testing.T) {
	payload := `{
		"total_available": 4,
		"summary": [{"count":4,"gpu_name":"NVIDIA GeForce RTX 3090","memory_mb":24576,"price":"0.30"}],
		"gpus": [
			{"id":1,"gpu_type":{"gpu_name":"NVIDIA GeForce RTX 3090","gpu_total_memory":24576,"gpu_price":"0.30"}},
			{"id":2,"gpu_type":{"gpu_name":"NVIDIA GeForce RTX 3090","gpu_total_memory":24576,"gpu_price":"0.30"}},
			{"id":3,"gpu_type":{"gpu_name":"NVIDIA GeForce RTX 3090","gpu_total_memory":20480,"gpu_price":"0.20"}},
			{"id":4,"gpu_type":{"gpu_name":"NVIDIA GeForce RTX 4090","gpu_total_memory":24576,"gpu_price":"0.30"}}
		]
	}`

	var resp AvailableGPUsResponse
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	got := resp.SummaryByType()
	if len(got) != 3 {
		t.Fatalf("got %d groups, want 3 (3090 24GB, 3090 20GB, 4090 24GB): %+v", len(got), got)
	}

	counts := map[GPUTypeKey]int{}
	for _, s := range got {
		counts[GPUTypeKey{Name: s.GPUName, MemoryMB: s.MemoryMB}] = s.Count
	}
	for key, want := range map[GPUTypeKey]int{
		{Name: "NVIDIA GeForce RTX 3090", MemoryMB: 24576}: 2,
		{Name: "NVIDIA GeForce RTX 3090", MemoryMB: 20480}: 1,
		{Name: "NVIDIA GeForce RTX 4090", MemoryMB: 24576}: 1,
	} {
		if counts[key] != want {
			t.Errorf("%v = %d, want %d", key, counts[key], want)
		}
	}
}

func TestSummaryByTypeKeepsAPISummaryWhenNoRows(t *testing.T) {
	var resp AvailableGPUsResponse
	if err := json.Unmarshal([]byte(`{"total_available":0,"summary":[],"gpus":[]}`), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := resp.SummaryByType(); got != nil {
		t.Errorf("SummaryByType() = %+v, want nil so the caller keeps the API's own summary", got)
	}
}
