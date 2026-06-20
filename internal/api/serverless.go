package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type ServerlessService struct {
	ID                         int     `json:"id"`
	UUID                       string  `json:"uuid"`
	Name                       string  `json:"name"`
	EndpointKey                string  `json:"endpoint_key"`
	EndpointURL                string  `json:"endpoint_url"`
	IsEnabled                  bool    `json:"is_enabled"`
	TemplateID                 int     `json:"template_id"`
	TemplateUUID               string  `json:"template_uuid"`
	TemplateName               string  `json:"template_name"`
	GPUTypeID                  int     `json:"gpu_type_id"`
	GPUTypeName                string  `json:"gpu_type_name"`
	RegionID                   string  `json:"region_id"`
	RegionName                 string  `json:"region_name"`
	RegionSlug                 string  `json:"region_slug"`
	NetworkVolumeID            *int    `json:"network_volume_id"`
	NetworkVolumeUUID          string  `json:"network_volume_uuid"`
	NetworkVolumeName          string  `json:"network_volume_name"`
	NetworkVolumeMaxSize       *int    `json:"network_volume_max_size"`
	TemplateVolumeMountPath    string  `json:"template_volume_mount_path"`
	GPUCount                   int     `json:"gpu_count"`
	Memory                     int     `json:"memory"`
	EndpointPort               int     `json:"endpoint_port"`
	HealthCheckPath            string  `json:"health_check_path"`
	MinReplicas                int     `json:"min_replicas"`
	MaxReplicas                int     `json:"max_replicas"`
	MaxConcurrentRequests      int     `json:"max_concurrent_requests"`
	AllowOverflowRequests      bool    `json:"allow_overflow_requests"`
	AutoscalingEnabled         bool    `json:"autoscaling_enabled"`
	AutoscalingTemplateKey     string  `json:"autoscaling_template_key"`
	AutoscalingPolicyCode      string  `json:"autoscaling_policy_code"`
	AutoscalingMetricsConfig   string  `json:"autoscaling_metrics_config"`
	AutoscalingLastEvaluatedAt string  `json:"autoscaling_last_evaluated_at"`
	AutoscalingLastErrorAt     string  `json:"autoscaling_last_error_at"`
	IdleTimeoutSeconds         int     `json:"idle_timeout_seconds"`
	ColdStartTimeoutSeconds    int     `json:"cold_start_timeout_seconds"`
	RequestTimeoutSeconds      int     `json:"request_timeout_seconds"`
	ImagePullPolicy            string  `json:"image_pull_policy"`
	CUDAVersion                string  `json:"cuda_version"`
	Command                    string  `json:"command"`
	EnvironmentVariables       string  `json:"environment_variables"`
	EnvironmentVariablesMap    JSONMap `json:"environment_variables_map"`
	Notes                      string  `json:"notes"`
	LastInvokedAt              string  `json:"last_invoked_at"`
	LastScaledAt               string  `json:"last_scaled_at"`
	CreatedAt                  string  `json:"created_at"`
	UpdatedAt                  string  `json:"updated_at"`
	ActiveReplicasCount        int     `json:"active_replicas_count"`
	ProvisioningReplicasCount  int     `json:"provisioning_replicas_count"`
	TotalRequestsCount         int     `json:"total_requests_count"`
	QueuedRequestsCount        int     `json:"queued_requests_count"`
}

type ServerlessReplica struct {
	ID         int               `json:"id"`
	UUID       string            `json:"uuid"`
	ServerName string            `json:"server_name"`
	Status     string            `json:"status"`
	HasLogs    bool              `json:"has_logs"`
	Uptime     string            `json:"uptime"`
	ServerID   *int              `json:"server_id"`
	Server     map[string]string `json:"server"`
	CreatedAt  string            `json:"created_at"`
	UpdatedAt  string            `json:"updated_at"`
}

type ServerlessBodyPayload struct {
	Encoding  string `json:"encoding"`
	Data      string `json:"data"`
	SizeBytes int64  `json:"size_bytes"`
}

type ServerlessRequestLog struct {
	UUID               string                 `json:"uuid"`
	ServiceName        string                 `json:"service_name"`
	Method             string                 `json:"method"`
	Path               string                 `json:"path"`
	QueryString        string                 `json:"query_string"`
	Status             string                 `json:"status"`
	ContentType        string                 `json:"content_type"`
	RequestSizeBytes   *int64                 `json:"request_size_bytes"`
	ResponseStatusCode *int                   `json:"response_status_code"`
	ResponseSizeBytes  *int64                 `json:"response_size_bytes"`
	DurationMS         *int64                 `json:"duration_ms"`
	ErrorMessage       string                 `json:"error_message"`
	QueuedAt           string                 `json:"queued_at"`
	StartedAt          string                 `json:"started_at"`
	CompletedAt        string                 `json:"completed_at"`
	RequestHeaders     JSONMap                `json:"request_headers"`
	RequestBody        *ServerlessBodyPayload `json:"request_body"`
	ResponseHeaders    JSONMap                `json:"response_headers"`
	ResponseBody       *ServerlessBodyPayload `json:"response_body"`
}

type ServerlessAutoscalingLog struct {
	ID              int     `json:"id"`
	Level           string  `json:"level"`
	Source          string  `json:"source"`
	Event           string  `json:"event"`
	Message         string  `json:"message"`
	CurrentReplicas *int    `json:"current_replicas"`
	DesiredReplicas *int    `json:"desired_replicas"`
	Metrics         JSONMap `json:"metrics"`
	Context         JSONMap `json:"context"`
	CreatedAt       string  `json:"created_at"`
}

type JSONMap map[string]interface{}

func (m *JSONMap) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*m = JSONMap{}
		return nil
	}

	var object map[string]interface{}
	if err := json.Unmarshal(data, &object); err == nil {
		if object == nil {
			object = map[string]interface{}{}
		}
		*m = object
		return nil
	}

	var list []interface{}
	if err := json.Unmarshal(data, &list); err == nil && len(list) == 0 {
		*m = JSONMap{}
		return nil
	}

	return fmt.Errorf("expected JSON object or empty array")
}

type ServerlessPage[T any] struct {
	Data        []T  `json:"data"`
	CurrentPage int  `json:"current_page"`
	LastPage    int  `json:"last_page"`
	PerPage     int  `json:"per_page"`
	Total       int  `json:"total"`
	HasNextPage bool `json:"has_next_page"`
	HasPrevPage bool `json:"has_prev_page"`
}

type ServerlessDetail struct {
	Data            ServerlessService                        `json:"data"`
	Requests        ServerlessPage[ServerlessRequestLog]     `json:"requests"`
	Replicas        []ServerlessReplica                      `json:"replicas"`
	AutoscalingLogs ServerlessPage[ServerlessAutoscalingLog] `json:"autoscaling_logs"`
}

type ServerlessTemplateOption struct {
	ID              int     `json:"id"`
	TemplateUUID    string  `json:"template_uuid"`
	Name            string  `json:"name"`
	DockerImage     string  `json:"docker_image"`
	ExposedPorts    *string `json:"exposed_ports"`
	MemoryLimit     *int    `json:"memory_limit"`
	VolumeMountPath *string `json:"volume_mount_path"`
}

type ServerlessGPUTypeOption struct {
	ID             int         `json:"id"`
	GPUName        string      `json:"gpu_name"`
	GPUTotalMemory int         `json:"gpu_total_memory"`
	GPUPrice       json.Number `json:"gpu_price"`
	FreeGPUsCount  int         `json:"free_gpus_count"`
}

type ServerlessRegionOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type ServerlessNetworkVolumeOption struct {
	ID         int    `json:"id"`
	VolumeUUID string `json:"volume_uuid"`
	VolumeName string `json:"volume_name"`
	RegionID   string `json:"region_id"`
	RegionName string `json:"region_name"`
	MaxSize    *int   `json:"max_size"`
}

type ServerlessAutoscalingTemplate struct {
	Key           string                   `json:"key"`
	Name          string                   `json:"name"`
	Summary       string                   `json:"summary"`
	PolicyCode    string                   `json:"policy_code"`
	MetricsConfig []map[string]interface{} `json:"metrics_config"`
}

type ServerlessOptions struct {
	Templates            []ServerlessTemplateOption      `json:"templates"`
	GPUTypes             []ServerlessGPUTypeOption       `json:"gpu_types"`
	Regions              []ServerlessRegionOption        `json:"regions"`
	NetworkVolumes       []ServerlessNetworkVolumeOption `json:"network_volumes"`
	AutoscalingTemplates []ServerlessAutoscalingTemplate `json:"autoscaling_templates"`
}

type ServerlessServiceRequest struct {
	Name                     string      `json:"name"`
	IsEnabled                bool        `json:"is_enabled"`
	TemplateID               int         `json:"template_id,omitempty"`
	Template                 string      `json:"template,omitempty"`
	GPUTypeID                int         `json:"gpu_type_id,omitempty"`
	GPUType                  string      `json:"gpu_type,omitempty"`
	RegionID                 string      `json:"region_id,omitempty"`
	Region                   string      `json:"region,omitempty"`
	NetworkVolumeID          *int        `json:"network_volume_id,omitempty"`
	NetworkVolume            string      `json:"network_volume,omitempty"`
	GPUCount                 int         `json:"gpu_count"`
	Memory                   int         `json:"memory"`
	EndpointPort             int         `json:"endpoint_port"`
	HealthCheckPath          string      `json:"health_check_path"`
	MinReplicas              int         `json:"min_replicas"`
	MaxReplicas              int         `json:"max_replicas"`
	MaxConcurrentRequests    int         `json:"max_concurrent_requests"`
	AllowOverflowRequests    bool        `json:"allow_overflow_requests"`
	AutoscalingEnabled       bool        `json:"autoscaling_enabled"`
	AutoscalingTemplateKey   string      `json:"autoscaling_template_key,omitempty"`
	AutoscalingPolicyCode    string      `json:"autoscaling_policy_code,omitempty"`
	AutoscalingMetricsConfig interface{} `json:"autoscaling_metrics_config,omitempty"`
	IdleTimeoutSeconds       int         `json:"idle_timeout_seconds"`
	ColdStartTimeoutSeconds  int         `json:"cold_start_timeout_seconds"`
	RequestTimeoutSeconds    int         `json:"request_timeout_seconds"`
	ImagePullPolicy          string      `json:"image_pull_policy"`
	CUDAVersion              string      `json:"cuda_version,omitempty"`
	EnvironmentVariables     interface{} `json:"environment_variables,omitempty"`
	Command                  string      `json:"command,omitempty"`
	Notes                    string      `json:"notes,omitempty"`
}

type ServerlessWriteResponse struct {
	Status  string            `json:"status"`
	Message string            `json:"message"`
	Data    ServerlessService `json:"data"`
}

type ServerlessPolicyValidationRequest struct {
	AutoscalingTemplateKey   string      `json:"autoscaling_template_key,omitempty"`
	AutoscalingPolicyCode    string      `json:"autoscaling_policy_code"`
	AutoscalingMetricsConfig interface{} `json:"autoscaling_metrics_config,omitempty"`
}

type ServerlessPolicyValidationResponse struct {
	Valid        bool     `json:"valid"`
	Message      string   `json:"message"`
	Errors       []string `json:"errors"`
	PolicyHash   string   `json:"policy_hash"`
	MetricsCount int      `json:"metrics_count"`
}

type ServerlessStatusResponse struct {
	RequestID   string                 `json:"request_id"`
	Service     map[string]string      `json:"service"`
	Status      string                 `json:"status"`
	Processing  bool                   `json:"processing"`
	Message     string                 `json:"message"`
	QueuedAt    string                 `json:"queued_at"`
	StartedAt   string                 `json:"started_at"`
	CompletedAt string                 `json:"completed_at"`
	Response    map[string]interface{} `json:"response"`
}

func (c *Client) ListServerlessServices() ([]ServerlessService, error) {
	data, err := c.Get("/v1/serverless")
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data []ServerlessService `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse serverless services: %w", err)
	}
	return resp.Data, nil
}

func (c *Client) GetServerlessOptions() (*ServerlessOptions, error) {
	data, err := c.Get("/v1/serverless/options")
	if err != nil {
		return nil, err
	}
	var options ServerlessOptions
	if err := json.Unmarshal(data, &options); err != nil {
		return nil, fmt.Errorf("failed to parse serverless options: %w", err)
	}
	return &options, nil
}

func (c *Client) GetServerlessService(identifier string) (*ServerlessDetail, error) {
	data, err := c.Get("/v1/serverless/" + url.PathEscape(identifier))
	if err != nil {
		return nil, err
	}
	var detail ServerlessDetail
	if err := json.Unmarshal(data, &detail); err != nil {
		return nil, fmt.Errorf("failed to parse serverless service: %w", err)
	}
	return &detail, nil
}

func (c *Client) CreateServerlessService(req *ServerlessServiceRequest) (*ServerlessWriteResponse, error) {
	data, err := c.Post("/v1/serverless", req)
	if err != nil {
		return nil, err
	}
	var resp ServerlessWriteResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse serverless create response: %w", err)
	}
	return &resp, nil
}

func (c *Client) UpdateServerlessService(identifier string, req *ServerlessServiceRequest) (*ServerlessWriteResponse, error) {
	data, err := c.Put("/v1/serverless/"+url.PathEscape(identifier), req)
	if err != nil {
		return nil, err
	}
	var resp ServerlessWriteResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse serverless update response: %w", err)
	}
	return &resp, nil
}

func (c *Client) DeleteServerlessService(identifier string) error {
	_, err := c.Delete("/v1/serverless/" + url.PathEscape(identifier))
	return err
}

func (c *Client) ListServerlessRequests(identifier string, page, perPage int) (*ServerlessPage[ServerlessRequestLog], error) {
	params := url.Values{}
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}
	if perPage > 0 {
		params.Set("per_page", strconv.Itoa(perPage))
	}
	path := "/v1/serverless/" + url.PathEscape(identifier) + "/requests"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	var resp ServerlessPage[ServerlessRequestLog]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse serverless requests: %w", err)
	}
	return &resp, nil
}

func (c *Client) ListServerlessAutoscalingLogs(identifier string, page, perPage int) (*ServerlessPage[ServerlessAutoscalingLog], error) {
	params := url.Values{}
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}
	if perPage > 0 {
		params.Set("per_page", strconv.Itoa(perPage))
	}
	path := "/v1/serverless/" + url.PathEscape(identifier) + "/autoscaling-logs"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	var resp ServerlessPage[ServerlessAutoscalingLog]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse serverless autoscaling logs: %w", err)
	}
	return &resp, nil
}

func (c *Client) CancelServerlessRequest(uuid string) error {
	_, err := c.Delete("/v1/serverless/requests/" + url.PathEscape(uuid))
	return err
}

type ServerlessReplicaActionResponse struct {
	Status                 string `json:"status"`
	Message                string `json:"message"`
	UUID                   string `json:"uuid"`
	ReplacementProvisioned bool   `json:"replacement_provisioned"`
}

func (c *Client) RestartServerlessReplica(identifier, replicaUUID string) (*ServerlessReplicaActionResponse, error) {
	path := "/v1/serverless/" + url.PathEscape(identifier) + "/replicas/" + url.PathEscape(replicaUUID) + "/restart"
	data, err := c.Post(path, nil)
	if err != nil {
		return nil, err
	}
	var resp ServerlessReplicaActionResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse restart response: %w", err)
	}
	return &resp, nil
}

func (c *Client) DeleteServerlessReplica(identifier, replicaUUID string) error {
	path := "/v1/serverless/" + url.PathEscape(identifier) + "/replicas/" + url.PathEscape(replicaUUID)
	_, err := c.Delete(path)
	return err
}

func (c *Client) ValidateServerlessPolicy(req *ServerlessPolicyValidationRequest) (*ServerlessPolicyValidationResponse, error) {
	data, err := c.Post("/v1/serverless/autoscaling-policies/validate", req)
	if err != nil {
		return nil, err
	}
	var resp ServerlessPolicyValidationResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse serverless policy validation response: %w", err)
	}
	return &resp, nil
}

func (c *Client) GetServerlessRequestStatus(uuid string) (*ServerlessStatusResponse, error) {
	data, err := c.Get("/serverless/requests/" + url.PathEscape(uuid))
	if err != nil {
		return nil, err
	}
	var resp ServerlessStatusResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse serverless request status: %w", err)
	}
	return &resp, nil
}

func (c *Client) ResolveServerlessService(partial string) (string, error) {
	if len(partial) < 3 {
		return "", fmt.Errorf("serverless identifier prefix too short (minimum 3 characters)")
	}

	services, err := c.ListServerlessServices()
	if err != nil {
		return "", err
	}

	var matches []ServerlessService
	for _, service := range services {
		if service.UUID == partial || service.EndpointKey == partial {
			return service.UUID, nil
		}
		if strings.HasPrefix(service.UUID, partial) ||
			strings.HasPrefix(service.EndpointKey, partial) ||
			strings.HasPrefix(strings.ToLower(service.Name), strings.ToLower(partial)) {
			matches = append(matches, service)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no serverless endpoint found with identifier %q", partial)
	case 1:
		return matches[0].UUID, nil
	default:
		msg := fmt.Sprintf("ambiguous serverless identifier %q matches %d endpoints:\n", partial, len(matches))
		for _, match := range matches {
			msg += fmt.Sprintf("  %s  %s  %s\n", match.UUID, match.EndpointKey, match.Name)
		}
		return "", fmt.Errorf("%s", msg)
	}
}
