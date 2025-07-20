package webrtc

import (
	"testing"
	"context"
	"time"
	"sync"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/assert"
)

type mocksignaler struct {
	offerChan chan webrtc.SessionDescription
	answerChan chan webrtc.SessionDescription
	candidateChan chan webrtc.ICECandidateInit
}

func newMockSignaler() *mocksignaler {
	return &mocksignaler{
		offerChan: make(chan webrtc.SessionDescription, 1),
		answerChan: make(chan webrtc.SessionDescription, 1),
		candidateChan: make(chan webrtc.ICECandidateInit, 10),
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
	signaler := newMockSignaler()
	api := NewWebRTCAPI()
	require.NotNil(t, api)
	config := Config {}

	var wg sync.WaitGroup
	wg.Add(2)

	dataChanMsg := make(chan string, 1)

	// Create a sender connection
	senderConn, err := api.NewSenderConnection(config, signaler)
	if err != nil {
		t.Fatalf("Failed to create sender connection: %v", err)
	}
	require.NotNil(t, senderConn)

	// Create a receiver connection
	receiverConn, err := api.NewReceiverConnection(config)
	if err != nil {
		t.Fatalf("Failed to create receiver connection: %v", err)
	}
	require.NotNil(t, receiverConn)


	receiverConn.OnDataChannel(func(dc *webrtc.DataChannel) {
		t.Log("Receiver data channel opened")
		assert.Equal(t, "file-transfer", dc.Label())
		dc.OnOpen(func() {
			t.Log("Receiver data channel opened")
			wg.Done()
		})

		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			t.Logf("Receiver received message: %s", string(msg.Data))
		})
	})

	receiverConn.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			signaler.SendICECandidate(candidate.ToJSON())
		}
	})

	// mock receiver
	go func(){
		offer := <-signaler.offerChan
		answer, err := receiverConn.HandleOfferAndCreateAnswer(offer)
		if err != nil {
			t.Fatalf("Failed to handle offer and create answer: %v", err)
		}
		signaler.answerChan <- *answer
	}()

	// mock sender
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20 * time.Second)
		defer cancel()
		dc, err := senderConn.CreateDataChannel("file-transfer", nil)
		if err != nil {
			t.Fatalf("Failed to create data channel: %v", err)
		}
		dc.OnOpen(func() {
			t.Log("Data channel opened")
			err := dc.SendText("Hello, Receiver!")
			if err != nil {
				t.Fatalf("Failed to send text: %v", err)
			}
			wg.Done()
		})

		t.Logf("Sender establishing connection")
		err = senderConn.Establish(ctx)
		if err != nil {
			t.Fatalf("Failed to establish connection: %v", err)
		}
		t.Log("Sender connection established")
	}()
	waitChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitChan)
	}()

	select {
		case <-waitChan:
			t.Log("All data channels are ready")
		case <- time.After(15 * time.Second):
			t.Fatalf("Timed out waiting for data channels")
	}

	select {
	case msg := <-dataChanMsg:
		assert.Equal(t, "Hello, Receiver!", msg)
	case <-time.After(5 * time.Second):
		t.Fatalf("Timed out waiting for message")
	}

	require.NoError(t, senderConn.Close())
	require.NoError(t, receiverConn.Close())
}
