package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"
	"backend/models"
	"backend/services"
	"backend/websocket"
)

// HandleStatus handles task status requests
func HandleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	// Extract UUID from URL path /status/{uuid}
	path := r.URL.Path
	if len(path) < 9 { // "/status/" is 8 characters
		http.Error(w, "Invalid UUID", http.StatusBadRequest)
		return
	}
	taskUUID := path[8:] // Remove "/status/" prefix

	// Validate UUID format
	if _, err := uuid.Parse(taskUUID); err != nil {
		http.Error(w, "Invalid UUID format", http.StatusBadRequest)
		return
	}

	// Get task status
	status, err := services.GetTaskStatus(taskUUID)
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// If task is done, also return the cached result
	var response models.TaskResponse
	if status == "done" {
		// Find the URL for this task by searching through url_task mappings
		urlTaskKeys, err := services.GetAllURLTaskKeys()
		if err != nil {
			log.Printf("Failed to get URL task keys: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		var taskURL string
		for _, key := range urlTaskKeys {
			urlTaskData, err := services.GetRedisClient().Get(services.GetContext(), key).Result()
			if err != nil {
				continue
			}
			var urlTask models.URLTaskMapping
			if err := json.Unmarshal([]byte(urlTaskData), &urlTask); err != nil {
				continue
			}
			if urlTask.UUID == taskUUID {
				taskURL = key[9:] // Remove "url_task:" prefix
				break
			}
		}

		if taskURL != "" {
			// Get cached result
			if cachedResult, err := services.CheckCache(taskURL); err == nil {
				response = models.TaskResponse{
					Status: status,
					UUID:   taskUUID,
					Result: cachedResult,
				}
			} else {
				response = models.TaskResponse{
					Status: status,
					UUID:   taskUUID,
				}
			}
		} else {
			response = models.TaskResponse{
				Status: status,
				UUID:   taskUUID,
			}
		}
	} else {
		response = models.TaskResponse{
			Status: status,
			UUID:   taskUUID,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleModelLoaded handles the notification that models are ready
func HandleModelLoaded(w http.ResponseWriter, r *http.Request, hub *websocket.Hub) {
	if r.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	log.Println("Received notification that models are loaded and ready")

	// Set a flag in Redis to indicate models are ready
	err := services.GetRedisClient().Set(services.GetContext(), "models:ready", "true", 0).Err()
	if err != nil {
		log.Printf("Failed to set models ready flag: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Broadcast to websocket clients that models are ready
	wsMessage := models.WSMessage{
		Type: "models_ready",
		Payload: map[string]interface{}{
			"ready": true,
		},
	}
	
	if messageBytes, err := json.Marshal(wsMessage); err == nil {
		hub.GetBroadcastChannel() <- messageBytes
		log.Printf("Broadcasted models ready message to WebSocket clients")
	} else {
		log.Printf("Failed to marshal WebSocket message: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleIsModelLoaded checks if models are loaded by querying the LLM server
func HandleIsModelLoaded(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	// Check Redis first (faster)
	redisFlag, err := services.GetRedisClient().Get(services.GetContext(), "models:ready").Result()
	if err == nil && redisFlag == "true" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ready": true,
			"source": "redis_cache",
		})
		return
	}

	// If not in Redis, check with LLM server
	llmServerURL := "http://llm-server:8000/is-model-loaded"
	resp, err := http.Get(llmServerURL)
	if err != nil {
		log.Printf("Failed to check model status with LLM server: %v", err)
		http.Error(w, "Failed to check model status", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("LLM server returned status %d", resp.StatusCode)
		http.Error(w, "LLM server error", http.StatusInternalServerError)
		return
	}

	// Forward the response from LLM server
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	
	// Copy the response body
	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		log.Printf("Failed to decode LLM server response: %v", err)
		http.Error(w, "Failed to decode response", http.StatusInternalServerError)
		return
	}
	
	// Add source information
	response["source"] = "llm_server"
	json.NewEncoder(w).Encode(response)
} 