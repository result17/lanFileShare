package webrtc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
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

func (m *mockSignaler) SendOffer(offer webrtc.SessionDescription, fileNodes []fileInfo.FileNode) error {
	m.offerChan <- offer
	return nil
}

func (m *mockSignaler) SendICECandidate(candidate webrtc.ICECandidateInit) {
	// This is called by the Sender, so the candidate is for the Receiver.
	m.senderCandidates <- candidate
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

func TestConnectionHandShake_CorrectArchitecture(t *testing.T) {
	const testTimeout = 20 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	signaler := newMockSignaler()
	api := NewWebrtcAPI()
	require.NotNil(t, api)
	config := Config{}

	dataChanMsg := make(chan string, 1)
	errChan := make(chan error, 3)

	// 1. Setup Receiver (does NOT get a signaler)
	receiverConn, err := api.NewReceiverConnection(config)
	require.NoError(t, err)
	defer receiverConn.Close()

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
	// We pass the mockSignaler, which satisfies the Signaler interface.
	senderConn, err := api.NewSenderConnection(ctx, config, nil)
	require.NoError(t, err)
	defer senderConn.Close()

	// The Sender's OnICECandidate will call the required SendICECandidate method
	// from the Signaler interface.
	senderConn.Peer().OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			t.Log("Sender: Got ICE candidate, sending via Signaler interface")
			signaler.SendICECandidate(candidate.ToJSON())
		}
	})

	// 3. Run Receiver logic in a goroutine
	go func() {
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
		for {
			select {
			case candidate := <-signaler.senderCandidates:
				t.Log("Receiver: Adding sender's ICE candidate")
				if err := receiverConn.Peer().AddICECandidate(candidate); err != nil {
					// Log non-critical errors, as some failures are expected
					t.Logf("Receiver failed to add ICE candidate: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// 4. Run Sender logic in a goroutine
	go func() {
		dc, err := senderConn.Peer().CreateDataChannel("file-transfer", nil)
		if err != nil {
			errChan <- fmt.Errorf("sender failed to create data channel: %w", err)
			return
		}
		dc.OnOpen(func() {
			t.Log("Sender: DataChannel opened, sending message")
			if err := dc.SendText("Hello, Receiver!"); err != nil {
				errChan <- fmt.Errorf("sender failed to send text: %w", err)
			}
		})

		// This will create offer, send it, and wait for the answer
		if err := senderConn.Establish(ctx, nil); err != nil {
			errChan <- fmt.Errorf("sender failed to establish connection: %w", err)
			return
		}
		t.Log("Sender: Connection established")

		// Process candidates sent from the Receiver
		for {
			select {
			case candidate := <-signaler.receiverCandidates:
				t.Log("Sender: Adding receiver's ICE candidate")
				if err := senderConn.Peer().AddICECandidate(candidate); err != nil {
					t.Logf("Sender failed to add ICE candidate: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// 5. Wait for the final message or a timeout/error
	select {
	case msg := <-dataChanMsg:
		assert.Equal(t, "Hello, Receiver!", msg)
		t.Log("SUCCESS: Message received successfully.")
	case err := <-errChan:
		t.Fatalf("Test failed with error from goroutine: %v", err)
	case <-ctx.Done():
		// Check for a lingering error before declaring a timeout
		select {
		case err := <-errChan:
			t.Fatalf("Test failed with error from goroutine: %v", err)
		default:
			t.Fatal("Test timed out waiting for message")
		}
	}
}
