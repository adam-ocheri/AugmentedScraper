package models

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
type ArticleResultPayload struct {
	UUID         string                `json:"Uuid"`
	URL          string                `json:"Url"`
	Summary      string                `json:"Summary"`
	Sentiment    string                `json:"Sentiment"`
	Conversation []ConversationEntry   `json:"Conversation"`
}

type ConversationEntry struct {
	Role    string `json:"Role"`
	Content string `json:"Content"`
}

// ConversationUpdateRequest represents a request to update conversation
type ConversationUpdateRequest struct {
	UUID         string              `json:"Uuid"`
	Conversation []ConversationEntry `json:"Conversation"`
}

// ConversationUpdateResponse represents the response from conversation update
type ConversationUpdateResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ChatRequest represents a chat request from the frontend
type ChatRequest struct {
	UUID    string `json:"uuid"`
	Message string `json:"message"`
}

// ChatResponse represents the response from a chat request
type ChatResponse struct {
	UUID     string `json:"uuid"`
	Response string `json:"response"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
} 