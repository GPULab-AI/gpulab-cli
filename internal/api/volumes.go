package api

import (
	"encoding/json"
	"fmt"
)

type Volume struct {
	VolumeUUID string `json:"volumeUuid"`
	VolumeName string `json:"volumeName"`
	MaxSize    *int   `json:"maxSize"`
	UsedSize   *int   `json:"usedSize"`
	Status     string `json:"status"`
	IsDeleted  *int   `json:"isDeleted"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

type VolumeListResponse struct {
	Data  []Volume    `json:"data"`
	Links interface{} `json:"links"`
	Meta  interface{} `json:"meta"`
}

func (c *Client) ListVolumes() ([]Volume, error) {
	data, err := c.Get("/v1/volumes")
	if err != nil {
		return nil, err
	}

	var resp VolumeListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse volumes: %w", err)
	}
	return resp.Data, nil
}

func (c *Client) GetVolume(id string) (*Volume, error) {
	data, err := c.Get("/v1/volumes/" + id)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data Volume `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse volume: %w", err)
	}
	return &resp.Data, nil
}
