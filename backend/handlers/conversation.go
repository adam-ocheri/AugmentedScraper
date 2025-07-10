package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"backend/models"
	"backend/services"
)

// HandleConversationUpdate handles conversation update requests
func HandleConversationUpdate(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Got conversation update request")

	if r.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	var req models.ConversationUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.UUID == "" {
		http.Error(w, "UUID is required", http.StatusBadRequest)
		return
	}

	if req.Conversation == nil {
		req.Conversation = []models.ConversationEntry{} // Initialize empty conversation
	}

	fmt.Printf("Updating conversation for UUID: %s with %d entries\n", req.UUID, len(req.Conversation))

	// 1. Update conversation in the database
	conversationJSON, err := json.Marshal(req)
	if err != nil {
		log.Printf("Failed to marshal conversation update request: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = services.UpdateConversationInDBService(req.UUID, string(conversationJSON))
	if err != nil {
		log.Printf("Failed to update conversation in database: %v", err)
		http.Error(w, "Failed to update conversation in database", http.StatusInternalServerError)
		return
	}

	// 2. Check if the article is currently cached and update cache if so
	err = services.UpdateConversationInCache(req.UUID, req.Conversation)
	if err != nil {
		// This is not a critical error - the article might not be cached
		fmt.Printf("Warning: Could not update conversation in cache: %v\n", err)
		fmt.Printf("This is normal if the article is not currently cached\n")
	} else {
		fmt.Printf("Successfully updated conversation in cache for UUID: %s\n", req.UUID)
	}

	// 3. Return success response
	response := models.ConversationUpdateResponse{
		Success: true,
		Message: "Conversation updated successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
} 