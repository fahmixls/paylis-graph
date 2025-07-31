package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	pool *pgxpool.Pool
}

func NewDatabase(connectionString string) (*Database, error) {
	pool, err := pgxpool.New(context.Background(), connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Database{pool: pool}, nil
}

func (db *Database) UpsertTransaction(ctx context.Context, tx *Transaction) error {
	query := `
		INSERT INTO transactions (
			transaction_id, payer_address, recipient_address, token_address,
			amount, tx_hash, block_number, chain_id, status, note, confirmed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (tx_hash) DO UPDATE SET
			payer_address = EXCLUDED.payer_address,
			recipient_address = EXCLUDED.recipient_address,
			token_address = EXCLUDED.token_address,
			amount = EXCLUDED.amount,
			block_number = EXCLUDED.block_number,
			status = EXCLUDED.status,
			note = EXCLUDED.note,
			confirmed_at = EXCLUDED.confirmed_at
	`

	_, err := db.pool.Exec(ctx, query,
		tx.TransactionID,
		tx.PayerAddress,
		tx.RecipientAddress,
		tx.TokenAddress,
		tx.Amount,
		tx.TxHash,
		tx.BlockNumber,
		tx.ChainID,
		tx.Status,
		tx.Note,
		tx.ConfirmedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert transaction %s: %w", tx.TxHash, err)
	}

	fmt.Printf("Upserted transaction: %s\n", tx.TxHash)
	return nil
}

func (db *Database) GetLastProcessedBlock(ctx context.Context) (uint64, error) {
	var blockNumber *int64
	query := `SELECT MAX(block_number) FROM transactions WHERE block_number IS NOT NULL`

	err := db.pool.QueryRow(ctx, query).Scan(&blockNumber)
	if err != nil {
		return 0, fmt.Errorf("failed to get last processed block: %w", err)
	}

	if blockNumber == nil {
		return 0, nil
	}

	return uint64(*blockNumber), nil
}

func (db *Database) Close() {
	db.pool.Close()
}
