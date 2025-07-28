package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (el *EventListener) broadcastSSE(msg SSEMessage) {
	el.sseClientsMu.RLock()
	defer el.sseClientsMu.RUnlock()

	for client := range el.sseClients {
		select {
		case client <- msg:
		default:
			delete(el.sseClients, client)
			close(client)
		}
	}
}

func (el *EventListener) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	client := make(chan SSEMessage, 10)
	el.sseClientsMu.Lock()
	el.sseClients[client] = true
	el.sseClientsMu.Unlock()

	defer func() {
		el.sseClientsMu.Lock()
		delete(el.sseClients, client)
		close(client)
		el.sseClientsMu.Unlock()
	}()

	fmt.Fprintf(w, "data: %s\n\n", `{"type":"connected","data":"SSE connection established"}`)
	w.(http.Flusher).Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-client:
			data, _ := json.Marshal(msg)
			fmt.Fprintf(w, "data: %s\n\n", data)
			w.(http.Flusher).Flush()
		}
	}
}
