package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

// EnvVarsField holds environment variables. The API may return them either as a
// JSON object (templates stored the modern way) or as a legacy "KEY=VALUE"
// newline string, so we normalize both into a map on decode.
type EnvVarsField map[string]string

func (e *EnvVarsField) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" || trimmed == `""` || trimmed == "[]" || trimmed == "{}" {
		*e = EnvVarsField{}
		return nil
	}

	// Object form: {"KEY":"VALUE"} (values may be non-string).
	var asMap map[string]interface{}
	if err := json.Unmarshal(data, &asMap); err == nil {
		out := EnvVarsField{}
		for k, v := range asMap {
			out[k] = fmt.Sprintf("%v", v)
		}
		*e = out
		return nil
	}

	// Legacy string form: "KEY=VALUE\nKEY2=VALUE2".
	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		out := EnvVarsField{}
		for _, line := range strings.Split(asString, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if idx := strings.IndexByte(line, '='); idx >= 0 {
				out[strings.TrimSpace(line[:idx])] = strings.TrimSpace(line[idx+1:])
			}
		}
		*e = out
		return nil
	}

	return fmt.Errorf("environment_variables: unexpected JSON %q", trimmed)
}

// FlexString decodes a value the API may send as either a JSON string or a JSON
// number (e.g. disk sizes are stored as strings but may come back either way).
type FlexString string

func (f *FlexString) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" {
		*f = ""
		return nil
	}
	if strings.HasPrefix(trimmed, "\"") {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*f = FlexString(s)
		return nil
	}
	*f = FlexString(trimmed)
	return nil
}

func (f FlexString) String() string { return string(f) }

type Template struct {
	TemplateUUID         string       `json:"templateUuid"`
	Name                 string       `json:"name"`
	Description          string       `json:"description"`
	AuthorURL            string       `json:"authorUrl"`
	AuthorName           string       `json:"authorName"`
	Thumbnail            string       `json:"thumbnail"`
	Visibility           string       `json:"visibility"`
	ContainerType        string       `json:"containerType"`
	VolumeMountPath      string       `json:"volumeMountPath"`
	DockerImage          string       `json:"dockerImage"`
	ExposedPorts         *string      `json:"exposedPorts"`
	PortsArray           []int        `json:"portsArray"`
	EnvironmentVariables EnvVarsField `json:"environmentVariables"`
	Command              *string      `json:"command"`
	ContainerDiskSize    FlexString   `json:"containerDiskSize"`
	VolumeDiskSize       FlexString   `json:"volumeDiskSize"`
	Notes                string       `json:"notes"`
	MemoryLimit          *int         `json:"memoryLimit"`
	CreatedAt            string       `json:"createdAt"`
	UpdatedAt            string       `json:"updatedAt"`
}

type TemplateListResponse struct {
	Data  []Template  `json:"data"`
	Links interface{} `json:"links"`
	Meta  interface{} `json:"meta"`
}

// TemplateCategory is a category that templates can be filed under. category_id
// is required by the model, so the CLI surfaces categories for discovery.
type TemplateCategory struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// TemplateRequest is the create/update payload. Pointer and omitempty fields let
// the edit command send only the flags the user actually changed (partial PUT).
type TemplateRequest struct {
	Name          string  `json:"name,omitempty"`
	DockerImage   string  `json:"docker_image,omitempty"`
	Description   *string `json:"description,omitempty"`
	AuthorURL     *string `json:"author_url,omitempty"`
	AuthorName    *string `json:"author_name,omitempty"`
	Thumbnail     *string `json:"thumbnail,omitempty"`
	Visibility    string  `json:"visibility,omitempty"`
	ContainerType string  `json:"container_type,omitempty"`
	CategoryID    *int    `json:"category_id,omitempty"`
	CredentialsID *int    `json:"credentials_id,omitempty"`
	// Inline registry credentials. When username + password are set, the API
	// creates a credential and links it, so a private image needs no separate
	// 'credentials add' step. Registry defaults to docker.io server-side.
	Registry             *string           `json:"registry,omitempty"`
	RegistryUsername     *string           `json:"registry_username,omitempty"`
	RegistryPassword     *string           `json:"registry_password,omitempty"`
	VolumeMountPath      *string           `json:"volume_mount_path,omitempty"`
	ExposedPorts         *string           `json:"exposed_ports,omitempty"`
	EnvironmentVariables map[string]string `json:"environment_variables,omitempty"`
	Command              *string           `json:"command,omitempty"`
	ContainerDiskSize    *int              `json:"container_disk_size,omitempty"`
	VolumeDiskSize       *int              `json:"volume_disk_size,omitempty"`
	MemoryLimit          *int              `json:"memory_limit,omitempty"`
	ImagePullPolicy      string            `json:"image_pull_policy,omitempty"`
	Notes                *string           `json:"notes,omitempty"`
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

// ResolveTemplateUUID resolves a full UUID, UUID prefix, or template name to a
// full template UUID so edit/info/delete can accept friendly identifiers.
func (c *Client) ResolveTemplateUUID(partial string) (string, error) {
	if partial == "" {
		return "", fmt.Errorf("template identifier required")
	}

	templates, err := c.ListTemplates()
	if err != nil {
		return "", err
	}

	// Exact UUID or name match wins outright.
	for _, t := range templates {
		if t.TemplateUUID == partial || strings.EqualFold(t.Name, partial) {
			return t.TemplateUUID, nil
		}
	}

	var matches []Template
	lower := strings.ToLower(partial)
	for _, t := range templates {
		if strings.HasPrefix(t.TemplateUUID, partial) || strings.Contains(strings.ToLower(t.Name), lower) {
			matches = append(matches, t)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no template found matching %q", partial)
	case 1:
		return matches[0].TemplateUUID, nil
	default:
		msg := fmt.Sprintf("ambiguous template identifier %q matches %d templates:\n", partial, len(matches))
		for _, m := range matches {
			msg += fmt.Sprintf("  %s  %s\n", m.TemplateUUID, m.Name)
		}
		return "", fmt.Errorf("%s", msg)
	}
}

func (c *Client) ListTemplateCategories() ([]TemplateCategory, error) {
	data, err := c.Get("/v1/templates/categories")
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []TemplateCategory `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse template categories: %w", err)
	}
	return resp.Data, nil
}

func (c *Client) CreateTemplate(req *TemplateRequest) (*Template, error) {
	data, err := c.Post("/v1/templates", req)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data Template `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse created template: %w", err)
	}
	return &resp.Data, nil
}

func (c *Client) UpdateTemplate(uuid string, req *TemplateRequest) (*Template, error) {
	data, err := c.Put("/v1/templates/"+uuid, req)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data Template `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse updated template: %w", err)
	}
	return &resp.Data, nil
}

func (c *Client) DeleteTemplate(uuid string) error {
	_, err := c.Delete("/v1/templates/" + uuid)
	return err
}
