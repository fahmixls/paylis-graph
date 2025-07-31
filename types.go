package main

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type Config struct {
	RPC    string        `yaml:"rpc"`
	DB     string        `yaml:"db"`
	From   uint64        `yaml:"from"`
	Conf   uint64        `yaml:"conf"`
	Batch  uint64        `yaml:"batch"`
	Tokens []TokenConfig `yaml:"tokens"`
}

type TokenConfig struct {
	Address  string `yaml:"address"`
	Symbol   string `yaml:"symbol"`
	Decimals int    `yaml:"decimals"`
}

type Transaction struct {
	ID               int        `db:"id"`
	TransactionID    *string    `db:"transaction_id"`
	PayerAddress     string     `db:"payer_address"`
	RecipientAddress string     `db:"recipient_address"`
	TokenAddress     string     `db:"token_address"`
	Amount           string     `db:"amount"`
	TxHash           string     `db:"tx_hash"`
	BlockNumber      *int64     `db:"block_number"`
	ChainID          int64      `db:"chain_id"`
	Status           string     `db:"status"`
	Note             *string    `db:"note"`
	CreatedAt        time.Time  `db:"created_at"`
	ConfirmedAt      *time.Time `db:"confirmed_at"`
}

type TransferEvent struct {
	From         common.Address
	To           common.Address
	Value        *big.Int
	TxHash       common.Hash
	BlockNumber  uint64
	TokenAddress common.Address
}
