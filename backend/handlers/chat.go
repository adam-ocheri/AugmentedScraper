package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"backend/models"
	"backend/websocket"
)

// HandleChat handles chat requests by forwarding them to the LLM server
func HandleChat(w http.ResponseWriter, r *http.Request, hub *websocket.Hub) {
	fmt.Println("Got chat request")

	if r.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	var req models.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.UUID == "" {
		http.Error(w, "UUID is required", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	fmt.Printf("Forwarding chat request for UUID: %s\n", req.UUID)

	// Forward request to LLM server
	llmServerURL := "http://llm-server:8000/chat"
	requestBody, err := json.Marshal(map[string]interface{}{
		"uuid":    req.UUID,
		"message": req.Message,
	})
	if err != nil {
		log.Printf("Failed to marshal chat request: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	resp, err := http.Post(llmServerURL, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		log.Printf("Failed to forward request to LLM server: %v", err)
		http.Error(w, "Failed to process chat request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Read response from LLM server
	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read LLM server response: %v", err)
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	// Check if LLM server returned an error
	if resp.StatusCode != http.StatusOK {
		log.Printf("LLM server returned error: %d - %s", resp.StatusCode, string(responseBody))
		http.Error(w, fmt.Sprintf("LLM server error: %s", string(responseBody)), resp.StatusCode)
		return
	}

	// Parse LLM server response
	var llmResponse map[string]interface{}
	if err := json.Unmarshal(responseBody, &llmResponse); err != nil {
		log.Printf("Failed to parse LLM server response: %v", err)
		http.Error(w, "Failed to parse response", http.StatusInternalServerError)
		return
	}

	// Create response for frontend
	response := models.ChatResponse{
		UUID:     req.UUID,
		Response: llmResponse["response"].(string),
		Success:  true,
	}

	// Broadcast chat response to WebSocket clients
	chatMessage := models.WSMessage{
		Type: "chat_response",
		Payload: map[string]interface{}{
			"uuid":     req.UUID,
			"response": llmResponse["response"].(string),
			"success":  true,
		},
	}

	if messageBytes, err := json.Marshal(chatMessage); err == nil {
		hub.GetBroadcastChannel() <- messageBytes
		fmt.Printf("Broadcasted chat response to WebSocket clients: %s\n", string(messageBytes))
	} else {
		log.Printf("Failed to marshal WebSocket chat message: %v", err)
	}

	// Return response to frontend
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
} 