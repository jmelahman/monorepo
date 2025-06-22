package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	GitHub GitHubConfig `json:"github"`
	Git    GitConfig    `json:"git"`
}

type GitHubConfig struct {
	Repositories []string `json:"repositories"`
}

type GitConfig struct {
	Repositories []string `json:"repositories"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
