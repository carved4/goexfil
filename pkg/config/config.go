package config

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
)

type Config struct {
	B2Config struct {
		Endpoint       string
		KeyID          string
		KeyName        string
		BucketName     string
		ApplicationKey string
	}

	Compression struct {
		Level int
	}

	Concurrency struct {
		Workers int
	}

	Buffer struct {
		Size int
	}
}

func NewDefaultConfig() *Config {
	cfg := &Config{}

	cfg.B2Config.Endpoint = "your backblaze bucket endpoint goes here"
	cfg.B2Config.KeyID = "your backblaze key id goes here"
	cfg.B2Config.KeyName = "name of key goes here"
	cfg.B2Config.BucketName = "name of bucket goes here"
	cfg.B2Config.ApplicationKey = "app key goes here"

	cfg.Compression.Level = 1

	cfg.Concurrency.Workers = runtime.NumCPU()

	cfg.Buffer.Size = 8 * 1024 * 1024

	return cfg
}

func LoadConfig(path string) (*Config, error) {

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {

			config := NewDefaultConfig()

			data, err := json.MarshalIndent(config, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("failed to marshal default config: %w", err)
			}

			if err := os.WriteFile(path, data, 0644); err != nil {
				return nil, fmt.Errorf("failed to write default config: %w", err)
			}

			return config, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if config.B2Config.KeyID == "" {
		return nil, fmt.Errorf("B2 Key ID is required")
	}
	if config.B2Config.ApplicationKey == "" {
		return nil, fmt.Errorf("B2 Application Key is required")
	}
	if config.B2Config.BucketName == "" {
		return nil, fmt.Errorf("B2 Bucket Name is required")
	}

	if config.Concurrency.Workers <= 0 {
		config.Concurrency.Workers = 4
	}

	return &config, nil
}
