package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/pion/webrtc/v4"
)

const serviceIDHeader = "X-Service-ID"

// serviceIDInjector is a custom http.RoundTripper that injects a service ID into each request.
type serviceIDInjector struct {
	serviceID string
	next      http.RoundTripper
}

// RoundTrip intercepts the request, adds the service ID header, and passes it to the next transport.
func (t *serviceIDInjector) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set(serviceIDHeader, t.serviceID)
	return t.next.RoundTrip(req)
}

// Client is a stateless HTTP client for communicating with the receiver's API.
type Client struct {
	HttpClient  *http.Client
	receiverURL string
}

// NewClient creates a new API client, configured to automatically inject the provided serviceID.
func NewClient(serviceID string) *Client {
	// 1. Create our custom transport (client-side middleware).
	transport := &serviceIDInjector{
		serviceID: serviceID,
		// 2. Use the default transport as the next step in the chain.
		next: http.DefaultTransport,
	}

	return &Client{
		HttpClient: &http.Client{
			Timeout: 30 * time.Second,
			// 3. Set the custom transport on the http.Client.
			Transport: transport,
		},
	}
}

func (c *Client) SetReceiverURL(receiverURL string) {
	c.receiverURL = receiverURL
}

// TODO: The receiver API currently does not have an endpoint to handle this request.
// This method is kept as a placeholder for the necessary sender-to-receiver
// ICE candidate signaling. Implementing the corresponding endpoint on the receiver
// is required to make WebRTC work in more complex network environments.
func (c *Client) SendICECandidateRequest(ctx context.Context, candidate webrtc.ICECandidateInit) error {
	if c.receiverURL == "" {
		log.Printf("receiver can not be empty.")
	}

	jsonData, err := json.Marshal(candidate)
	if err != nil {
		return fmt.Errorf("failed to marshal candidate payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.receiverURL+"/candidate", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create candidate request: %w", err)
	}
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send candidate request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("candidate responded with non-OK status: %s", resp.Status)
	}

	return nil
}
