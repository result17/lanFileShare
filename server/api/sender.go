package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

type AskResponse struct {
	Accepted bool `json:"accepted"`
}

type AskPayload struct {
	ServiceID string `json:"serviceID"`
	infos  []*fileInfo.FileNode `json:"serviceID"`
}

func Ask(ctx context.Context, receiverURL string, serviceID string, files []*fileInfo.FileNode) (bool, error) {
	payload := &AskPayload{
		ServiceID: serviceID,
		infos: files,
	}
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("Failed to marshal file nodes to JSON: %w", err)
	}

	reqBody := bytes.NewBuffer(jsonData)

	askURL := fmt.Sprintf("%s/ask", receiverURL)

	req, err := http.NewRequestWithContext(ctx, "POST", askURL, reqBody)
	if err != nil {
		return false, fmt.Errorf("Failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("Failed to send request to receiver: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("receiver responded with status %s: %s", resp.Status, string(bodyBytes))
	}
	var askResp AskResponse
	if err := json.NewDecoder(resp.Body).Decode(&askResp); err != nil {
		return false, fmt.Errorf("Failed to decode response from receiver: %w", err)
	}
	return askResp.Accepted, nil
}
