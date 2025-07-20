package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/pion/webrtc/v4"
)

type APISignaler struct {
	apiClient *Client
	ctx 	context.Context
}

const (
	DATA_PREFIX = "data:"
)

func NewAPISignaler(ctx context.Context, apiClient *Client) *APISignaler {
	return &APISignaler{
		apiClient: apiClient,
		ctx: ctx,
	}
}

func (s *APISignaler) SendOffer(offer webrtc.SessionDescription) error {
	return s.apiClient.SendOfferRequest(context.Background(), offer)
}

func (s *APISignaler) WaitForAnswer(ctx context.Context) (*webrtc.SessionDescription, error) {
	log.Println("Signaler: Connecting to Answer event stream...")

	url := s.apiClient.receiverURL + "/webrtc/answer-stream"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		err = fmt.Errorf("failed to request answer-stream %w", err)
		log.Printf("[waitForAnswer] %w", err)
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := s.apiClient.HttpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to connect to answer stream: %w", err)
		log.Printf("[waitForAnswer] %w", err)
		return nil, err
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, DATA_PREFIX) {
			jsonData := strings.TrimSpace(strings.TrimPrefix(line, DATA_PREFIX))
			var answer webrtc.SessionDescription
			if err := json.Unmarshal([]byte(jsonData), &answer); err != nil {
				err = fmt.Errorf("failed to decode sse answer event: %w", err)
				log.Printf("[waitForAnswer] %w", err)
				return nil, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		err = fmt.Errorf("error reading sse stream: %w", err)
		log.Printf("[waitForAnswer] %w", err)
		return nil, err
	}
	err = fmt.Errorf("event stream ended without an answer")
	log.Printf("[waitForAnswer] %w", err)
	return nil, err
}

func (s *APISignaler) SendICECandidate(candidate webrtc.ICECandidateInit) {
	go s.apiClient.SendICECandidateRequest(context.Background(), candidate)
}
