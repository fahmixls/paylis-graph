package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func (el *EventListener) Start(ctx context.Context) error {
	log.Println("Listening for Transfer events...")

	addresses := make([]common.Address, len(el.config.TokenAddresses))
	for i, addr := range el.config.TokenAddresses {
		addresses[i] = common.HexToAddress(strings.TrimSpace(addr))
	}

	query := ethereum.FilterQuery{
		Addresses: addresses,
		Topics:    [][]common.Hash{{el.transferSig}},
	}

	logs := make(chan types.Log)
	sub, err := el.client.SubscribeFilterLogs(ctx, query, logs)
	if err != nil {
		return fmt.Errorf("log subscription failed: %v", err)
	}
	defer sub.Unsubscribe()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping event listener...")
			return nil
		case err := <-sub.Err():
			log.Printf("Subscription error: %v", err)
		case vLog := <-logs:
			if err := el.processTransferEvent(vLog); err != nil {
				log.Printf("Process error: %v", err)
			}
		}
	}
}
