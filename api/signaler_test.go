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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	require.Nil(t, err, "Failed to create test file")

	signedFiles, err := crypto.CreateSignedFileStructure([]string{testFile})

	require.Nil(t, err, "Failed to create signed file structure")

	return signedFiles
}

// createTestOffer creates a test WebRTC offer for testing
func createTestOffer() webrtc.SessionDescription {
	return webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  "test-offer-sdp",
	}
}

func TestNewAPISignaler(t *testing.T) {
	client := NewClient("test-service-id")
	receiverURL := "http://localhost:8080"

	signaler := NewAPISignaler(client, receiverURL, mockAddICECandidate)

	require.NotNil(t, signaler, "NewAPISignaler returned nil")

	assert.Equal(t, client, signaler.apiClient, "APISignaler should store the provided client")
	assert.Equal(t, receiverURL, signaler.receiverURL, "APISignaler should store the provided receiver URL")
	assert.NotNil(t, signaler.addIceCandidateFunc, "APISignaler should store the provided ICE candidate function")
	assert.NotNil(t, signaler.answerChan, "APISignaler should initialize answer channel")
	assert.NotNil(t, signaler.errChan, "APISignaler should initialize error channel")
}

func TestAPISignaler_SendOffer_Success(t *testing.T) {
	// Create test server that responds with SSE stream
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/ask", r.URL.Path, "Expected path /ask")
		assert.Equal(t, "POST", r.Method, "Expected POST method")
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"), "Expected Content-Type application/json")
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"), "Expected Accept text/event-stream")

		// Verify request body
		var payload AskPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		assert.NoError(t, err, "Failed to decode request body")

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
	require.NoError(t, err, "SendOffer failed")
}

func TestAPISignaler_SendOffer_InvalidURL(t *testing.T) {
	client := NewClient("test-service-id")
	signaler := NewAPISignaler(client, "://invalid-url", mockAddICECandidate)

	ctx := context.Background()
	offer := createTestOffer()
	signedFiles := createTestSignedFiles(t)

	err := signaler.SendOffer(ctx, offer, signedFiles)
	assert.Error(t, err, "SendOffer should fail with invalid URL")
	assert.Contains(t, err.Error(), "failed to create ask url", "Expected URL creation error")
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
	require.NoError(t, err, "SendOffer failed")

	// Wait for answer
	answer, err := signaler.WaitForAnswer(ctx)
	assert.NoError(t, err, "WaitForAnswer failed")
	assert.NotNil(t, answer, "Answer should not be nil")
	assert.Equal(t, webrtc.SDPTypeAnswer, answer.Type, "Expected answer type")
	assert.Equal(t, "test-answer-sdp", answer.SDP, "Expected test-answer-sdp")
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
	require.NoError(t, err, "SendOffer failed")

	// Wait for answer (should get rejection error)
	answer, err := signaler.WaitForAnswer(ctx)
	assert.Error(t, err, "WaitForAnswer should fail with rejection")
	assert.Equal(t, ErrTransferRejected, err, "Expected ErrTransferRejected")
	assert.Nil(t, answer, "Answer should be nil on rejection")
}

func TestAPISignaler_WaitForAnswer_Timeout(t *testing.T) {
	client := NewClient("test-service-id")
	signaler := NewAPISignaler(client, "http://localhost:9999", mockAddICECandidate)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Wait for answer with timeout
	answer, err := signaler.WaitForAnswer(ctx)
	assert.Error(t, err, "WaitForAnswer should fail with timeout")
	assert.Contains(t, err.Error(), "context canceled", "Expected context cancellation error")
	assert.Nil(t, answer, "Answer should be nil on timeout")
}

func TestAPISignaler_HandleCandidateEvent_Success(t *testing.T) {
	candidateReceived := false
	mockAddICE := func(candidate webrtc.ICECandidateInit) error {
		candidateReceived = true
		assert.Equal(t, "test-candidate", candidate.Candidate, "Expected test-candidate")
		return nil
	}

	client := NewClient("test-service-id")
	signaler := NewAPISignaler(client, "http://localhost:8080", mockAddICE)

	// Test candidate event handling
	candidateData := `{"candidate":{"candidate":"test-candidate","sdpMid":"0","sdpMLineIndex":0}}`
	signaler.handleCandidateEvent(candidateData)

	assert.True(t, candidateReceived, "ICE candidate should have been processed")
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
		assert.Equal(t, "/candidate", r.URL.Path, "Expected path /candidate")
		assert.Equal(t, "POST", r.Method, "Expected POST method")

		// Verify request body
		var candidate webrtc.ICECandidateInit
		err := json.NewDecoder(r.Body).Decode(&candidate)
		assert.NoError(t, err, "Failed to decode candidate")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient("test-service-id")
	signaler := NewAPISignaler(client, server.URL, mockAddICECandidate)

	ctx := context.Background()
	candidate := webrtc.ICECandidateInit{
		Candidate:     "test-candidate",
		SDPMid:        stringPtr("0"),
		SDPMLineIndex: uint16Ptr(0),
	}

	err := signaler.SendICECandidate(ctx, candidate)
	assert.NoError(t, err, "SendICECandidate failed")
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
	assert.Error(t, err, "SendICECandidate should fail with timeout")

	// The error could be either context cancellation or connection refused
	// Both are valid failure scenarios for this test
	assert.True(t, strings.Contains(err.Error(), "context canceled") || strings.Contains(err.Error(), "refused"),
		"Expected context cancellation or connection refused error, got: %v", err)
}

// Helper functions for creating pointers
func stringPtr(s string) *string {
	return &s
}

func uint16Ptr(u uint16) *uint16 {
	return &u
}
