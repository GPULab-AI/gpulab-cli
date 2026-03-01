package api

import (
	"encoding/json"
	"fmt"
)

type GPUType struct {
	GPUName        string      `json:"gpu_name"`
	GPUTotalMemory int         `json:"gpu_total_memory"`
	GPUPrice       json.Number `json:"gpu_price"`
	AvailableCount int         `json:"available_count"`
}

type AvailableGPUsResponse struct {
	TotalAvailable int          `json:"total_available"`
	Summary        []GPUSummary `json:"summary"`
	GPUs           []interface{} `json:"gpus"`
}

type GPUSummary struct {
	Count    int      `json:"count"`
	GPUName  string   `json:"gpu_name"`
	MemoryMB int      `json:"memory_mb"`
	Price    *float64 `json:"price"`
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
