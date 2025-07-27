package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

const (
	erc20ABI = `[{"anonymous":false,"inputs":[{"indexed":true,"name":"from","type":"address"},{"indexed":true,"name":"to","type":"address"},{"indexed":false,"name":"value","type":"uint256"}],"name":"Transfer","type":"event"}]`
)

type Config struct {
	DatabaseURL    string
	EthereumRPC    string
	TokenAddresses []string
	ChainID        int64
	ServerPort     string
}

type TransferEvent struct {
	From        common.Address
	To          common.Address
	Value       *big.Int
	TxHash      string
	BlockNumber uint64
}

type SSEMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type TransferConfirmation struct {
	RecipientAddress string `json:"recipientAddress"`
	TokenAddress     string `json:"tokenAddress"`
	Amount           string `json:"amount"`
	TxHash           string `json:"txHash"`
	BlockNumber      uint64 `json:"blockNumber"`
	ConfirmedAt      string `json:"confirmedAt"`
}

type EventListener struct {
	client       *ethclient.Client
	db           *sql.DB
	config       *Config
	sseClients   map[chan SSEMessage]bool
	sseClientsMu sync.RWMutex
	transferSig  common.Hash
	contractABI  abi.ABI
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

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseInt64(s string) int64 {
	if s == "" {
		return 0
	}
	// Simple conversion for demo - in production, use strconv.ParseInt with error handling
	switch s {
	case "1":
		return 1
	case "137":
		return 137
	default:
		return 1
	}
}

func NewEventListener(config *Config) (*EventListener, error) {
	// Connect to Ethereum client
	client, err := ethclient.Dial(config.EthereumRPC)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum client: %v", err)
	}

	// Connect to database
	db, err := sql.Open("postgres", config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	// Parse contract ABI
	contractABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse contract ABI: %v", err)
	}

	// Transfer event signature
	transferSig := crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

	return &EventListener{
		client:      client,
		db:          db,
		config:      config,
		sseClients:  make(map[chan SSEMessage]bool),
		transferSig: transferSig,
		contractABI: contractABI,
	}, nil
}

func (el *EventListener) Start(ctx context.Context) error {
	log.Println("Starting event listener...")

	// Create filter query
	addresses := make([]common.Address, len(el.config.TokenAddresses))
	for i, addr := range el.config.TokenAddresses {
		addresses[i] = common.HexToAddress(strings.TrimSpace(addr))
	}

	query := ethereum.FilterQuery{
		Addresses: addresses,
		Topics:    [][]common.Hash{{el.transferSig}},
	}

	// Subscribe to logs
	logs := make(chan types.Log)
	sub, err := el.client.SubscribeFilterLogs(ctx, query, logs)
	if err != nil {
		return fmt.Errorf("failed to subscribe to logs: %v", err)
	}
	defer sub.Unsubscribe()

	log.Printf("Listening for Transfer events from %d token contracts", len(addresses))

	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled, stopping event listener")
			return ctx.Err()
		case err := <-sub.Err():
			log.Printf("Subscription error: %v", err)
			return err
		case vLog := <-logs:
			if err := el.processTransferEvent(vLog); err != nil {
				log.Printf("Error processing transfer event: %v", err)
			}
		}
	}
}

func (el *EventListener) processTransferEvent(vLog types.Log) error {
	// Parse the Transfer event
	event, err := el.parseTransferEvent(vLog)
	if err != nil {
		return fmt.Errorf("failed to parse transfer event: %v", err)
	}

	log.Printf("Transfer event: from=%s, to=%s, value=%s, tx=%s, block=%d",
		event.From.Hex(), event.To.Hex(), event.Value.String(), event.TxHash, event.BlockNumber)

	// Check if transaction exists in database by tx_hash
	exists, err := el.checkTransactionExists(event.TxHash)
	if err != nil {
		return fmt.Errorf("failed to check transaction: %v", err)
	}

	if !exists {
		log.Printf("Transaction %s not found in database, skipping", event.TxHash)
		return nil
	}

	// Update transaction
	if err := el.updateTransaction(event); err != nil {
		return fmt.Errorf("failed to update transaction: %v", err)
	}

	// Send SSE notification
	el.broadcastSSE(SSEMessage{
		Type: "transfer_confirmed",
		Data: TransferConfirmation{
			RecipientAddress: event.To.Hex(),
			TokenAddress:     vLog.Address.Hex(),
			Amount:           event.Value.String(),
			TxHash:           event.TxHash,
			BlockNumber:      event.BlockNumber,
			ConfirmedAt:      time.Now().Format(time.RFC3339),
		},
	})

	log.Printf("Transaction updated and SSE sent for tx_hash %s", event.TxHash)
	return nil
}

func (el *EventListener) parseTransferEvent(vLog types.Log) (*TransferEvent, error) {
	event := &TransferEvent{}

	// Parse the event data
	err := el.contractABI.UnpackIntoInterface(event, "Transfer", vLog.Data)
	if err != nil {
		return nil, err
	}

	// Extract indexed parameters (from and to addresses)
	if len(vLog.Topics) < 3 {
		return nil, fmt.Errorf("insufficient topics in log")
	}

	event.From = common.HexToAddress(vLog.Topics[1].Hex())
	event.To = common.HexToAddress(vLog.Topics[2].Hex())
	event.TxHash = vLog.TxHash.Hex()
	event.BlockNumber = vLog.BlockNumber

	// The Value is in the data field, let's extract it manually
	if len(vLog.Data) >= 32 {
		event.Value = new(big.Int).SetBytes(vLog.Data[0:32])
	}

	return event, nil
}

func (el *EventListener) checkTransactionExists(txHash string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM transactions WHERE tx_hash = $1 AND confirmed_at IS NULL)`
	err := el.db.QueryRow(query, txHash).Scan(&exists)
	return exists, err
}

func (el *EventListener) updateTransaction(event *TransferEvent) error {
	query := `
		UPDATE transactions 
		SET block_number = $1, status = 'confirmed', confirmed_at = $2
		WHERE tx_hash = $3 AND confirmed_at IS NULL`

	result, err := el.db.Exec(query,
		event.BlockNumber,
		time.Now(),
		event.TxHash,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	log.Printf("Updated %d transaction(s) for tx_hash %s", rowsAffected, event.TxHash)
	return nil
}

func (el *EventListener) addSSEClient(client chan SSEMessage) {
	el.sseClientsMu.Lock()
	defer el.sseClientsMu.Unlock()
	el.sseClients[client] = true
}

func (el *EventListener) removeSSEClient(client chan SSEMessage) {
	el.sseClientsMu.Lock()
	defer el.sseClientsMu.Unlock()
	delete(el.sseClients, client)
	close(client)
}

func (el *EventListener) broadcastSSE(message SSEMessage) {
	el.sseClientsMu.RLock()
	defer el.sseClientsMu.RUnlock()

	for client := range el.sseClients {
		select {
		case client <- message:
		default:
			// Client buffer is full, remove it
			delete(el.sseClients, client)
			close(client)
		}
	}
}

func (el *EventListener) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	// Create client channel
	client := make(chan SSEMessage, 10)
	el.addSSEClient(client)
	defer el.removeSSEClient(client)

	// Send initial connection message
	fmt.Fprintf(w, "data: %s\n\n", `{"type":"connected","data":"SSE connection established"}`)
	w.(http.Flusher).Flush()

	// Listen for messages and client disconnect
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case message := <-client:
			data, err := json.Marshal(message)
			if err != nil {
				log.Printf("Error marshaling SSE message: %v", err)
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			w.(http.Flusher).Flush()
		}
	}
}

func (el *EventListener) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check database connection
	if err := el.db.Ping(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	// Check Ethereum connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := el.client.BlockNumber(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "unhealthy",
			"error":  fmt.Sprintf("ethereum connection failed: %v", err),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"chainId":   el.config.ChainID,
		"contracts": el.config.TokenAddresses,
	})
}

func (el *EventListener) startHTTPServer(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/events", el.handleSSE)
	mux.HandleFunc("/health", el.handleHealth)

	server := &http.Server{
		Addr:    ":" + el.config.ServerPort,
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		log.Printf("HTTP server starting on port %s", el.config.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
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

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
	// Load configuration
	config := loadConfig()

	// Create event listener
	listener, err := NewEventListener(config)
	if err != nil {
		log.Fatalf("Failed to create event listener: %v", err)
	}
	defer listener.Close()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start HTTP server
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := listener.startHTTPServer(ctx); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Start event listener
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := listener.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("Event listener error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutdown signal received, stopping...")

	// Cancel context to stop all goroutines
	cancel()

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait for graceful shutdown or timeout
	select {
	case <-done:
		log.Println("Application stopped gracefully")
	case <-time.After(15 * time.Second):
		log.Println("Shutdown timeout exceeded, forcing exit")
	}
}
