package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ── SSE broker for real-time knowledge library updates ──

// sseClient represents a connected SSE subscriber.
type sseClient struct {
	ch   chan []byte
	done <-chan struct{}
}

// sseBroker manages SSE client connections and event broadcasting.
type sseBroker struct {
	mu          sync.Mutex
	clients     map[*sseClient]struct{}
	register    chan *sseClient
	unregister  chan *sseClient
	broadcastCh chan []byte
}

func newSSEBroker() *sseBroker {
	b := &sseBroker{
		clients:     make(map[*sseClient]struct{}),
		register:    make(chan *sseClient),
		unregister:  make(chan *sseClient),
		broadcastCh: make(chan []byte, 64),
	}
	go b.run()
	return b
}

func (b *sseBroker) run() {
	for {
		select {
		case c := <-b.register:
			b.mu.Lock()
			b.clients[c] = struct{}{}
			b.mu.Unlock()
		case c := <-b.unregister:
			b.mu.Lock()
			delete(b.clients, c)
			b.mu.Unlock()
		case msg := <-b.broadcastCh:
			b.mu.Lock()
			for c := range b.clients {
				select {
				case c.ch <- msg:
				default:
					// Client buffer full — skip to avoid blocking broadcast.
				}
			}
			b.mu.Unlock()
		}
	}
}

func (b *sseBroker) addClient(ctx context.Context) *sseClient {
	c := &sseClient{
		ch:   make(chan []byte, 8),
		done: ctx.Done(),
	}
	b.register <- c
	return c
}

func (b *sseBroker) removeClient(c *sseClient) {
	b.unregister <- c
}

func (b *sseBroker) broadcast(event string, data []byte) {
	payload := fmt.Appendf(nil, "event: %s\ndata: %s\n\n", event, data)
	b.broadcastCh <- payload
}

func (a *App) broadcastLibraryEvent(event string, body any) {
	if a.sseBroker == nil {
		return
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return
	}
	a.sseBroker.broadcast(event, raw)
}

func (a *App) handleKnowledgeLibraryEvents(w http.ResponseWriter, r *http.Request) {
	if a.sseBroker == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "SSE broker not initialized"})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming not supported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	client := a.sseBroker.addClient(r.Context())
	defer a.sseBroker.removeClient(client)

	// Send initial connected event so frontend knows the stream is ready.
	fmt.Fprintf(w, "event: connected\ndata: %s\n\n", time.Now().UTC().Format(time.RFC3339))
	flusher.Flush()

	for {
		select {
		case msg := <-client.ch:
			if _, err := w.Write(msg); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
