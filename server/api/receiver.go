package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var AskRequests sync.Map

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func AskHandler(w http.ResponseWriter, r *http.Request) {
	if websocket.IsWebSocketUpgrade(r) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("WebSocket upgrade error:", err)
			http.Error(w, "WebSocket upgrade failed", http.StatusInternalServerError)
			return
		}
		defer conn.Close()
		// exchange sdp

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Println("WebSocket read error:", err)
				break
			}
			log.Printf("Received WS message: %s", msg)
			// reply answer
		}
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AskPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	log.Printf("Ask received: %+v", req)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Accepted"))

	// TODO show file information

}
