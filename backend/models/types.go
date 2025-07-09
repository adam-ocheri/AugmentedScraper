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
	UUID      string `json:"uuid"`
	URL       string `json:"url"`
	Summary   string `json:"summary"`
	Sentiment string `json:"sentiment"`
} 