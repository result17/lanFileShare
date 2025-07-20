package webrtc

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mocksignaler's design is correct and remains unchanged.
type mocksignaler struct {
	offerChan     chan webrtc.SessionDescription
	answerChan    chan webrtc.SessionDescription
	candidateChan chan webrtc.ICECandidateInit
}

func newMockSignaler() *mocksignaler {
	return &mocksignaler{
		offerChan:     make(chan webrtc.SessionDescription, 1),
		answerChan:    make(chan webrtc.SessionDescription, 1),
		candidateChan: make(chan webrtc.ICECandidateInit, 20), // Increased buffer size as both peers will send candidates.
	}
}

func (m *mocksignaler) SendOffer(offer webrtc.SessionDescription) error {
	m.offerChan <- offer
	return nil
}

func (m *mocksignaler) SendICECandidate(candidate webrtc.ICECandidateInit) {
	m.candidateChan <- candidate
}

func (m *mocksignaler) WaitForAnswer(ctx context.Context) (*webrtc.SessionDescription, error) {
	select {
	case answer := <-m.answerChan:
		return &answer, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestConnectionHandShake(t *testing.T) {
	// 1. Initialization
	signaler := newMockSignaler()
	api := NewWebRTCAPI()
	require.NotNil(t, api)
	config := Config{}

	senderConn, err := api.NewSenderConnection(config, signaler)
	require.NoError(t, err)
	defer senderConn.Close()

	receiverConn, err := api.NewReceiverConnection(config)
	require.NoError(t, err)
	defer receiverConn.Close()

	// 2. Setup channels for synchronization and data exchange
	var wg sync.WaitGroup
	wg.Add(2) // Wait for both DataChannels to open

	dataChanMsg := make(chan string, 1)
	// Improvement: Create an error channel to safely report errors from goroutines
	errChan := make(chan error, 2)

	// 3. Configure Receiver's callbacks
	receiverConn.OnDataChannel(func(dc *webrtc.DataChannel) {
		assert.Equal(t, "file-transfer", dc.Label())
		dc.OnOpen(func() {
			t.Log("Receiver: DataChannel opened")
			wg.Done()
		})
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			t.Logf("Receiver: Received message: '%s'", string(msg.Data))
			dataChanMsg <- string(msg.Data)
		})
	})
	receiverConn.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			signaler.SendICECandidate(candidate.ToJSON())
		}
	})

	// 4. Improvement: Set OnICECandidate callback for the Sender as well
	senderConn.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			signaler.SendICECandidate(candidate.ToJSON())
		}
	})

	// 5. Simulate Receiver's behavior in a goroutine
	go func() {
		offer := <-signaler.offerChan
		answer, err := receiverConn.HandleOfferAndCreateAnswer(offer)
		if err != nil {
			errChan <- fmt.Errorf("receiver failed to handle offer: %w", err)
			return
		}
		signaler.answerChan <- *answer
	}()

	// 6. Simulate Sender's behavior in a goroutine
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		dc, err := senderConn.CreateDataChannel("file-transfer", nil)
		if err != nil {
			errChan <- fmt.Errorf("sender failed to create data channel: %w", err)
			return
		}
		dc.OnOpen(func() {
			t.Log("Sender: DataChannel opened")
			if err := dc.SendText("Hello, Receiver!"); err != nil {
				errChan <- fmt.Errorf("sender failed to send text: %w", err)
			}
			wg.Done()
		})

		if err := senderConn.Establish(ctx); err != nil {
			errChan <- fmt.Errorf("sender failed to establish connection: %w", err)
		}
	}()

	// 7. !!! KEY FIX: Create a goroutine to consume and distribute ICE Candidates !!!
	// This loop will run until the DataChannels are open (signaled by closing `done`).
	done := make(chan struct{})
	go func() {
		for {
			select {
			case candidate := <-signaler.candidateChan:
				// This is a simplified model where we assume the received candidate is for the other peer.
				// In this test, we can't easily distinguish the origin of the candidate,
				// so we attempt to add it to both peers. AddICECandidate will automatically
				// ignore candidates that don't belong to it. This is a simple and effective testing trick.
				_ = senderConn.AddICECandidate(candidate)
				_ = receiverConn.AddICECandidate(candidate)
			case <-done:
				return
			}
		}
	}()

	// 8. Wait for DataChannels to open or for an error to occur
	waitChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitChan)
	}()

	select {
	case <-waitChan:
		t.Log("All data channels are ready.")
		close(done) // DataChannels are open, we can stop the ICE exchange goroutine.
	case err := <-errChan:
		t.Fatalf("Test failed with error from goroutine: %v", err)
	case <-time.After(15 * time.Second):
		t.Fatal("Timed out waiting for data channels")
	}

	// 9. Verify message reception and check for any final errors
	select {
	case msg := <-dataChanMsg:
		assert.Equal(t, "Hello, Receiver!", msg)
		t.Log("SUCCESS: Message received successfully.")
	case err := <-errChan:
		t.Fatalf("Test failed with error from goroutine: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for message")
	}
}
