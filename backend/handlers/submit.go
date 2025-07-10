package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"backend/models"
	"backend/services"
)

// validateURL checks if the provided string is a valid HTTPS URL
func validateURL(urlStr string) error {
	// Check if URL is empty
	if strings.TrimSpace(urlStr) == "" {
		return fmt.Errorf("URL is required")
	}

	// Parse the URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("Only valid https links are allowed! please make sure you are using a public link, such as 'https://www.example.com'")
	}

	// Check if scheme is HTTPS
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("Only valid https links are allowed! please make sure you are using a public link, such as 'https://www.example.com'")
	}

	// Check if host is not empty
	if parsedURL.Host == "" {
		return fmt.Errorf("Only valid https links are allowed! please make sure you are using a public link, such as 'https://www.example.com'")
	}

	return nil
}

// checkURLAccessibility makes an HTTP HEAD request to check if the URL is accessible
func checkURLAccessibility(urlStr string) error {
	client := &http.Client{
		Timeout: 10 * time.Second, // 10 second timeout
	}

	req, err := http.NewRequest("HEAD", urlStr, nil)
	if err != nil {
		return fmt.Errorf("The provided link is invalid or cannot be loaded! please try again later or try a different link")
	}

	// Set a user agent to avoid being blocked by some servers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("The provided link is invalid or cannot be loaded! please try again later or try a different link")
	}
	defer resp.Body.Close()

	// Check if status code is 200 (OK)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("The provided link is invalid or cannot be loaded! please try again later or try a different link")
	}

	return nil
}

// HandleSubmit handles article submission requests
func HandleSubmit(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Got request")

	if r.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	var req models.ArticleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Validate URL format (HTTPS only)
	if err := validateURL(req.URL); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check URL accessibility (returns 200)
	if err := checkURLAccessibility(req.URL); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 1. Check if the URL has already been processed and cached
	cachedResult, err := services.CheckCache(req.URL)
	if err == nil {
		// URL is cached, return the cached result
		fmt.Printf("Cache hit for URL: %s\n", req.URL)
		fmt.Printf("cachedResult: %v\n", cachedResult)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.TaskResponse{
			Status: "done",
			Result: cachedResult,
		})
		return
	}

	// 1.5. URL is not cached - Check db-service (Postgres) for the article
	dbResult, err := services.GetArticleFromDBService(req.URL)
	if err == nil && dbResult != "" {
		// Cache in Redis for next time (set TTL)
		fmt.Printf("Cache miss for URL: %s | Retrieved from db-service | setting cache for 1 minute\n", req.URL)
		services.SetCache(req.URL, dbResult, 1*time.Minute)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.TaskResponse{
			Status: "done",
			Result: dbResult,
		})
		return
	} else {
		fmt.Printf("Error retrieving article from db-service: %v\n", err)
	}

	// 2. Check if there's already a task in progress for this URL
	urlTaskData, err := services.GetURLTaskMapping(req.URL)
	if err == nil {
		// URL is already being processed, return existing task info
		var urlTask models.URLTaskMapping
		if err := json.Unmarshal([]byte(urlTaskData), &urlTask); err != nil {
			log.Printf("Failed to unmarshal URL task mapping: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		fmt.Printf("Task already in progress for URL: %s, UUID: %s, Status: %s\n", req.URL, urlTask.UUID, urlTask.Status)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models.TaskResponse{
			Status: urlTask.Status,
			UUID:   urlTask.UUID,
		})
		return
	}

	// 3. URL not cached and no task in progress, create new task
	taskUUID := uuid.New().String()
	fmt.Printf("Creating new task for URL: %s, UUID: %s\n", req.URL, taskUUID)

	// 4. Store task status as "pending"
	if err := services.SetTaskStatus(taskUUID, "pending"); err != nil {
		log.Printf("Failed to set task status: %v", err)
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
		return
	}

	// 5. Immediately cache the URL-to-task mapping to prevent duplicates
	urlTaskMapping := models.URLTaskMapping{
		UUID:   taskUUID,
		Status: "pending",
	}
	urlTaskMappingData, err := json.Marshal(urlTaskMapping)
	if err != nil {
		log.Printf("Failed to marshal URL task mapping: %v", err)
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
		return
	}

	if err := services.SetURLTaskMapping(req.URL, urlTaskMappingData); err != nil {
		log.Printf("Failed to set URL task mapping: %v", err)
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
		return
	}

	// 6. Create task payload and add to queue
	taskPayload := models.TaskPayload{
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
	if err := services.PushTaskToQueue(taskData); err != nil {
		log.Printf("Failed to push to queue: %v", err)
		http.Error(w, "Failed to queue task", http.StatusInternalServerError)
		return
	}

	fmt.Printf("Task queued successfully: %s\n", string(taskData))

	// 7. Return pending status with UUID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.TaskResponse{
		Status: "pending",
		UUID:   taskUUID,
	})
} 