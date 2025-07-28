package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (el *EventListener) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := el.db.Ping(); err != nil {
		http.Error(w, `{"status":"unhealthy","error":"db"}`, http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if _, err := el.client.BlockNumber(ctx); err != nil {
		http.Error(w, fmt.Sprintf(`{"status":"unhealthy","error":"eth: %v"}`, err), http.StatusServiceUnavailable)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"chainId":   el.config.ChainID,
		"contracts": el.config.TokenAddresses,
	})
}
