package app

import (
	"sync"
	"errors"
	"log"
	"github.com/pion/webrtc/v4"
)

// Decision is the type for user's decision.
type Decision bool

const (
	Accepted Decision = true
	Rejected Decision = false
)

// RequestState holds all the necessary information for a single file transfer request.
type RequestState struct {
	Offer         webrtc.SessionDescription
	DecisionChan  chan Decision
	AnswerChan    chan webrtc.SessionDescription
	CandidateChan chan webrtc.ICECandidateInit
	TransferDone  chan struct{}
}

// StateManager manages the lifecycle of a file transfer request state in a concurrent-safe manner.
type StateManager struct {
	mu    sync.Mutex
	state *RequestState // Holds the state for the *single* active request
}

// NewStateManager creates a new StateManager instance.
func NewStateManager() *StateManager {
	return &StateManager{}
}

// CreateRequest initializes a new request state, storing the offer and creating a channel to await a decision.
// It returns the decision channel for the caller to wait on.
func (m *StateManager) CreateRequest() (<-chan Decision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != nil || m.state.DecisionChan != nil {
		err := errors.New("invalid state: request already exists")
		log.Printf("Failed to create request: %v", err)
		return nil, err
	}
	m.state.DecisionChan = make(chan Decision, 1) // Buffered channel to avoid blocking
	m.state.AnswerChan = make(chan webrtc.SessionDescription, 1) // Buffered for the answer
	m.state.CandidateChan = make(chan webrtc.ICECandidateInit, 10) // Buffered for multiple candidates
	m.state.TransferDone = make(chan struct{}) // Channel to signal

	return m.state.DecisionChan, nil
}

func (m *StateManager) SetOffer(offer webrtc.SessionDescription) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != nil {
		err := errors.New("invalid state: request already exists")
		log.Printf("Failed to set offer: %v", err)
		return err
	}
	m.state = &RequestState{
		Offer: offer,
	}
	return nil
}

// GetOffer retrieves the currently stored offer.
func (m *StateManager) GetOffer() webrtc.SessionDescription {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == nil {
		return webrtc.SessionDescription{}
	}
	return m.state.Offer
}

// SetDecision records the user's decision and sends it to the waiting handler.
func (m *StateManager) SetDecision(decision Decision) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != nil && m.state.DecisionChan != nil {
		m.state.DecisionChan <- decision
		close(m.state.DecisionChan)
	}
}

// SetAnswer stores the generated answer from the WebRTC peer.
func (m *StateManager) SetAnswer(answer webrtc.SessionDescription) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != nil && m.state.AnswerChan != nil {
		m.state.AnswerChan <- answer
		close(m.state.AnswerChan)
	}
}

// GetAnswerChan returns the channel from which the answer can be read.
func (m *StateManager) GetAnswerChan() <-chan webrtc.SessionDescription {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == nil {
		// Return a closed channel if there's no active request
		ch := make(chan webrtc.SessionDescription)
		close(ch)
		return ch
	}
	return m.state.AnswerChan
}

// SetCandidate sends a new ICE candidate to the listening handler.
func (m *StateManager) SetCandidate(candidate webrtc.ICECandidateInit) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != nil && m.state.CandidateChan != nil {
		m.state.CandidateChan <- candidate
	}
}

// CloseCandidateChan closes the candidate channel to signal completion.
func (m *StateManager) CloseCandidateChan() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != nil && m.state.CandidateChan != nil {
		close(m.state.CandidateChan)
	}
}

// GetCandidateChan returns the channel from which ICE candidates can be read.
func (m *StateManager) GetCandidateChan() <-chan webrtc.ICECandidateInit {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == nil {
		ch := make(chan webrtc.ICECandidateInit)
		close(ch)
		return ch
	}
	return m.state.CandidateChan
}

// CloseRequest cleans up the state of the current request.
func (m *StateManager) CloseRequest() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != nil {
		// Signal that the transfer process is complete.
		close(m.state.TransferDone)
	}
	m.state = nil
}

// WaitForTransferDone returns a channel that blocks until the transfer is marked as complete.
func (m *StateManager) WaitForTransferDone() <-chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return m.state.TransferDone
}
