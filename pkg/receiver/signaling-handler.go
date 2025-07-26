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

type answerResult struct {
	answer *webrtc.SessionDescription
	err    error
}

type SignalingHandler struct {
	mu         sync.Mutex
	webrtcConn *webrtcPkg.ReceiverConn
	answerChan chan answerResult
}

func NewSignalingHandler() *SignalingHandler {
	return &SignalingHandler{
		answerChan: make(chan answerResult, 1),
	}
}

func (h *SignalingHandler) RegisterHandler(mux *http.ServeMux) {
	mux.Handle("POST /offer", http.HandlerFunc(h.OfferHandler))
	mux.Handle("POST /answer-stream", http.HandlerFunc(h.AnswerStreamHandler))
	mux.Handle("POST /candidate", http.HandlerFunc(h.ICECandidateHanlder))
}

func (h *SignalingHandler) OfferHandler(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	if h.webrtcConn == nil {
		h.mu.Unlock()
		slog.Info("[offerHandler]: webrtc connection is null")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	conn := h.webrtcConn
	h.mu.Unlock()

	var offer webrtc.SessionDescription
	if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
		slog.Info("[offerHandler]: failed to decode offer")
		http.Error(w, "Bad Request: could not decode offer", http.StatusBadRequest)
		return
	}

	go func() {
		answer, err := conn.HandleOfferAndCreateAnswer(offer)
		answerResult := answerResult {
			err: err,
			answer: answer,
		}
		
		select {
		case h.answerChan <- answerResult:
			slog.Info("sent answer successful")
		default:
			slog.Warn("answer channel was full, dropping answer")
		}
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
	case answerResult := <-h.answerChan:
		if answerResult.answer == nil {
			slog.Error("SSE answer channel closed, stop connection.")
			http.Error(w, "Failed to create WebRTC answer", http.StatusInternalServerError)
			return
		}
		answerJSON, err := json.Marshal(answerResult.answer)
		if err != nil {
			slog.Error("failed to encode answer json", "error", err)
			http.Error(w, "Failed to encode answer", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "event: answer\ndata: %s\n\n", answerJSON)
		flusher.Flush()
		slog.Info("SSE answer had sent")
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
		slog.Info("failed to add ICE candidate %v", err)
		http.Error(w, "Failed to add ICE candidate", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
