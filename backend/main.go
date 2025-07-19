package main

import (
	"fmt"
	"log"
	"net/http"

	"backend/handlers"
	"backend/middleware"
	"backend/services"
	"backend/websocket"
)

func main() {
	// Initialize Redis
	services.InitRedis()

	// Initialize WebSocket hub
	hub := websocket.NewHub()
	go hub.Run()

	// Start the Redis pub/sub subscriber
	go services.StartResultSubscriber(hub)

	// Apply CORS middleware to all routes
	http.HandleFunc("/submit", middleware.CORS(handlers.HandleSubmit))
	http.HandleFunc("/status/", middleware.CORS(handlers.HandleStatus))
	http.HandleFunc("/tasks", middleware.CORS(handlers.HandleTasks))
	http.HandleFunc("/conversation/update", middleware.CORS(handlers.HandleConversationUpdate))
	http.HandleFunc("/inform-model-loaded", middleware.CORS(func(w http.ResponseWriter, r *http.Request) {
		handlers.HandleModelLoaded(w, r, hub)
	}))
	http.HandleFunc("/is-model-loaded", middleware.CORS(handlers.HandleIsModelLoaded))
	http.HandleFunc("/chat", middleware.CORS(func(w http.ResponseWriter, r *http.Request) {
		handlers.HandleChat(w, r, hub)
	}))
	http.HandleFunc("/ws", middleware.CORS(func(w http.ResponseWriter, r *http.Request) {
		handlers.HandleWebSocket(w, r, hub)
	}))

	fmt.Println("Go API running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
} 