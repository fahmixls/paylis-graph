package main

import (
	"os"
	"strings"
)

type Config struct {
	DatabaseURL    string
	EthereumRPC    string
	TokenAddresses []string
	ChainID        int64
	ServerPort     string
}

func loadConfig() *Config {
	return &Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://user:password@localhost/dbname?sslmode=disable"),
		EthereumRPC: getEnv("ETHEREUM_RPC", "wss://mainnet.infura.io/ws/v3/YOUR_PROJECT_ID"),
		TokenAddresses: strings.Split(
			getEnv("TOKEN_ADDRESSES", "0xA0b86a33E6441ecDe3E3a7Bd5bCa2e5FE8e30E5B,0xdAC17F958D2ee523a2206206994597C13D831ec7"),
			",",
		),
		ChainID:    parseInt64(getEnv("CHAIN_ID", "1")),
		ServerPort: getEnv("SERVER_PORT", "8080"),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func parseInt64(val string) int64 {
	switch val {
	case "1":
		return 1
	case "137":
		return 137
	default:
		return 1
	}
}
