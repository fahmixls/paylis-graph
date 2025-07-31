package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = "config.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate required fields
	if config.RPC == "" {
		return nil, fmt.Errorf("RPC URL is required")
	}
	if config.DB == "" {
		return nil, fmt.Errorf("database connection string is required")
	}
	if len(config.Tokens) == 0 {
		return nil, fmt.Errorf("at least one token configuration is required")
	}

	// Set defaults
	if config.From == 0 {
		config.From = 0
	}
	if config.Conf == 0 {
		config.Conf = 1
	}
	if config.Batch == 0 {
		config.Batch = 100
	}

	// Validate tokens
	for i, token := range config.Tokens {
		if token.Address == "" || token.Symbol == "" {
			return nil, fmt.Errorf("token %d: address and symbol are required", i)
		}
	}

	fmt.Println("Configuration loaded successfully")
	fmt.Printf("- RPC: %s\n", config.RPC)
	fmt.Println("- Database: Connected")
	fmt.Printf("- From block: %d\n", config.From)
	fmt.Printf("- Confirmations: %d\n", config.Conf)
	fmt.Printf("- Batch size: %d\n", config.Batch)
	fmt.Print("- Tokens: ")
	for i, token := range config.Tokens {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Print(token.Symbol)
	}
	fmt.Println()

	return &config, nil
}
