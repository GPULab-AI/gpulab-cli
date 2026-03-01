package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	APIKey        string `json:"api_key"`
	APIURL        string `json:"api_url"`
	DefaultOutput string `json:"default_output"`
}

func DefaultConfig() *Config {
	return &Config{
		APIURL:        "https://gpulab.ai/api",
		DefaultOutput: "table",
	}
}

func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".gpulab")
}

func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.json")
}

func Load() (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return DefaultConfig(), err
	}

	// Apply env var override
	if envKey := os.Getenv("GPULAB_API_KEY"); envKey != "" {
		cfg.APIKey = envKey
	}
	if envURL := os.Getenv("GPULAB_API_URL"); envURL != "" {
		cfg.APIURL = envURL
	}

	return cfg, nil
}

func Save(cfg *Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(ConfigPath(), data, 0600)
}

func GetAPIKey(flagKey string) string {
	// Priority: flag > env > config file
	if flagKey != "" {
		return flagKey
	}
	if envKey := os.Getenv("GPULAB_API_KEY"); envKey != "" {
		return envKey
	}
	cfg, err := Load()
	if err != nil {
		return ""
	}
	return cfg.APIKey
}

func GetAPIURL(flagURL string) string {
	if flagURL != "" {
		return flagURL
	}
	if envURL := os.Getenv("GPULAB_API_URL"); envURL != "" {
		return envURL
	}
	cfg, err := Load()
	if err != nil {
		return "https://gpulab.ai/api"
	}
	if cfg.APIURL != "" {
		return cfg.APIURL
	}
	return "https://gpulab.ai/api"
}
