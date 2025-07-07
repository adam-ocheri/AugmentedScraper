package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()
var rdb *redis.Client

type ArticleRequest struct {
	URL string `json:"url"`
}

type TaskResponse struct {
	Status string `json:"status"`
	UUID   string `json:"uuid,omitempty"`
	Result string `json:"result,omitempty"`
}

type TaskPayload struct {
	URL  string `json:"url"`
	UUID string `json:"uuid"`
}

type URLTaskMapping struct {
	UUID   string `json:"uuid"`
	Status string `json:"status"`
}

type ProcessResult struct {
	UUID   string                 `json:"uuid"`
	URL    string                 `json:"url"`
	Result map[string]interface{} `json:"result"`
}

func main() {
	rdb = redis.NewClient(&redis.Options{
		Addr: "redis:6379", // container name
	})

	// Start the Redis pub/sub subscriber
	go startResultSubscriber()

	http.HandleFunc("/submit", handleSubmit)
	http.HandleFunc("/status/", handleStatus)

	fmt.Println("Go API running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func startResultSubscriber() {
	fmt.Println("Starting Redis pub/sub subscriber for process:results...")
	
	pubsub := rdb.Subscribe(ctx, "process:results")
	defer pubsub.Close()
	
	ch := pubsub.Channel()
	
	for msg := range ch {
		fmt.Printf("Received result from LLM server: %s\n", msg.Payload)
		
		// Optionally parse and log the structured result
		var result ProcessResult
		if err := json.Unmarshal([]byte(msg.Payload), &result); err != nil {
			log.Printf("Failed to parse result message: %v", err)
			continue
		}
		
		fmt.Printf("Task %s completed for URL: %s\n", result.UUID, result.URL)
		fmt.Printf("Result summary: %v\n", result.Result["summary"])
	}
}

func handleSubmit(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Got request")

	if r.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	var req ArticleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Check if URL is empty
	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// 1. Check if the URL has already been processed and cached
	cacheKey := "cache:" + req.URL
	cachedResult, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		// URL is cached, return the cached result
		fmt.Printf("Cache hit for URL: %s\n", req.URL)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TaskResponse{
			Status: "done",
			Result: cachedResult,
		})
		return
	}

	// 2. Check if there's already a task in progress for this URL
	urlTaskKey := "url_task:" + req.URL
	urlTaskData, err := rdb.Get(ctx, urlTaskKey).Result()
	if err == nil {
		// URL is already being processed, return existing task info
		var urlTask URLTaskMapping
		if err := json.Unmarshal([]byte(urlTaskData), &urlTask); err != nil {
			log.Printf("Failed to unmarshal URL task mapping: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		fmt.Printf("Task already in progress for URL: %s, UUID: %s, Status: %s\n", req.URL, urlTask.UUID, urlTask.Status)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TaskResponse{
			Status: urlTask.Status,
			UUID:   urlTask.UUID,
		})
		return
	}

	// 3. URL not cached and no task in progress, create new task
	taskUUID := uuid.New().String()
	fmt.Printf("Creating new task for URL: %s, UUID: %s\n", req.URL, taskUUID)

	// 4. Store task status as "pending"
	statusKey := "status:" + taskUUID
	if err := rdb.Set(ctx, statusKey, "pending", 0).Err(); err != nil {
		log.Printf("Failed to set task status: %v", err)
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
		return
	}

	// 5. Immediately cache the URL-to-task mapping to prevent duplicates
	urlTaskMapping := URLTaskMapping{
		UUID:   taskUUID,
		Status: "pending",
	}
	urlTaskMappingData, err := json.Marshal(urlTaskMapping)
	if err != nil {
		log.Printf("Failed to marshal URL task mapping: %v", err)
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
		return
	}

	if err := rdb.Set(ctx, urlTaskKey, urlTaskMappingData, 0).Err(); err != nil {
		log.Printf("Failed to set URL task mapping: %v", err)
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
		return
	}

	// 6. Create task payload and add to queue
	taskPayload := TaskPayload{
		URL:  req.URL,
		UUID: taskUUID,
	}

	taskData, err := json.Marshal(taskPayload)
	if err != nil {
		log.Printf("Failed to marshal task payload: %v", err)
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
		return
	}

	// Push task to queue
	if err := rdb.LPush(ctx, "queue:tasks", taskData).Err(); err != nil {
		log.Printf("Failed to push to queue: %v", err)
		http.Error(w, "Failed to queue task", http.StatusInternalServerError)
		return
	}

	fmt.Printf("Task queued successfully: %s\n", string(taskData))

	// 7. Return pending status with UUID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(TaskResponse{
		Status: "pending",
		UUID:   taskUUID,
	})
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
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
	statusKey := "status:" + taskUUID
	status, err := rdb.Get(ctx, statusKey).Result()
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// If task is done, also return the cached result
	var response TaskResponse
	if status == "done" {
		// Find the URL for this task by searching through url_task mappings
		urlTaskKeys, err := rdb.Keys(ctx, "url_task:*").Result()
		if err != nil {
			log.Printf("Failed to get URL task keys: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		var taskURL string
		for _, key := range urlTaskKeys {
			urlTaskData, err := rdb.Get(ctx, key).Result()
			if err != nil {
				continue
			}
			var urlTask URLTaskMapping
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
			cacheKey := "cache:" + taskURL
			if cachedResult, err := rdb.Get(ctx, cacheKey).Result(); err == nil {
				response = TaskResponse{
					Status: status,
					UUID:   taskUUID,
					Result: cachedResult,
				}
			} else {
				response = TaskResponse{
					Status: status,
					UUID:   taskUUID,
				}
			}
		} else {
			response = TaskResponse{
				Status: status,
				UUID:   taskUUID,
			}
		}
	} else {
		response = TaskResponse{
			Status: status,
			UUID:   taskUUID,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
