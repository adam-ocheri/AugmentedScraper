package websocket

import (
	"net/http"
	"github.com/gorilla/websocket"
)

// WebSocket upgrader
var Upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
	EnableCompression: true,
} 