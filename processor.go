package main

import (
	"context"
	"fmt"
	"math/big"
	"time"
)

type TransactionProcessor struct {
	eth     *EthereumService
	db      *Database
	config  *Config
	chainID *big.Int
	running bool
}

func NewTransactionProcessor(config *Config) (*TransactionProcessor, error) {
	eth, err := NewEthereumService(config.RPC, config.Tokens)
	if err != nil {
		return nil, fmt.Errorf("failed to create Ethereum service: %w", err)
	}

	db, err := NewDatabase(config.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	return &TransactionProcessor{
		eth:    eth,
		db:     db,
		config: config,
	}, nil
}

func (tp *TransactionProcessor) Initialize(ctx context.Context) error {
	chainID, err := tp.eth.GetChainID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get chain ID: %w", err)
	}

	tp.chainID = chainID
	fmt.Printf("Connected to chain ID: %s\n", chainID.String())
	return nil
}

func (tp *TransactionProcessor) Start(ctx context.Context) error {
	tp.running = true
	fmt.Println("Starting transaction processor...")

	currentBlock, err := tp.eth.GetCurrentBlock(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current block: %w", err)
	}

	fromBlock := tp.config.From

	// Get last processed block from database
	lastProcessed, err := tp.db.GetLastProcessedBlock(ctx)
	if err != nil {
		fmt.Printf("Warning: failed to get last processed block: %v\n", err)
	} else if lastProcessed > fromBlock {
		fromBlock = lastProcessed + 1
	}

	fmt.Printf("Starting from block %d, current block: %d\n", fromBlock, currentBlock)

	for tp.running {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Process historical blocks in batches
		if fromBlock < currentBlock {
			toBlock := fromBlock + tp.config.Batch - 1
			if toBlock > currentBlock {
				toBlock = currentBlock
			}

			if err := tp.processBlockRange(ctx, fromBlock, toBlock); err != nil {
				fmt.Printf("Error processing block range %d-%d: %v\n", fromBlock, toBlock, err)
				time.Sleep(10 * time.Second)
				continue
			}

			fromBlock = toBlock + 1
		} else {
			// Wait for new blocks
			time.Sleep(5 * time.Second)
			newCurrentBlock, err := tp.eth.GetCurrentBlock(ctx)
			if err != nil {
				fmt.Printf("Error getting current block: %v\n", err)
				time.Sleep(10 * time.Second)
				continue
			}
			currentBlock = newCurrentBlock
		}
	}

	return nil
}

func (tp *TransactionProcessor) Stop() {
	fmt.Println("Stopping transaction processor...")
	tp.running = false
	tp.eth.Close()
	tp.db.Close()
}

func (tp *TransactionProcessor) processBlockRange(ctx context.Context, fromBlock, toBlock uint64) error {
	events, err := tp.eth.ProcessBlockRange(ctx, fromBlock, toBlock)
	if err != nil {
		return err
	}

	for _, event := range events {
		if err := tp.processTransferEvent(ctx, event); err != nil {
			fmt.Printf("Error processing transfer event %s: %v\n", event.TxHash.Hex(), err)
		}
	}

	return nil
}

func (tp *TransactionProcessor) processTransferEvent(ctx context.Context, event TransferEvent) error {
	receipt, err := tp.eth.GetTransactionReceipt(ctx, event.TxHash)
	if err != nil {
		return fmt.Errorf("failed to get transaction receipt: %w", err)
	}

	// Check confirmations
	currentBlock, err := tp.eth.GetCurrentBlock(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current block: %w", err)
	}

	confirmations := currentBlock - event.BlockNumber
	if confirmations < tp.config.Conf {
		fmt.Printf("Transaction %s has %d confirmations, waiting...\n", event.TxHash.Hex(), confirmations)
		return nil
	}

	tokenConfig, exists := tp.eth.GetTokenConfig(event.TokenAddress)
	if !exists {
		return nil
	}

	status := "failed"
	if receipt.Status == 1 {
		status = "confirmed"
	}

	now := time.Now()
	blockNumber := int64(event.BlockNumber)
	note := fmt.Sprintf("%s transfer", tokenConfig.Symbol)

	tx := &Transaction{
		PayerAddress:     event.From.Hex(),
		RecipientAddress: event.To.Hex(),
		TokenAddress:     event.TokenAddress.Hex(),
		Amount:           event.Value.String(),
		TxHash:           event.TxHash.Hex(),
		BlockNumber:      &blockNumber,
		ChainID:          tp.chainID.Int64(),
		Status:           status,
		Note:             &note,
		ConfirmedAt:      &now,
	}

	return tp.db.UpsertTransaction(ctx, tx)
}
