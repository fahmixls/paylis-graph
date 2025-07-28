package main

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const erc20ABI = `[{"anonymous":false,"inputs":[{"indexed":true,"name":"from","type":"address"},{"indexed":true,"name":"to","type":"address"},{"indexed":false,"name":"value","type":"uint256"}],"name":"Transfer","type":"event"}]`

type EventListener struct {
	client       *ethclient.Client
	db           *sql.DB
	config       *Config
	sseClients   map[chan SSEMessage]bool
	sseClientsMu sync.RWMutex
	transferSig  common.Hash
	contractABI  abi.ABI
}

func NewEventListener(cfg *Config) (*EventListener, error) {
	client, err := ethclient.Dial(cfg.EthereumRPC)
	if err != nil {
		return nil, fmt.Errorf("Ethereum client connection failed: %v", err)
	}

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("Database connection failed: %v", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("Database ping failed: %v", err)
	}

	contractABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return nil, fmt.Errorf("Failed to parse contract ABI: %v", err)
	}

	return &EventListener{
		client:      client,
		db:          db,
		config:      cfg,
		sseClients:  make(map[chan SSEMessage]bool),
		contractABI: contractABI,
		transferSig: crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)")),
	}, nil
}

func (el *EventListener) Close() error {
	if el.client != nil {
		el.client.Close()
	}
	if el.db != nil {
		el.db.Close()
	}
	return nil
}
