package api

import (
	"encoding/json"
	"fmt"
)

type Template struct {
	TemplateUUID  string  `json:"templateUuid"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	DockerImage   string  `json:"dockerImage"`
	ContainerType string  `json:"containerType"`
	Visibility    string  `json:"visibility"`
	ExposedPorts  *string `json:"exposedPorts"`
	Command       *string `json:"command"`
	MemoryLimit   *int    `json:"memoryLimit"`
	CreatedAt     string  `json:"createdAt"`
	UpdatedAt     string  `json:"updatedAt"`
}

type TemplateListResponse struct {
	Data  []Template  `json:"data"`
	Links interface{} `json:"links"`
	Meta  interface{} `json:"meta"`
}

func (c *Client) ListTemplates() ([]Template, error) {
	var all []Template
	page := 1
	for {
		data, err := c.Get(fmt.Sprintf("/v1/templates?page=%d", page))
		if err != nil {
			return nil, err
		}

		var resp TemplateListResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("failed to parse templates: %w", err)
		}
		all = append(all, resp.Data...)

		// Check if there are more pages
		if meta, ok := resp.Meta.(map[string]interface{}); ok {
			lastPage, _ := meta["last_page"].(float64)
			if float64(page) >= lastPage {
				break
			}
		} else {
			break
		}
		page++
	}
	return all, nil
}

func (c *Client) GetTemplate(id string) (*Template, error) {
	data, err := c.Get("/v1/templates/" + id)
	if err != nil {
		return nil, err
	}

	// API Resource wraps in "data" key
	var resp struct {
		Data Template `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	return &resp.Data, nil
}
