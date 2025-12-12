package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// EventBroker manages SSE clients
type EventBroker struct {
	clients    map[chan string]bool
	newClients chan chan string
	defunct    chan chan string
	messages   chan string
	mutex      sync.Mutex
}

func NewEventBroker() *EventBroker {
	broker := &EventBroker{
		clients:    make(map[chan string]bool),
		newClients: make(chan chan string),
		defunct:    make(chan chan string),
		messages:   make(chan string),
	}
	go broker.listen()
	go broker.startHeartbeat()
	return broker
}

func (b *EventBroker) listen() {
	for {
		select {
		case s := <-b.newClients:
			b.mutex.Lock()
			b.clients[s] = true
			b.mutex.Unlock()
			log.Printf("Client added. %d registered clients", len(b.clients))

		case s := <-b.defunct:
			b.mutex.Lock()
			delete(b.clients, s)
			close(s)
			b.mutex.Unlock()
			log.Printf("Client removed. %d registered clients", len(b.clients))

		case msg := <-b.messages:
			b.mutex.Lock()
			for s := range b.clients {
				select {
				case s <- msg:
				default:
					// If client channel is blocked, assume disconnected?
					// For now, we just skip to avoid blocking the broker
					log.Println("Skipping blocked client")
				}
			}
			b.mutex.Unlock()
		}
	}
}

func (b *EventBroker) startHeartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for t := range ticker.C {
		event := map[string]interface{}{
			"timestamp": t.Format(time.RFC3339),
			"clients":   len(b.clients),
		}
		b.Broadcast("heartbeat", event)
	}
}

func (b *EventBroker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Make sure that the writer supports flushing.
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	messageChan := make(chan string)
	b.newClients <- messageChan

	// Send initial connected event
	initialEvent := map[string]interface{}{
		"type":      "connected",
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   "Connected to BridgeGround Data Events",
	}
	initialJSON, _ := json.Marshal(initialEvent)
	fmt.Fprintf(w, "event: connected\ndata: %s\n\n", initialJSON)
	flusher.Flush()

	notify := r.Context().Done()

	for {
		select {
		case <-notify:
			b.defunct <- messageChan
			return
		case msg := <-messageChan:
			fmt.Fprintf(w, "%s", msg)
			flusher.Flush()
		}
	}
}

func (b *EventBroker) Broadcast(eventType string, data interface{}) {
	payload, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling event data: %v", err)
		return
	}
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, payload)
	b.messages <- msg
}

