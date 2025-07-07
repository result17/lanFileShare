package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
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
	httpClient *http.Client
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
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			// 3. Set the custom transport on the http.Client.
			Transport: transport,
		},
	}
}

// AskPayload defines the structure for the /ask request.
type AskPayload struct {
	Files []*fileInfo.FileNode `json:"files"`
}

// SendAskRequest sends the list of files to the receiver and asks for confirmation.
// It no longer needs to know about the service ID; the transport handles it automatically.
func (c *Client) SendAskRequest(ctx context.Context, receiverURL string, files []*fileInfo.FileNode) error {
	payload := AskPayload{
		Files: files,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal ask payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", receiverURL+"/ask", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create ask request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send ask request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("receiver responded with non-OK status: %s", resp.Status)
	}

	return nil
}
