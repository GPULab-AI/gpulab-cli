package api

import (
	"encoding/json"
	"fmt"
)

type Account struct {
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	Team           *TeamInfo `json:"team"`
	ContainerCount int       `json:"container_count"`
}

type TeamInfo struct {
	Name string `json:"name"`
}

func (c *Client) GetAccount() (*Account, error) {
	data, err := c.Get("/v1/account")
	if err != nil {
		return nil, err
	}

	var account Account
	if err := json.Unmarshal(data, &account); err != nil {
		return nil, fmt.Errorf("failed to parse account: %w", err)
	}
	return &account, nil
}
