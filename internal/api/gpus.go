package api

import (
	"encoding/json"
	"fmt"
)

type GPUType struct {
	GPUName        string    `json:"gpu_name"`
	GPUTotalMemory int       `json:"gpu_total_memory"`
	GPUPrice       FlexFloat `json:"gpu_price"`
	AvailableCount int       `json:"available_count"`

	// PlaceableCount is how many of the free GPUs the scheduler can actually
	// put work on. available_count counts every unassigned online GPU on an
	// online host, including hosts that are not GPU hosts and so can never
	// take a container. It is filled in from the serverless options endpoint,
	// which counts with the scheduler's own predicate, and is nil when that
	// endpoint could not be read.
	PlaceableCount *int `json:"placeable_count"`
}

// GPUTypeKey identifies a GPU type across endpoints that do not share ids.
type GPUTypeKey struct {
	Name     string
	MemoryMB int
}

// PlaceableGPUCounts returns, per GPU type, the number of free GPUs that sit on
// a host the scheduler will place work on. The serverless options endpoint
// counts free GPUs with the same predicate the placement query uses — the host
// must be online AND of type gpu — so it is the honest denominator for "can I
// run something on this right now".
func (c *Client) PlaceableGPUCounts() (map[GPUTypeKey]int, error) {
	options, err := c.GetServerlessOptions()
	if err != nil {
		return nil, err
	}
	counts := make(map[GPUTypeKey]int, len(options.GPUTypes))
	for _, t := range options.GPUTypes {
		counts[GPUTypeKey{Name: t.GPUName, MemoryMB: t.GPUTotalMemory}] = t.FreeGPUsCount
	}
	return counts, nil
}

type AvailableGPUsResponse struct {
	TotalAvailable int           `json:"total_available"`
	Summary        []GPUSummary  `json:"summary"`
	GPUs           []interface{} `json:"gpus"`

	// TotalPlaceable is the sum of the per-type placeable counts. See
	// GPUType.PlaceableCount for why the API's own total is an upper bound.
	TotalPlaceable *int `json:"total_placeable"`
}

type GPUSummary struct {
	Count    int       `json:"count"`
	GPUName  string    `json:"gpu_name"`
	MemoryMB int       `json:"memory_mb"`
	Price    FlexFloat `json:"price"`

	PlaceableCount *int `json:"placeable_count"`
}

func (c *Client) ListGPUTypes() ([]GPUType, error) {
	data, err := c.Get("/v1/gpus/types")
	if err != nil {
		return nil, err
	}

	var types []GPUType
	if err := json.Unmarshal(data, &types); err != nil {
		return nil, fmt.Errorf("failed to parse GPU types: %w", err)
	}
	return types, nil
}

// SummaryByType regroups the raw GPU rows by GPU name AND memory size. The API
// builds its own summary by grouping on name alone and then labels each group
// with the memory of whichever row it saw first, so free 20GB and 24GB RTX
// 3090s arrive merged into one bucket under a single size. Regrouping here
// keeps the per-size counts honest. It returns nil when there are no raw rows
// to group, so the caller keeps whatever the API sent.
func (r *AvailableGPUsResponse) SummaryByType() []GPUSummary {
	if len(r.GPUs) == 0 {
		return nil
	}

	counts := map[GPUTypeKey]*GPUSummary{}
	var order []GPUTypeKey
	for _, row := range r.GPUs {
		fields, ok := row.(map[string]interface{})
		if !ok {
			return nil
		}
		gpuType, ok := fields["gpu_type"].(map[string]interface{})
		if !ok {
			return nil
		}
		name, _ := gpuType["gpu_name"].(string)
		memory, _ := gpuType["gpu_total_memory"].(float64)
		key := GPUTypeKey{Name: name, MemoryMB: int(memory)}
		if existing, seen := counts[key]; seen {
			existing.Count++
			continue
		}
		price, _ := json.Marshal(gpuType["gpu_price"])
		summary := &GPUSummary{Count: 1, GPUName: name, MemoryMB: int(memory)}
		_ = json.Unmarshal(price, &summary.Price)
		counts[key] = summary
		order = append(order, key)
	}

	out := make([]GPUSummary, 0, len(order))
	for _, key := range order {
		out = append(out, *counts[key])
	}
	return out
}

func (c *Client) GetAvailableGPUs() (*AvailableGPUsResponse, error) {
	data, err := c.Get("/v1/gpus/available")
	if err != nil {
		return nil, err
	}

	var resp AvailableGPUsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse available GPUs: %w", err)
	}
	return &resp, nil
}
