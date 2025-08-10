package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/rescp17/lanFileSharer/pkg/crypto"
)

// mockAddICECandidate is a mock function for testing ICE candidate handling
func mockAddICECandidate(candidate webrtc.ICECandidateInit) error {
	return nil
}

// mockAddICECandidateWithError is a mock function that returns an error
func mockAddICECandidateWithError(candidate webrtc.ICECandidateInit) error {
	return fmt.Errorf("mock ICE candidate error")
}

// createTestSignedFiles creates a test SignedFileStructure for testing
func createTestSignedFiles(t *testing.T) *crypto.SignedFileStructure {
	// Create temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	signedFiles, err := crypto.CreateSignedFileStructure([]string{testFile})
	if err != nil {
		t.Fatalf("Failed to create signed file structure: %v", err)
	}

	return signedFiles
}

// createTestOffer creates a test WebRTC offer for testing
func createTestOffer() webrtc.SessionDescription {
	return webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  "test-offer-sdp",
	}
}

// createTestAnswer creates a test WebRTC answer for testing
func createTestAnswer() webrtc.SessionDescription {
	return webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  "test-answer-sdp",
	}
}

func TestNewAPISignaler(t *testing.T) {
	client := NewClient("test-service-id")
	receiverURL := "http://localhost:8080"
	
	signaler := NewAPISignaler(client, receiverURL, mockAddICECandidate)
	
	if signaler == nil {
		t.Fatal("NewAPISignaler returned nil")
	}
	
	if signaler.apiClient != client {
		t.Error("APISignaler should store the provided client")
	}
	
	if signaler.receiverURL != receiverURL {
		t.Error("APISignaler should store the provided receiver URL")
	}
	
	if signaler.addIceCandidateFunc == nil {
		t.Error("APISignaler should store the provided ICE candidate function")
	}
	
	if signaler.answerChan == nil {
		t.Error("APISignaler should initialize answer channel")
	}
	
	if signaler.errChan == nil {
		t.Error("APISignaler should initialize error channel")
	}
}

func TestAPISignaler_SendOffer_Success(t *testing.T) {
	// Create test server that responds with SSE stream
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ask" {
			t.Errorf("Expected path /ask, got %s", r.URL.Path)
		}
		
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}
		
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("Expected Accept text/event-stream, got %s", r.Header.Get("Accept"))
		}
		
		// Verify request body
		var payload AskPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}
		
		// Send SSE response with answer
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		
		fmt.Fprint(w, "event: answer\n")
		fmt.Fprint(w, "data: {\"answer\":{\"type\":\"answer\",\"sdp\":\"test-answer-sdp\"}}\n")
		fmt.Fprint(w, "\n")
	}))
	defer server.Close()
	
	client := NewClient("test-service-id")
	signaler := NewAPISignaler(client, server.URL, mockAddICECandidate)
	
	ctx := context.Background()
	offer := createTestOffer()
	signedFiles := createTestSignedFiles(t)
	
	err := signaler.SendOffer(ctx, offer, signedFiles)
	if err != nil {
		t.Errorf("SendOffer failed: %v", err)
	}
	
	// Give some time for the goroutine to process the response
	time.Sleep(100 * time.Millisecond)
}

func TestAPISignaler_SendOffer_InvalidURL(t *testing.T) {
	client := NewClient("test-service-id")
	signaler := NewAPISignaler(client, "://invalid-url", mockAddICECandidate)
	
	ctx := context.Background()
	offer := createTestOffer()
	signedFiles := createTestSignedFiles(t)
	
	err := signaler.SendOffer(ctx, offer, signedFiles)
	if err == nil {
		t.Error("SendOffer should fail with invalid URL")
	}
	
	if !strings.Contains(err.Error(), "failed to create ask url") {
		t.Errorf("Expected URL creation error, got: %v", err)
	}
}

func TestAPISignaler_WaitForAnswer_Success(t *testing.T) {
	// Create test server that responds with answer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		
		fmt.Fprint(w, "event: answer\n")
		fmt.Fprint(w, "data: {\"answer\":{\"type\":\"answer\",\"sdp\":\"test-answer-sdp\"}}\n")
		fmt.Fprint(w, "\n")
	}))
	defer server.Close()
	
	client := NewClient("test-service-id")
	signaler := NewAPISignaler(client, server.URL, mockAddICECandidate)
	
	ctx := context.Background()
	offer := createTestOffer()
	signedFiles := createTestSignedFiles(t)
	
	// Send offer to start the SSE stream
	err := signaler.SendOffer(ctx, offer, signedFiles)
	if err != nil {
		t.Fatalf("SendOffer failed: %v", err)
	}
	
	// Wait for answer
	answer, err := signaler.WaitForAnswer(ctx)
	if err != nil {
		t.Errorf("WaitForAnswer failed: %v", err)
	}
	
	if answer == nil {
		t.Error("Answer should not be nil")
	}
	
	if answer.Type != webrtc.SDPTypeAnswer {
		t.Errorf("Expected answer type, got %v", answer.Type)
	}
	
	if answer.SDP != "test-answer-sdp" {
		t.Errorf("Expected test-answer-sdp, got %s", answer.SDP)
	}
}

func TestAPISignaler_WaitForAnswer_Rejection(t *testing.T) {
	// Create test server that responds with rejection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		
		fmt.Fprint(w, "event: rejection\n")
		fmt.Fprint(w, "data: Transfer rejected\n")
		fmt.Fprint(w, "\n")
	}))
	defer server.Close()
	
	client := NewClient("test-service-id")
	signaler := NewAPISignaler(client, server.URL, mockAddICECandidate)
	
	ctx := context.Background()
	offer := createTestOffer()
	signedFiles := createTestSignedFiles(t)
	
	// Send offer to start the SSE stream
	err := signaler.SendOffer(ctx, offer, signedFiles)
	if err != nil {
		t.Fatalf("SendOffer failed: %v", err)
	}
	
	// Wait for answer (should get rejection error)
	answer, err := signaler.WaitForAnswer(ctx)
	if err == nil {
		t.Error("WaitForAnswer should fail with rejection")
	}
	
	if err != ErrTransferRejected {
		t.Errorf("Expected ErrTransferRejected, got: %v", err)
	}
	
	if answer != nil {
		t.Error("Answer should be nil on rejection")
	}
}

func TestAPISignaler_WaitForAnswer_Timeout(t *testing.T) {
	client := NewClient("test-service-id")
	signaler := NewAPISignaler(client, "http://localhost:9999", mockAddICECandidate)
	
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	// Wait for answer with timeout
	answer, err := signaler.WaitForAnswer(ctx)
	if err == nil {
		t.Error("WaitForAnswer should fail with timeout")
	}
	
	if !strings.Contains(err.Error(), "context cancelled") {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}
	
	if answer != nil {
		t.Error("Answer should be nil on timeout")
	}
}

func TestAPISignaler_HandleCandidateEvent_Success(t *testing.T) {
	candidateReceived := false
	mockAddICE := func(candidate webrtc.ICECandidateInit) error {
		candidateReceived = true
		if candidate.Candidate != "test-candidate" {
			t.Errorf("Expected test-candidate, got %s", candidate.Candidate)
		}
		return nil
	}
	
	client := NewClient("test-service-id")
	signaler := NewAPISignaler(client, "http://localhost:8080", mockAddICE)
	
	// Test candidate event handling
	candidateData := `{"candidate":{"candidate":"test-candidate","sdpMid":"0","sdpMLineIndex":0}}`
	signaler.handleCandidateEvent(candidateData)
	
	if !candidateReceived {
		t.Error("ICE candidate should have been processed")
	}
}

func TestAPISignaler_HandleCandidateEvent_InvalidJSON(t *testing.T) {
	client := NewClient("test-service-id")
	signaler := NewAPISignaler(client, "http://localhost:8080", mockAddICECandidate)
	
	// Test with invalid JSON - should not panic
	signaler.handleCandidateEvent("invalid-json")
	
	// If we reach here, the test passes (no panic occurred)
}

func TestAPISignaler_HandleCandidateEvent_AddCandidateError(t *testing.T) {
	client := NewClient("test-service-id")
	signaler := NewAPISignaler(client, "http://localhost:8080", mockAddICECandidateWithError)
	
	// Test candidate event handling with error - should not panic
	candidateData := `{"candidate":{"candidate":"test-candidate","sdpMid":"0","sdpMLineIndex":0}}`
	signaler.handleCandidateEvent(candidateData)
	
	// If we reach here, the test passes (no panic occurred)
}

func TestAPISignaler_SendICECandidate_Success(t *testing.T) {
	// Create test server that accepts ICE candidates
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/candidate" {
			t.Errorf("Expected path /candidate, got %s", r.URL.Path)
		}
		
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}
		
		// Verify request body
		var candidate webrtc.ICECandidateInit
		if err := json.NewDecoder(r.Body).Decode(&candidate); err != nil {
			t.Errorf("Failed to decode candidate: %v", err)
		}
		
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	client := NewClient("test-service-id")
	signaler := NewAPISignaler(client, server.URL, mockAddICECandidate)
	
	ctx := context.Background()
	candidate := webrtc.ICECandidateInit{
		Candidate: "test-candidate",
		SDPMid:    stringPtr("0"),
		SDPMLineIndex: uint16Ptr(0),
	}
	
	err := signaler.SendICECandidate(ctx, candidate)
	if err != nil {
		t.Errorf("SendICECandidate failed: %v", err)
	}
}

func TestAPISignaler_SendICECandidate_Timeout(t *testing.T) {
	client := NewClient("test-service-id")
	signaler := NewAPISignaler(client, "http://localhost:9999", mockAddICECandidate)
	
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	candidate := webrtc.ICECandidateInit{
		Candidate: "test-candidate",
	}
	
	err := signaler.SendICECandidate(ctx, candidate)
	if err == nil {
		t.Error("SendICECandidate should fail with timeout")
	}
	
	if !strings.Contains(err.Error(), "context cancelled") {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}
}

// Helper functions for creating pointers
func stringPtr(s string) *string {
	return &s
}

func uint16Ptr(u uint16) *uint16 {
	return &u
}