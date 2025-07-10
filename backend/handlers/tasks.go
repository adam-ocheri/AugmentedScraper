package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"backend/models"
	"backend/services"
)

// trimUserMessage removes the context part from user messages to show only the original question
func trimUserMessage(content string) string {
	// Look for the pattern " to answer the following question: " and trim everything before it
	if strings.Contains(content, " to answer the following question: ") {
		parts := strings.Split(content, " to answer the following question: ")
		if len(parts) > 1 {
			return parts[1]
		}
	}
	return content
}

// trimConversationMessages processes conversation entries to trim user messages
func trimConversationMessages(conversation []models.ConversationEntry) []models.ConversationEntry {
	trimmed := make([]models.ConversationEntry, len(conversation))
	for i, entry := range conversation {
		trimmed[i] = entry
		if entry.Role == "user" {
			trimmed[i].Content = trimUserMessage(entry.Content)
		}
	}
	return trimmed
}

// HandleTasks handles task history requests
func HandleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	// Get all URL task mappings
	urlTaskKeys, err := services.GetAllURLTaskKeys()
	if err != nil {
		log.Printf("Failed to get URL task keys: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var tasks []models.TaskHistoryItem

	for _, key := range urlTaskKeys {
		// Extract URL from key (remove "url_task:" prefix)
		url := key[9:] // "url_task:" is 9 characters

		// Get URL task mapping data
		urlTaskData, err := services.GetRedisClient().Get(services.GetContext(), key).Result()
		if err != nil {
			log.Printf("Failed to get URL task data for key %s: %v", key, err)
			continue
		}

		var urlTask models.URLTaskMapping
		if err := json.Unmarshal([]byte(urlTaskData), &urlTask); err != nil {
			log.Printf("Failed to unmarshal URL task data for key %s: %v", key, err)
			continue
		}

		// Create task history item
		taskItem := models.TaskHistoryItem{
			URL:    url,
			UUID:   urlTask.UUID,
			Status: urlTask.Status,
		}

		// If task is done, get results (from cache or DB)
		if urlTask.Status == "done" {
			var resultData string
			var result map[string]interface{}

			// First, try to get from cache
			if cachedResult, err := services.CheckCache(url); err == nil {
				resultData = cachedResult
				fmt.Printf("Cache hit for URL: %s\n", url)
			} else {
				// Cache miss - try to get from DB
				fmt.Printf("Cache miss for URL: %s, checking DB...\n", url)
				if dbResult, err := services.GetArticleFromDBService(url); err == nil && dbResult != "" {
					// Transform DB result to cache format
					var dbArticle struct {
						UUID         string                `json:"uuid"`
						URL          string                `json:"url"`
						Summary      string                `json:"summary"`
						Sentiment    string                `json:"sentiment"`
						Conversation []models.ConversationEntry `json:"conversation"`
					}
					
					if err := json.Unmarshal([]byte(dbResult), &dbArticle); err == nil {
						// Create ProcessResult format to match cache structure
						processResult := models.ProcessResult{
							UUID: dbArticle.UUID,
							URL:  dbArticle.URL,
							Result: map[string]interface{}{
								"summary":      dbArticle.Summary,
								"sentiment":    dbArticle.Sentiment,
								"conversation": dbArticle.Conversation,
							},
						}
						
						// Convert to JSON string for caching
						if processResultJSON, err := json.Marshal(processResult); err == nil {
							fmt.Printf("processResultJSON: %v\n", processResultJSON)
							resultData = string(processResultJSON)
							// Cache the transformed result for next time
							services.SetCache(url, resultData, 1*time.Minute)
							fmt.Printf("Retrieved from DB, transformed, and cached for URL: %s\n", url)
						}
						
						// Also set the conversation data directly on the task item
						taskItem.Conversation = trimConversationMessages(dbArticle.Conversation)
						fmt.Printf("Retrieved conversation data for URL %s: %d entries\n", url, len(dbArticle.Conversation))
					}
				} else {
					fmt.Printf("Not found in DB for URL: %s\n", url)
					// Continue without result data
				}
			}

			// Parse result data if we have it
			if resultData != "" {
				// Try to parse as ProcessResult first (cached format)
				var processResult models.ProcessResult
				if err := json.Unmarshal([]byte(resultData), &processResult); err == nil {
					// Successfully parsed as ProcessResult - extract from result field
					if summary, ok := processResult.Result["summary"].(string); ok {
						taskItem.Summary = summary
					}
					if sentiment, ok := processResult.Result["sentiment"].(string); ok {
						taskItem.Sentiment = sentiment
					}
					// Extract conversation data from cache
					if conversationData, ok := processResult.Result["conversation"]; ok {
						if conversationBytes, err := json.Marshal(conversationData); err == nil {
							var conversation []models.ConversationEntry
							if err := json.Unmarshal(conversationBytes, &conversation); err == nil {
								taskItem.Conversation = trimConversationMessages(conversation)
								fmt.Printf("Retrieved conversation data from cache for URL %s: %d entries\n", url, len(conversation))
							}
						}
					}
					fmt.Printf("Parsed ProcessResult for URL %s: summary='%s', sentiment='%s', conversation=%d entries\n", url, taskItem.Summary, taskItem.Sentiment, len(taskItem.Conversation))
				} else {
					// Fallback: try to parse as flat structure (legacy or DB format)
					if err := json.Unmarshal([]byte(resultData), &result); err == nil {
						if summary, ok := result["summary"].(string); ok {
							taskItem.Summary = summary
						}
						if sentiment, ok := result["sentiment"].(string); ok {
							taskItem.Sentiment = sentiment
						}
						// Extract conversation data from flat structure
						if conversationData, ok := result["conversation"]; ok {
							if conversationBytes, err := json.Marshal(conversationData); err == nil {
								var conversation []models.ConversationEntry
								if err := json.Unmarshal(conversationBytes, &conversation); err == nil {
									taskItem.Conversation = trimConversationMessages(conversation)
									fmt.Printf("Retrieved conversation data from flat structure for URL %s: %d entries\n", url, len(conversation))
								}
							}
						}
						fmt.Printf("Parsed flat structure for URL %s: summary='%s', sentiment='%s', conversation=%d entries\n", url, taskItem.Summary, taskItem.Sentiment, len(taskItem.Conversation))
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

	response := models.TaskHistoryResponse{
		Tasks: tasks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
} 