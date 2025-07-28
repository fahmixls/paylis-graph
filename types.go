package main

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

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
