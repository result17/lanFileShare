package webrtc

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/rescp17/lanFileSharer/pkg/crypto"
	"github.com/rescp17/lanFileSharer/pkg/transfer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSignaler now correctly simulates the one-way signaler ownership.
// It implements the Signaler interface for the Sender, and provides
// test-only helper methods to simulate the Receiver sending data back.
type mockSignaler struct {
	offerChan          chan webrtc.SessionDescription
	answerChan         chan webrtc.SessionDescription
	senderCandidates   chan webrtc.ICECandidateInit // Candidates from Sender to Receiver
	receiverCandidates chan webrtc.ICECandidateInit // Candidates from Receiver to Sender
}

func newMockSignaler() *mockSignaler {
	return &mockSignaler{
		offerChan:          make(chan webrtc.SessionDescription, 1),
		answerChan:         make(chan webrtc.SessionDescription, 1),
		senderCandidates:   make(chan webrtc.ICECandidateInit, 10),
		receiverCandidates: make(chan webrtc.ICECandidateInit, 10),
	}
}

// --- Methods for the Signaler interface (used by Sender) ---

func (m *mockSignaler) SendOffer(ctx context.Context, offer webrtc.SessionDescription, signedFiles *crypto.SignedFileStructure) error {
	m.offerChan <- offer
	return nil
}

func (m *mockSignaler) SendICECandidate(ctx context.Context, candidate webrtc.ICECandidateInit) error {
	// This is called by the Sender, so the candidate is for the Receiver.
	select {
	case m.senderCandidates <- candidate:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *mockSignaler) WaitForAnswer(ctx context.Context) (*webrtc.SessionDescription, error) {
	select {
	case answer := <-m.answerChan:
		return &answer, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// --- Test-only helper methods to simulate Receiver's actions ---

func (m *mockSignaler) SendAnswerFromReceiver(answer webrtc.SessionDescription) {
	m.answerChan <- answer
}

func (m *mockSignaler) SendCandidateFromReceiver(candidate webrtc.ICECandidateInit) {
	m.receiverCandidates <- candidate
}

func processCandidates(t *testing.T, ctx context.Context, conn CommonConnection, candidateChan <-chan webrtc.ICECandidateInit, done <-chan struct{}, side string) {
	for {
		select {
		case candidate := <-candidateChan:
			t.Logf("%s: Adding peer's ICE candidate", side)
			if err := conn.Peer().AddICECandidate(candidate); err != nil {
				// Log non-critical errors, as some failures are expected
				t.Logf("%s failed to add ICE candidate: %v", side, err)
			}
		case <-ctx.Done():
			return
		case <-done:
			t.Logf("%s: Stopping candidate processing due to done signal", side)
			return
		}
	}
}

//nolint:gocyclo // Test function complexity is acceptable
func TestConnectionHandShake_CorrectArchitecture(t *testing.T) {
	const testTimeout = 20 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	signaler := newMockSignaler()
	api := NewWebrtcAPI()
	require.NotNil(t, api)
	config := Config{}

	errChan := make(chan error, 2)
	dataChanMsg := make(chan string, 1)
	done := make(chan struct{})
	var wg sync.WaitGroup

	// 1. Setup Receiver (does NOT get a signaler)
	receiverConn, err := api.NewReceiverConnection(config)
	require.NoError(t, err)
	defer func() {
		if err := receiverConn.Close(); err != nil {
			t.Logf("Error closing receiver connection: %v", err)
		}
	}()

	receiverConn.Peer().OnDataChannel(func(dc *webrtc.DataChannel) {
		assert.Equal(t, "file-transfer", dc.Label())
		dc.OnOpen(func() { t.Log("Receiver: DataChannel opened") })
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			t.Logf("Receiver: Received message: '%s'", string(msg.Data))
			dataChanMsg <- string(msg.Data)
		})
	})

	// The Receiver's OnICECandidate callback will now use the mock's helper method
	// to simulate sending the candidate back to the sender.
	receiverConn.Peer().OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			t.Log("Receiver: Got ICE candidate, sending via mock helper")
			signaler.SendCandidateFromReceiver(candidate.ToJSON())
		}
	})

	// 2. Setup Sender (gets the signaler)
	// Create a basic sender connection first, then we'll replace its signaler
	senderConn, err := api.NewSenderConnection(ctx, config, nil, "http://mock-receiver")
	require.NoError(t, err)
	defer func() {
		if err := senderConn.Close(); err != nil {
			t.Logf("Error closing sender connection: %v", err)
		}
	}()

	// Replace the signaler with our mock using the SetSignaler method
	if sc, ok := senderConn.(*SenderConn); ok {
		sc.SetSignaler(signaler)
	} else {
		t.Fatal("Failed to cast senderConn to *SenderConn")
	}

	// The Sender's OnICECandidate will call the required SendICECandidate method
	// from the Signaler interface.
	senderConn.Peer().OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			t.Log("Sender: Got ICE candidate, sending via Signaler interface")
			if err := signaler.SendICECandidate(ctx, candidate.ToJSON()); err != nil {
				errChan <- fmt.Errorf("sender failed to send ICE candidate: %w", err)
				return
			}
		}
	})

	// 3. Run Receiver logic in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		var offer webrtc.SessionDescription
		select {
		case offer = <-signaler.offerChan:
			t.Log("Receiver: Got offer")
		case <-ctx.Done():
			errChan <- fmt.Errorf("receiver timed out waiting for offer: %w", ctx.Err())
			return
		}

		answer, err := receiverConn.HandleOfferAndCreateAnswer(offer)
		if err != nil {
			errChan <- fmt.Errorf("receiver failed to handle offer: %w", err)
			return
		}
		// Use the test helper to send the answer back
		signaler.SendAnswerFromReceiver(*answer)
		t.Log("Receiver: Sent answer via mock helper")

		// Process candidates sent from the Sender
		processCandidates(t, ctx, receiverConn, signaler.senderCandidates, done, "Receiver")
	}()

	// 4. Run Sender logic in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		dc, err := senderConn.Peer().CreateDataChannel("file-transfer", nil)
		if err != nil {
			select {
			case errChan <- fmt.Errorf("sender failed to create data channel: %w", err):
			default:
			}
			return
		}
		dc.OnOpen(func() {
			t.Log("Sender: DataChannel opened, sending message")
			if err := dc.SendText("Hello, Receiver!"); err != nil {
				select {
				case errChan <- fmt.Errorf("sender failed to send text: %w", err):
				default:
				}
			}
		})

		wg.Add(1)
		go func() {
			defer wg.Done()
			processCandidates(t, ctx, senderConn, signaler.receiverCandidates, done, "Sender")
		}()

		// This will create offer, send it, and wait for the answer
		// Create empty FileStructureManager for testing
		emptyFSM := transfer.NewFileStructureManager()
		if err := senderConn.Establish(ctx, emptyFSM); err != nil {
			select {
			case errChan <- fmt.Errorf("sender failed to establish connection: %w", err):
			default:
			}
			return
		}
		t.Log("Sender: Connection established")
	}()

	// 5. Wait for the final message or a timeout/error
	select {
	case msg := <-dataChanMsg:
		assert.Equal(t, "Hello, Receiver!", msg)
		t.Log("SUCCESS: Message received successfully.")
		close(done)
	case err := <-errChan:
		t.Fatalf("A goroutine reported an error: %v", err)
	case <-ctx.Done():
		t.Fatal("Test timed out waiting for message")
	}

	wg.Wait()
}
