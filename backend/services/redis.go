package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"backend/models"
	"backend/websocket"
)

var ctx = context.Background()
var rdb *redis.Client

// InitRedis initializes the Redis client
func InitRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr: "redis:6379", // container name
	})
}

// GetRedisClient returns the Redis client
func GetRedisClient() *redis.Client {
	return rdb
}

// StartResultSubscriber starts the Redis pub/sub subscriber for process results
func StartResultSubscriber(hub *websocket.Hub) {
	fmt.Println("Starting Redis pub/sub subscriber for process:results...")
	
	pubsub := rdb.Subscribe(ctx, "process:results")
	defer pubsub.Close()
	
	ch := pubsub.Channel()
	
	for msg := range ch {
		fmt.Printf("Received result from LLM server: %s\n", msg.Payload)
		
		// Parse the structured result
		var result models.ProcessResult
		if err := json.Unmarshal([]byte(msg.Payload), &result); err != nil {
			log.Printf("Failed to parse result message: %v", err)
			continue
		}
		
		fmt.Printf("Task %s completed for URL: %s\n", result.UUID, result.URL)
		fmt.Printf("Result summary: %v\n", result.Result["summary"])
		
		// Broadcast the result to all WebSocket clients
		taskUpdate := models.TaskUpdateMessage{
			UUID:   result.UUID,
			URL:    result.URL,
			Status: "done",
		}
		fmt.Printf("Task update: %v\n", taskUpdate)
		
		// Convert result to JSON string for the message
		if resultStr, err := json.Marshal(result.Result); err == nil {
			taskUpdate.Result = string(resultStr)
		}
		
		wsMessage := models.WSMessage{
			Type:    "task_update",
			Payload: taskUpdate,
		}
		
		if messageBytes, err := json.Marshal(wsMessage); err == nil {
			hub.GetBroadcastChannel() <- messageBytes
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
		
		fmt.Printf("UUID from result: %s\n", result.UUID)
		
		articlePayload := models.ArticleResultPayload{
			UUID:         result.UUID,
			URL:          result.URL,
			Summary:      summary,
			Sentiment:    sentiment,
			Conversation: []models.ConversationEntry{}, // Initialize empty conversation
		}
		resultJSON, err := json.Marshal(articlePayload)
		if err == nil {
			fmt.Printf("Sending to db-service: %s\n", string(resultJSON))
			SaveArticleToDBService(string(resultJSON))
		} else {
			fmt.Printf("Failed to marshal article payload: %v\n", err)
		}
		// 2. Save to Redis with TTL (cache the original result as before)
		rdb.Set(ctx, "cache:"+result.URL, msg.Payload, 1*time.Minute)
	}
}

// CheckCache checks if URL is cached in Redis
func CheckCache(url string) (string, error) {
	cacheKey := "cache:" + url
	return rdb.Get(ctx, cacheKey).Result()
}

// SetCache sets a URL result in Redis cache with TTL
func SetCache(url string, result string, ttl time.Duration) error {
	cacheKey := "cache:" + url
	return rdb.Set(ctx, cacheKey, result, ttl).Err()
}

// GetURLTaskMapping gets the task mapping for a URL
func GetURLTaskMapping(url string) (string, error) {
	urlTaskKey := "url_task:" + url
	return rdb.Get(ctx, urlTaskKey).Result()
}

// SetURLTaskMapping sets the task mapping for a URL
func SetURLTaskMapping(url string, mappingData []byte) error {
	urlTaskKey := "url_task:" + url
	return rdb.Set(ctx, urlTaskKey, mappingData, 0).Err()
}

// SetTaskStatus sets the status for a task UUID
func SetTaskStatus(taskUUID string, status string) error {
	statusKey := "status:" + taskUUID
	return rdb.Set(ctx, statusKey, status, 0).Err()
}

// GetTaskStatus gets the status for a task UUID
func GetTaskStatus(taskUUID string) (string, error) {
	statusKey := "status:" + taskUUID
	return rdb.Get(ctx, statusKey).Result()
}

// PushTaskToQueue pushes a task to the Redis queue
func PushTaskToQueue(taskData []byte) error {
	return rdb.LPush(ctx, "queue:tasks", taskData).Err()
}

// GetAllURLTaskKeys gets all URL task mapping keys
func GetAllURLTaskKeys() ([]string, error) {
	return rdb.Keys(ctx, "url_task:*").Result()
}

// GetContext returns the Redis context
func GetContext() context.Context {
	return ctx
}

// UpdateConversationInCache updates the conversation for an article in the cache
func UpdateConversationInCache(uuid string, conversation []models.ConversationEntry) error {
	// First, we need to find the URL for this UUID by checking all cached items
	// This is a bit inefficient but necessary since we cache by URL, not UUID
	cacheKeys, err := rdb.Keys(ctx, "cache:*").Result()
	if err != nil {
		return fmt.Errorf("failed to get cache keys: %v", err)
	}
	
	for _, cacheKey := range cacheKeys {
		cachedData, err := rdb.Get(ctx, cacheKey).Result()
		if err != nil {
			continue
		}
		
		// Try to parse the cached data to check if it matches our UUID
		var result models.ProcessResult
		if err := json.Unmarshal([]byte(cachedData), &result); err != nil {
			continue
		}
		
		if result.UUID == uuid {
			// Found the matching cached item, update the conversation
			if result.Result == nil {
				result.Result = make(map[string]interface{})
			}
			result.Result["conversation"] = conversation
			
			// Marshal the updated result
			updatedData, err := json.Marshal(result)
			if err != nil {
				return fmt.Errorf("failed to marshal updated result: %v", err)
			}
			
			// Update the cache with the new data (preserve TTL)
			ttl, err := rdb.TTL(ctx, cacheKey).Result()
			if err != nil {
				ttl = 1 * time.Minute // Default TTL if we can't get it
			}
			
			err = rdb.Set(ctx, cacheKey, updatedData, ttl).Err()
			if err != nil {
				return fmt.Errorf("failed to update cache: %v", err)
			}
			
			fmt.Printf("Updated conversation in cache for UUID: %s, URL: %s\n", uuid, result.URL)
			return nil
		}
	}
	
	return fmt.Errorf("no cached article found for UUID: %s", uuid)
}

// GetArticleByUUIDFromCache retrieves an article from cache by UUID
func GetArticleByUUIDFromCache(uuid string) (*models.ProcessResult, error) {
	cacheKeys, err := rdb.Keys(ctx, "cache:*").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get cache keys: %v", err)
	}
	
	for _, cacheKey := range cacheKeys {
		cachedData, err := rdb.Get(ctx, cacheKey).Result()
		if err != nil {
			continue
		}
		
		var result models.ProcessResult
		if err := json.Unmarshal([]byte(cachedData), &result); err != nil {
			continue
		}
		
		if result.UUID == uuid {
			return &result, nil
		}
	}
	
	return nil, fmt.Errorf("no cached article found for UUID: %s", uuid)
} 