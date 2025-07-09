package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"bytes"
	"io/ioutil"
)

var ctx = context.Background()
var rdb *redis.Client
var hub *Hub

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
	EnableCompression: true,
}

// WebSocket hub to manage connections
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mutex      sync.RWMutex
}

// WebSocket client
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// WebSocket message types
type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type TaskUpdateMessage struct {
	UUID   string `json:"uuid"`
	URL    string `json:"url"`
	Status string `json:"status"`
	Result string `json:"result,omitempty"`
}

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

type TaskHistoryItem struct {
	URL      string `json:"url"`
	UUID     string `json:"uuid"`
	Status   string `json:"status"`
	Summary  string `json:"summary,omitempty"`
	Sentiment string `json:"sentiment,omitempty"`
}

type TaskHistoryResponse struct {
	Tasks []TaskHistoryItem `json:"tasks"`
}

// ArticleResultPayload matches the db-service model
// Used to send the correct shape to db-service
//
type ArticleResultPayload struct {
	UUID      string `json:"uuid"`
	URL       string `json:"url"`
	Summary   string `json:"summary"`
	Sentiment string `json:"sentiment"`
}

// CORS middleware
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler
		next(w, r)
	}
}

// Helper: Query db-service for article by URL
func getArticleFromDBService(url string) (string, error) {
	dbServiceURL := "http://db-service:5000/article?url=" + url
	resp, err := http.Get(dbServiceURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("not found")
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// Helper: Save article to db-service
func saveArticleToDBService(articleJSON string) error {
	fmt.Printf("\nCalled saveArticleToDBService\n")

	dbServiceURL := "http://db-service:5000/article"
	resp, err := http.Post(dbServiceURL, "application/json", bytes.NewBuffer([]byte(articleJSON)))
	if err != nil {
		fmt.Printf("Error saving article to db-service: %v\n", err)
		return err
	} else {
		fmt.Printf("response: %v\n", resp)
		fmt.Printf("Article saved to db-service: %v\n", resp.StatusCode)
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		fmt.Printf("Failed to save article: %v\n", resp.StatusCode)
		return fmt.Errorf("failed to save article")
	}
	return nil
}

func main() {
	rdb = redis.NewClient(&redis.Options{
		Addr: "redis:6379", // container name
	})

	// Initialize WebSocket hub
	hub = newHub()
	go hub.run()

	// Start the Redis pub/sub subscriber
	go startResultSubscriber()

	// Apply CORS middleware to all routes
	http.HandleFunc("/submit", corsMiddleware(handleSubmit))
	http.HandleFunc("/status/", corsMiddleware(handleStatus))
	http.HandleFunc("/tasks", corsMiddleware(handleTasks))
	http.HandleFunc("/ws", corsMiddleware(handleWebSocket))

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
		
		// Parse the structured result
		var result ProcessResult
		if err := json.Unmarshal([]byte(msg.Payload), &result); err != nil {
			log.Printf("Failed to parse result message: %v", err)
			continue
		}
		
		fmt.Printf("Task %s completed for URL: %s\n", result.UUID, result.URL)
		fmt.Printf("Result summary: %v\n", result.Result["summary"])
		
		// Broadcast the result to all WebSocket clients
		taskUpdate := TaskUpdateMessage{
			UUID:   result.UUID,
			URL:    result.URL,
			Status: "done",
		}
		fmt.Printf("Task update: %v\n", taskUpdate)
		
		// Convert result to JSON string for the message
		if resultStr, err := json.Marshal(result.Result); err == nil {
			taskUpdate.Result = string(resultStr)
		}
		
		wsMessage := WSMessage{
			Type:    "task_update",
			Payload: taskUpdate,
		}
		
		if messageBytes, err := json.Marshal(wsMessage); err == nil {
			hub.broadcast <- messageBytes
			fmt.Printf("Broadcasted task update to WebSocket clients: %s\n", string(messageBytes))
		} else {
			log.Printf("Failed to marshal WebSocket message: %v", err)
		}

		// 1. Save to db-service (send flat ArticleResultPayload)
		var summary, sentiment string
		if s, ok := result.Result["summary"].(string); ok {
			summary = s
		}
		if s, ok := result.Result["sentiment"].(string); ok {
			sentiment = s
		}
		articlePayload := ArticleResultPayload{
			UUID:      result.UUID,
			URL:       result.URL,
			Summary:   summary,
			Sentiment: sentiment,
		}
		resultJSON, err := json.Marshal(articlePayload)
		if err == nil {
			saveArticleToDBService(string(resultJSON))
		}
		// 2. Save to Redis with TTL (cache the original result as before)
		rdb.Set(ctx, "cache:"+result.URL, msg.Payload, 5*time.Minute)
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
		fmt.Printf("cachedResult: %v\n", cachedResult)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TaskResponse{
			Status: "done",
			Result: cachedResult,
		})
		return
	}

	// 1.5. Check db-service (Postgres) for the article
	dbResult, err := getArticleFromDBService(req.URL)
	if err == nil && dbResult != "" {
		// Cache in Redis for next time (set TTL)
		fmt.Printf("Cache miss for URL: %s | Retrieved from db-service | setting cache for 1 minute\n", req.URL)
		rdb.Set(ctx, cacheKey, dbResult, 5*time.Minute)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TaskResponse{
			Status: "done",
			Result: dbResult,
		})
		return
	} else {
		fmt.Printf("Error retrieving article from db-service: %v\n", err)
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

func handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	// Get all URL task mappings
	urlTaskKeys, err := rdb.Keys(ctx, "url_task:*").Result()
	if err != nil {
		log.Printf("Failed to get URL task keys: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var tasks []TaskHistoryItem

	for _, key := range urlTaskKeys {
		// Extract URL from key (remove "url_task:" prefix)
		url := key[9:] // "url_task:" is 9 characters

		// Get URL task mapping data
		urlTaskData, err := rdb.Get(ctx, key).Result()
		if err != nil {
			log.Printf("Failed to get URL task data for key %s: %v", key, err)
			continue
		}

		var urlTask URLTaskMapping
		if err := json.Unmarshal([]byte(urlTaskData), &urlTask); err != nil {
			log.Printf("Failed to unmarshal URL task data for key %s: %v", key, err)
			continue
		}

		// Create task history item
		taskItem := TaskHistoryItem{
			URL:    url,
			UUID:   urlTask.UUID,
			Status: urlTask.Status,
		}

		// If task is done, get results (from cache or DB)
		if urlTask.Status == "done" {
			cacheKey := "cache:" + url
			var resultData string
			var result map[string]interface{}

			// First, try to get from cache
			if cachedResult, err := rdb.Get(ctx, cacheKey).Result(); err == nil {
				resultData = cachedResult
				fmt.Printf("Cache hit for URL: %s\n", url)
			} else {
				// Cache miss - try to get from DB
				fmt.Printf("Cache miss for URL: %s, checking DB...\n", url)
				if dbResult, err := getArticleFromDBService(url); err == nil && dbResult != "" {
					// Transform DB result to cache format
					var dbArticle struct {
						UUID      string `json:"uuid"`
						URL       string `json:"url"`
						Summary   string `json:"summary"`
						Sentiment string `json:"sentiment"`
					}
					
					if err := json.Unmarshal([]byte(dbResult), &dbArticle); err == nil {
						// Create ProcessResult format to match cache structure
						processResult := ProcessResult{
							UUID: dbArticle.UUID,
							URL:  dbArticle.URL,
							Result: map[string]interface{}{
								"summary":   dbArticle.Summary,
								"sentiment": dbArticle.Sentiment,
							},
						}
						
						// Convert to JSON string for caching
						if processResultJSON, err := json.Marshal(processResult); err == nil {
							fmt.Printf("processResultJSON: %v\n", processResultJSON)
							resultData = string(processResultJSON)
							// Cache the transformed result for next time
							rdb.Set(ctx, cacheKey, resultData, 5*time.Minute)
							fmt.Printf("Retrieved from DB, transformed, and cached for URL: %s\n", url)
						}
					}
				} else {
					fmt.Printf("Not found in DB for URL: %s\n", url)
					// Continue without result data
				}
			}

			// Parse result data if we have it
			if resultData != "" {
				// Try to parse as ProcessResult first (cached format)
				var processResult ProcessResult
				if err := json.Unmarshal([]byte(resultData), &processResult); err == nil {
					// Successfully parsed as ProcessResult - extract from result field
					if summary, ok := processResult.Result["summary"].(string); ok {
						taskItem.Summary = summary
					}
					if sentiment, ok := processResult.Result["sentiment"].(string); ok {
						taskItem.Sentiment = sentiment
					}
					fmt.Printf("Parsed ProcessResult for URL %s: summary='%s', sentiment='%s'\n", url, taskItem.Summary, taskItem.Sentiment)
				} else {
					// Fallback: try to parse as flat structure (legacy or DB format)
					if err := json.Unmarshal([]byte(resultData), &result); err == nil {
						if summary, ok := result["summary"].(string); ok {
							taskItem.Summary = summary
						}
						if sentiment, ok := result["sentiment"].(string); ok {
							taskItem.Sentiment = sentiment
						}
						fmt.Printf("Parsed flat structure for URL %s: summary='%s', sentiment='%s'\n", url, taskItem.Summary, taskItem.Sentiment)
					} else {
						fmt.Printf("Failed to parse result data for URL %s: %v\n", url, err)
					}
				}
			}
		}

		tasks = append(tasks, taskItem)
	}

	// Sort tasks by most recent first (assuming newer tasks have later UUIDs)
	// In a production system, you might want to add timestamps to tasks

	response := TaskHistoryResponse{
		Tasks: tasks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// WebSocket hub methods
func newHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()
			fmt.Printf("WebSocket client connected. Total clients: %d\n", len(h.clients))
			
		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mutex.Unlock()
			fmt.Printf("WebSocket client disconnected. Total clients: %d\n", len(h.clients))
			
		case message := <-h.broadcast:
			h.mutex.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mutex.RUnlock()
		}
	}
}

// WebSocket client methods
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	
	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}
		
		// Echo the message back for now (can be extended for client commands)
		fmt.Printf("Received WebSocket message: %s\n", string(message))
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			
			if err := w.Close(); err != nil {
				return
			}
			
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// WebSocket handler
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	
	client := &Client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 256),
	}
	client.hub.register <- client
	
	// Send welcome message
	welcomeMessage := WSMessage{
		Type:    "connected",
		Payload: "WebSocket connection established",
	}
	if messageBytes, err := json.Marshal(welcomeMessage); err == nil {
		client.send <- messageBytes
	}
	
	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}
