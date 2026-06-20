package api

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// volumeUUIDPattern matches a full volume UUID (10-char nanoid). When the input
// already looks like one, file commands skip the (potentially many-page) volume
// listing and let the server validate it.
var volumeUUIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{10}$`)

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

type VolumeListMeta struct {
	CurrentPage int `json:"current_page"`
	LastPage    int `json:"last_page"`
}

type VolumeListResponse struct {
	Data  []Volume       `json:"data"`
	Links interface{}    `json:"links"`
	Meta  VolumeListMeta `json:"meta"`
}

// ListVolumes returns every volume, following Laravel pagination so callers
// are not silently truncated to the first page (default 15 per page).
func (c *Client) ListVolumes() ([]Volume, error) {
	var all []Volume

	for page := 1; ; page++ {
		data, err := c.Get(fmt.Sprintf("/v1/volumes?page=%d", page))
		if err != nil {
			return nil, err
		}

		var resp VolumeListResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("failed to parse volumes: %w", err)
		}

		all = append(all, resp.Data...)

		// Stop when there are no more pages. LastPage == 0 guards against a
		// non-paginated response so we never loop forever.
		if resp.Meta.LastPage <= page || len(resp.Data) == 0 {
			break
		}
	}

	return all, nil
}

// ResolveVolumeUUID resolves a full UUID, UUID prefix, or volume name to a full
// volume UUID so file commands can accept friendly identifiers.
func (c *Client) ResolveVolumeUUID(partial string) (string, error) {
	if partial == "" {
		return "", fmt.Errorf("volume identifier required")
	}

	// Fast path: a full UUID needs no lookup (avoids paging the whole list).
	if volumeUUIDPattern.MatchString(partial) {
		return partial, nil
	}

	volumes, err := c.ListVolumes()
	if err != nil {
		return "", err
	}

	// Exact UUID or name match wins outright.
	for _, v := range volumes {
		if v.VolumeUUID == partial || strings.EqualFold(v.VolumeName, partial) {
			return v.VolumeUUID, nil
		}
	}

	var matches []Volume
	lower := strings.ToLower(partial)
	for _, v := range volumes {
		if strings.HasPrefix(v.VolumeUUID, partial) || strings.Contains(strings.ToLower(v.VolumeName), lower) {
			matches = append(matches, v)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no volume found matching %q", partial)
	case 1:
		return matches[0].VolumeUUID, nil
	default:
		msg := fmt.Sprintf("ambiguous volume identifier %q matches %d volumes:\n", partial, len(matches))
		for _, m := range matches {
			msg += fmt.Sprintf("  %s  %s\n", m.VolumeUUID, m.VolumeName)
		}
		return "", fmt.Errorf("%s", msg)
	}
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

// CreateVolumeRequest is the payload for provisioning a new network volume.
type CreateVolumeRequest struct {
	VolumeName  string `json:"volume_name"`
	VolumeSpace int    `json:"volume_space"`
	RegionID    string `json:"region_id,omitempty"`
	VolumeType  string `json:"volume_type,omitempty"`
	Description string `json:"description,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

func (c *Client) CreateVolume(req *CreateVolumeRequest) (*Volume, error) {
	data, err := c.Post("/v1/volumes", req)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data Volume `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse created volume: %w", err)
	}
	return &resp.Data, nil
}

func (c *Client) DeleteVolume(uuid string) error {
	_, err := c.Delete("/v1/volumes/" + uuid)
	return err
}

// CloneVolumeRequest is the payload for duplicating an existing volume.
type CloneVolumeRequest struct {
	NewVolumeName string `json:"new_volume_name"`
	VolumeSpace   int    `json:"volume_space"`
}

func (c *Client) CloneVolume(sourceUUID string, req *CloneVolumeRequest) (*Volume, error) {
	data, err := c.Post("/v1/volumes/"+sourceUUID+"/duplicate", req)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data Volume `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse cloned volume: %w", err)
	}
	return &resp.Data, nil
}
