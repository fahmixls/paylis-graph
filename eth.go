package main

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var transferEventSignature = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

type EthereumService struct {
	client       *ethclient.Client
	tokenConfigs map[common.Address]TokenConfig
}

func NewEthereumService(rpcURL string, tokens []TokenConfig) (*EthereumService, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum client: %w", err)
	}

	tokenConfigs := make(map[common.Address]TokenConfig)
	for _, token := range tokens {
		addr := common.HexToAddress(token.Address)
		tokenConfigs[addr] = token
	}

	return &EthereumService{
		client:       client,
		tokenConfigs: tokenConfigs,
	}, nil
}

func (e *EthereumService) GetCurrentBlock(ctx context.Context) (uint64, error) {
	return e.client.BlockNumber(ctx)
}

func (e *EthereumService) GetChainID(ctx context.Context) (*big.Int, error) {
	return e.client.ChainID(ctx)
}

func (e *EthereumService) ProcessBlockRange(ctx context.Context, fromBlock, toBlock uint64) ([]TransferEvent, error) {
	var events []TransferEvent

	// Get token addresses
	var addresses []common.Address
	for addr := range e.tokenConfigs {
		addresses = append(addresses, addr)
	}

	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(fromBlock)),
		ToBlock:   big.NewInt(int64(toBlock)),
		Addresses: addresses,
		Topics:    [][]common.Hash{{transferEventSignature}},
	}

	logs, err := e.client.FilterLogs(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to filter logs: %w", err)
	}

	for _, log := range logs {
		event, err := e.parseTransferLog(log)
		if err != nil {
			fmt.Printf("Error parsing log: %v\n", err)
			continue
		}
		if event != nil {
			events = append(events, *event)
		}
	}

	return events, nil
}

func (e *EthereumService) parseTransferLog(log types.Log) (*TransferEvent, error) {
	// Verify this is a Transfer event
	if len(log.Topics) < 3 || log.Topics[0] != transferEventSignature {
		return nil, nil
	}

	// Check if this token is in our config
	_, exists := e.tokenConfigs[log.Address]
	if !exists {
		return nil, nil
	}

	// Parse Transfer event: Transfer(address indexed from, address indexed to, uint256 value)
	from := common.BytesToAddress(log.Topics[1].Bytes())
	to := common.BytesToAddress(log.Topics[2].Bytes())
	value := new(big.Int).SetBytes(log.Data)

	return &TransferEvent{
		From:         from,
		To:           to,
		Value:        value,
		TxHash:       log.TxHash,
		BlockNumber:  log.BlockNumber,
		TokenAddress: log.Address,
	}, nil
}

func (e *EthereumService) GetTransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	return e.client.TransactionReceipt(ctx, txHash)
}

func (e *EthereumService) Close() {
	e.client.Close()
}

func (e *EthereumService) GetTokenConfig(address common.Address) (TokenConfig, bool) {
	config, exists := e.tokenConfigs[address]
	return config, exists
}
