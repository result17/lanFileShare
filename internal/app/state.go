package app

import (
	"errors"
	"log/slog"
	"sync"

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

// SetOffer is the first step in creating a request. It stores the offer
// and initializes a partial state. It fails if a request is already active.
func (m *StateManager) SetOffer(offer webrtc.SessionDescription) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != nil {
		return errors.New("a request is already in progress")
	}
	m.state = &RequestState{Offer: offer}
	return nil
}

// CreateRequest finishes initializing the request state created by SetOffer.
// It returns the decision channel for the caller to wait on.
func (m *StateManager) CreateRequest() (<-chan Decision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == nil || m.state.DecisionChan != nil {
		return nil, errors.New("invalid state: SetOffer must be called first and exactly once")
	}

	m.state.DecisionChan = make(chan Decision, 1)
	m.state.AnswerChan = make(chan webrtc.SessionDescription, 1) // Correct type
	m.state.CandidateChan = make(chan webrtc.ICECandidateInit, 10)
	m.state.TransferDone = make(chan struct{})

	return m.state.DecisionChan, nil
}

// GetOffer retrieves the currently stored offer.
func (m *StateManager) GetOffer() (webrtc.SessionDescription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == nil {
		return webrtc.SessionDescription{}, errors.New("no active request state")
	}
	return m.state.Offer, nil
}

// SetDecision records the user's decision and sends it to the waiting handler.
func (m *StateManager) SetDecision(decision Decision) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != nil && m.state.DecisionChan != nil {
		select {
		case _, ok := <-m.state.DecisionChan:
			if !ok {
				return
			}
		default:
		}
		m.state.DecisionChan <- decision
		close(m.state.DecisionChan)
	}
}

// SetAnswer stores the generated answer from the WebRTC peer.
func (m *StateManager) SetAnswer(answer webrtc.SessionDescription) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != nil && m.state.AnswerChan != nil {
		defer func() {
			if r := recover(); r != nil {
				// Ignore "send on closed channel" panic
				slog.Warn("Attempted to send answer on closed channel", "error", r)
			}
		}()
		m.state.AnswerChan <- answer
		close(m.state.AnswerChan) // Answer is sent only once
	}
}

// GetAnswerChan returns the channel from which the answer can be read.
func (m *StateManager) GetAnswerChan() <-chan webrtc.SessionDescription {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == nil {
		ch := make(chan webrtc.SessionDescription)
		close(ch)
		return ch
	}
	return m.state.AnswerChan
}

// SetCandidate sends a new ICE candidate to the listening handler.
func (m *StateManager) SetCandidate(candidate webrtc.ICECandidateInit) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != nil && m.state.CandidateChan != nil {
		defer func() {
			if r := recover(); r != nil {
				// Ignore "send on closed channel" panic
				slog.Warn("Attempted to send candidate on closed channel", "error", r)
			}
		}()
		m.state.CandidateChan <- candidate
		return nil
	}
	return errors.New("no active request state or candidate channel is not initialized")
}

// CloseCandidateChan closes the candidate channel to signal completion.
func (m *StateManager) CloseCandidateChan() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != nil && m.state.CandidateChan != nil {
		defer func() {
			if r := recover(); r != nil {
				// Ignore "send on closed channel" panic
				slog.Warn("Attempted to send candidate on closed channel", "error", r)
			}
		}()
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
		defer func() {
			recover()
		}()
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