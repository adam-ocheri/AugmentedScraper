package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"backend/models"
	"backend/websocket"
)

// HandleWebSocket handles WebSocket connections
func HandleWebSocket(w http.ResponseWriter, r *http.Request, hub *websocket.Hub) {
	conn, err := websocket.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	
	client := websocket.NewClient(hub, conn)
	hub.GetRegisterChannel() <- client
	
	// Send welcome message
	welcomeMessage := models.WSMessage{
		Type:    "connected",
		Payload: "WebSocket connection established",
	}
	if messageBytes, err := json.Marshal(welcomeMessage); err == nil {
		client.GetSendChannel() <- messageBytes
	}
	
	// Start goroutines for reading and writing
	go client.WritePump()
	go client.ReadPump()
} 