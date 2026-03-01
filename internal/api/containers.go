package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

type Container struct {
	UUID                 string      `json:"uuid"`
	ServerName           string      `json:"server_name"`
	Status               string      `json:"status"`
	Type                 string      `json:"type"`
	Memory               *int        `json:"memory"`
	AllocatedCPUCores    *int        `json:"allocated_cpu_cores"`
	PublicURLs           *string     `json:"public_urls"`
	OpenedPorts          *string     `json:"opened_ports"`
	EnvironmentVariables interface{} `json:"environment_variables"`
	VolumeMountPath      *string     `json:"volume_mount_path"`
	Command              *string     `json:"command"`
	WebTerminal          interface{} `json:"web_terminal"`
	Uptime               *string     `json:"uptime"`
	GPUCount             *int        `json:"gpu_count"`
	GPUs                 []GPU       `json:"gpus"`
	CreatedAt            string      `json:"created_at"`
	UpdatedAt            string      `json:"updated_at"`
}

type GPU struct {
	GPUUuid     string      `json:"gpu_uuid"`
	GPUIndex    string      `json:"gpu_index"`
	GPUStatus   string      `json:"gpu_status"`
	Temperature *string     `json:"gpu_temperature"`
	MemoryUsed  json.Number `json:"memory_used"`
	TotalMemory json.Number `json:"total_memory"`
	Type        interface{} `json:"type"`
}

type ContainerDetail struct {
	Container
	Template      interface{} `json:"template"`
	NetworkVolume interface{} `json:"network_volume"`
	GpuType       interface{} `json:"gpu_type"`
	Logs          *string     `json:"logs,omitempty"`
}

type CreateContainerRequest struct {
	ServerName           string      `json:"server_name"`
	Type                 string      `json:"type"`
	GPUType              string      `json:"gpu_type"`
	TemplateUUID         string      `json:"template_uuid"`
	NetworkVolumeUUID    string      `json:"network_volume_uuid,omitempty"`
	OpenedPorts          string      `json:"opened_ports,omitempty"`
	EnvironmentVariables interface{} `json:"environment_variables,omitempty"`
	VolumeMountPath      string      `json:"volume_mount_path,omitempty"`
	Command              string      `json:"command,omitempty"`
	Memory               *int        `json:"memory,omitempty"`
	ImagePullPolicy      string      `json:"image_pull_policy,omitempty"`
}

type CreateContainerResponse struct {
	Status       string `json:"status"`
	Message      string `json:"message"`
	ContainerID  string `json:"container_id"`
	ServerName   string `json:"server_name"`
	ServerType   string `json:"server_type"`
	ServerStatus string `json:"server_status"`
}

type ContainerStats struct {
	CPUPercentage    float64 `json:"cpu_percentage"`
	MemoryUsage      int64   `json:"memory_usage"`
	MemoryLimit      int64   `json:"memory_limit"`
	MemoryPercentage float64 `json:"memory_percentage"`
	MemoryCache      int64   `json:"memory_cache"`
	NetworkRx        int64   `json:"network_rx"`
	NetworkTx        int64   `json:"network_tx"`
	BlockRead        int64   `json:"block_read"`
	BlockWrite       int64   `json:"block_write"`
	PIDs             int     `json:"pids"`
	Timestamp        string  `json:"timestamp"`
}

type ExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

type TerminalInfo struct {
	Status   string `json:"status"`
	Terminal struct {
		TerminalURL   string `json:"terminal_url"`
		TerminalPort  int    `json:"terminal_port"`
		ContainerUUID string `json:"container_uuid"`
	} `json:"terminal"`
}

func (c *Client) ListContainers() ([]Container, error) {
	data, err := c.Get("/v1/containers")
	if err != nil {
		return nil, err
	}

	var containers []Container
	if err := json.Unmarshal(data, &containers); err != nil {
		return nil, fmt.Errorf("failed to parse containers: %w", err)
	}
	return containers, nil
}

func (c *Client) GetContainer(uuid string) (*ContainerDetail, error) {
	data, err := c.Get("/v1/containers/" + uuid)
	if err != nil {
		return nil, err
	}

	var container ContainerDetail
	if err := json.Unmarshal(data, &container); err != nil {
		return nil, fmt.Errorf("failed to parse container: %w", err)
	}
	return &container, nil
}

func (c *Client) CreateContainer(req *CreateContainerRequest) (*CreateContainerResponse, error) {
	data, err := c.Post("/v1/containers", req)
	if err != nil {
		return nil, err
	}

	var resp CreateContainerResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &resp, nil
}

func (c *Client) DeleteContainer(uuid string) error {
	_, err := c.Delete("/v1/containers/" + uuid)
	return err
}

func (c *Client) StopContainer(uuid string) error {
	_, err := c.Post("/v1/containers/"+uuid+"/stop", nil)
	return err
}

func (c *Client) StartContainer(uuid string) error {
	_, err := c.Post("/v1/containers/"+uuid+"/start", nil)
	return err
}

func (c *Client) RestartContainer(uuid string) error {
	_, err := c.Post("/v1/containers/"+uuid+"/restart", nil)
	return err
}

func (c *Client) RedeployContainer(uuid string) error {
	_, err := c.Post("/v1/containers/"+uuid+"/redeploy", nil)
	return err
}

func (c *Client) GetContainerLogs(uuid string, tail int, since string, timestamps bool) (string, error) {
	params := url.Values{}
	if tail > 0 {
		params.Set("tail", strconv.Itoa(tail))
	}
	if since != "" {
		params.Set("since", since)
	}
	if timestamps {
		params.Set("timestamps", "true")
	}

	path := "/v1/containers/" + uuid + "/logs"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	data, err := c.Get(path)
	if err != nil {
		return "", err
	}

	var resp struct {
		Logs      string `json:"logs"`
		Timestamp int64  `json:"timestamp"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("failed to parse logs: %w", err)
	}
	return resp.Logs, nil
}

func (c *Client) GetDeploymentLogs(uuid string) (string, error) {
	data, err := c.Get("/v1/containers/" + uuid + "/logs/deploy")
	if err != nil {
		return "", err
	}

	var resp struct {
		Logs string `json:"logs"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("failed to parse logs: %w", err)
	}
	return resp.Logs, nil
}

func (c *Client) GetContainerStats(uuid string) (*ContainerStats, error) {
	data, err := c.Get("/v1/containers/" + uuid + "/stats")
	if err != nil {
		return nil, err
	}

	var resp struct {
		Success bool            `json:"success"`
		Stats   *ContainerStats `json:"stats"`
		Message string          `json:"message"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse stats: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Message)
	}
	return resp.Stats, nil
}

func (c *Client) ExecCommand(uuid, command string, timeout int) (*ExecResult, error) {
	body := map[string]interface{}{
		"command": command,
		"timeout": timeout,
	}

	data, err := c.Post("/v1/containers/"+uuid+"/exec", body)
	if err != nil {
		return nil, err
	}

	var result ExecResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse exec result: %w", err)
	}
	return &result, nil
}

func (c *Client) StartTerminal(uuid string) (*TerminalInfo, error) {
	data, err := c.Post("/v1/containers/"+uuid+"/terminal", nil)
	if err != nil {
		return nil, err
	}

	var info TerminalInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to parse terminal info: %w", err)
	}
	return &info, nil
}

// ResolveContainerUUID resolves a partial UUID to a full UUID by listing all containers
func (c *Client) ResolveContainerUUID(partial string) (string, error) {
	if len(partial) < 6 {
		return "", fmt.Errorf("UUID prefix too short (minimum 6 characters)")
	}

	containers, err := c.ListContainers()
	if err != nil {
		return "", err
	}

	var matches []Container
	for _, ct := range containers {
		if len(ct.UUID) >= len(partial) && ct.UUID[:len(partial)] == partial {
			matches = append(matches, ct)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no container found with UUID prefix %q", partial)
	case 1:
		return matches[0].UUID, nil
	default:
		msg := fmt.Sprintf("ambiguous UUID prefix %q matches %d containers:\n", partial, len(matches))
		for _, m := range matches {
			msg += fmt.Sprintf("  %s  %s  %s\n", m.UUID, m.ServerName, m.Status)
		}
		return "", fmt.Errorf("%s", msg)
	}
}
