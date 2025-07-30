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
	decisionSent  bool
	answerSent    bool
	candidateChanClose bool
	transferDoneClose bool
}

// StateManager manages the lifecycle of a file transfer request state in a concurrent-safe manner.
type SingleRequestManager  struct {
	mu    sync.Mutex
	state *RequestState // Holds the state for the *single* active request
}

// NewStateManager creates a new StateManager instance.
func NewSingleRequestManager() *SingleRequestManager {
	return &SingleRequestManager{}
}

// CreateRequest finishes initializing the request state created by SetOffer.
// It returns the decision channel for the caller to wait on.
func (m *SingleRequestManager) CreateRequest(offer webrtc.SessionDescription) (<-chan Decision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != nil {
		return nil, errors.New("a request is already in progress")
	}

	m.state = &RequestState{
		Offer:         offer,
		DecisionChan:  make(chan Decision, 1),
		AnswerChan:    make(chan webrtc.SessionDescription, 1),
		CandidateChan: make(chan webrtc.ICECandidateInit, 10),
		TransferDone:  make(chan struct{}),
	}

	return m.state.DecisionChan, nil
}

// GetOffer retrieves the currently stored offer.
func (m *SingleRequestManager) GetOffer() (webrtc.SessionDescription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == nil {
		return webrtc.SessionDescription{}, errors.New("no active request state")
	}
	return m.state.Offer, nil
}

// SetDecision records the user's decision and sends it to the waiting handler.
func (m *SingleRequestManager) SetDecision(decision Decision) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == nil || m.state.DecisionChan == nil {
		slog.Error("no active request")
		return errors.New("no active request")
	}
	if m.state.decisionSent {
		slog.Error("a decision has already been made")
		return errors.New("a decision has already been made")
	}
	m.state.DecisionChan <- decision
	m.state.decisionSent = true
	close(m.state.DecisionChan)
	return nil
}

// SetAnswer stores the generated answer from the WebRTC peer.
func (m *SingleRequestManager) SetAnswer(answer webrtc.SessionDescription) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == nil || m.state.AnswerChan == nil || m.state.answerSent {
		slog.Error("an answer has already been sent")
		return errors.New("an answer has already been sent")
	}

	m.state.AnswerChan <- answer
	close(m.state.AnswerChan)
	m.state.answerSent = true
	return nil
}

// GetAnswerChan returns the channel from which the answer can be read.
func (m *SingleRequestManager) GetAnswerChan() <-chan webrtc.SessionDescription {
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
func (m *SingleRequestManager) SetCandidate(candidate webrtc.ICECandidateInit) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == nil || m.state.candidateChanClose {
		return errors.New("request is not active or candidates are no longer accepted")
	}

	m.state.CandidateChan <- candidate
	return nil
}

// CloseCandidateChan closes the candidate channel to signal completion.
func (m *SingleRequestManager) CloseCandidateChan() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != nil || m.state.CandidateChan != nil {
		defer func() {
			if r := recover(); r != nil {
				// Ignore "send on closed channel" panic
				slog.Warn("Attempted to send candidate on closed channel", "error", r)
			}
		}()
		close(m.state.CandidateChan)
		m.state.candidateChanClose = true
	}
}

// GetCandidateChan returns the channel from which ICE candidates can be read.
func (m *SingleRequestManager) GetCandidateChan() <-chan webrtc.ICECandidateInit {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == nil || m.state.candidateChanClose {
		ch := make(chan webrtc.ICECandidateInit)
		close(ch)
		return ch
	}
	return m.state.CandidateChan
}

// CloseRequest cleans up the state of the current request.
func (m *SingleRequestManager) CloseRequest() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != nil {
		defer func() {
			recover()
		}()
		close(m.state.TransferDone)
		m.state.transferDoneClose = true
	}
	m.state = nil
}

// WaitForTransferDone returns a channel that blocks until the transfer is marked as complete.
func (m *SingleRequestManager) WaitForTransferDone() <-chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return m.state.TransferDone
}
