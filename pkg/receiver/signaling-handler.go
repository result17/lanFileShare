package receiver

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"sync"

	"github.com/pion/webrtc/v4"
	webrtcPkg "github.com/rescp17/lanFileSharer/pkg/webrtc"
)

type SignalingHandler struct {
	mu         sync.Mutex
	webrtcConn *webrtcPkg.ReceiverConn
	answerChan chan *webrtc.SessionDescription
}

func NewSignalingHandler() *SignalingHandler {
	return &SignalingHandler{
		answerChan: make(chan *webrtc.SessionDescription, 1),
	}
}

func (h *SignalingHandler) RegisterHandler(mux *http.ServeMux) {}

func (h *SignalingHandler) OfferHandler(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	if h.webrtcConn == nil {
		h.mu.Unlock()
		log.Printf("[offerHandler]: webrtc connection is null")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	conn := h.webrtcConn
	h.mu.Unlock()

	var offer webrtc.SessionDescription
	if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
		log.Printf("[offerHandler]: failed to decode offer")
		return
	}

	go func() {
		answer, err := conn.HandleOfferAndCreateAnswer(offer)
		if err != nil {
			err := fmt.Errorf("failed to create answer: %w", err)
			log.Printf("[OfferHandler]: %v", err)
			close(h.answerChan)
			return
		}
		h.answerChan <- answer
	}()

	w.WriteHeader(http.StatusOK)
}

func (h *SignalingHandler) AnswerStreamHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	slog.Info("SSE Connected. Waiting for answer.")

	select {
	case answer := <-h.answerChan:
		if answer == nil {
			log.Printf("SSE answer channel closed, stop connection.")
			return
		}
		answerJSON, err := json.Marshal(answer)
		if err != nil {
			log.Printf("failed to encode answer json %v", err)
			return
		}
		fmt.Fprintf(w, "event: answer\ndata: %s\n\n", answerJSON)
		flusher.Flush()
		log.Printf("SSE answer had sent")
	case <-r.Context().Done():
		log.Print("SSE client connection closed")
		return
	}
}

func (h *SignalingHandler) ICECandidateHanlder(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	if h.webrtcConn == nil {
		h.mu.Unlock()
		http.Error(w, "WebRTC connection not ready", http.StatusServiceUnavailable)
		return
	}
	conn := h.webrtcConn
	h.mu.Unlock()
	var candidate webrtc.ICECandidateInit
	if err := json.NewDecoder(r.Body).Decode(&candidate); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if err := conn.Peer().AddICECandidate(candidate); err != nil {
		log.Printf("failed to add ICE candidate %v", err)
	}
	w.WriteHeader(http.StatusOK)
}
