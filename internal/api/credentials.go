package api

import (
	"encoding/json"
	"fmt"
)

// Credential is a stored Docker registry credential. The password is write-only
// and never returned by the API; templates reference a credential by its ID.
type Credential struct {
	ID        int    `json:"id"`
	Registry  string `json:"registry"`
	Username  string `json:"username"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// CreateCredentialRequest is the payload for storing a new registry credential.
// Registry defaults to docker.io server-side when left empty.
type CreateCredentialRequest struct {
	Registry string `json:"registry,omitempty"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (c *Client) ListCredentials() ([]Credential, error) {
	data, err := c.Get("/v1/credentials")
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []Credential `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}
	return resp.Data, nil
}

func (c *Client) CreateCredential(req *CreateCredentialRequest) (*Credential, error) {
	data, err := c.Post("/v1/credentials", req)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data Credential `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse created credential: %w", err)
	}
	return &resp.Data, nil
}

func (c *Client) DeleteCredential(id int) error {
	_, err := c.Delete(fmt.Sprintf("/v1/credentials/%d", id))
	return err
}
