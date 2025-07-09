package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"
	"backend/models"
	"backend/services"
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