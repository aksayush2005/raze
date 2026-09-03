// Package ai provides the client for the Python AI investigation service.
// It only produces advisory recommendations; the Go control plane applies
// everything. Failures are surfaced as errors so callers fail closed.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/aksayush2005/raze/services/api/internal/engine"
	"github.com/aksayush2005/raze/services/api/internal/models"
	"github.com/aksayush2005/raze/services/api/internal/services"
)

// Client talks to the Python AI service over HTTP/JSON (ADR-003).
type Client struct {
	baseURL string
	httpc   *http.Client
}

// NewClient returns nil when baseURL is empty (AI disabled — deterministic only).
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		return nil
	}
	return &Client{
		baseURL: baseURL,
		httpc:   &http.Client{Timeout: 30 * time.Second},
	}
}

// Advise requests a recommendation for a non-deterministic item. The returned
// advice is advisory only.
func (c *Client) Advise(ctx context.Context, item *models.Item, result *engine.Result) (*services.Advice, error) {
	if c == nil {
		// Fail closed: a disabled/unconfigured client is an AI failure, not a
		// crash. The reconciler routes on the error and keeps the deterministic
		// outcome.
		return nil, errors.New("ai client not configured")
	}
	payload := map[string]any{
		"item_id":       item.ID,
		"record_id":     item.RecordID,
		"decision":      result.Decision,
		"confidence":    result.Confidence,
		"candidates":    result.Candidates,
		"evidence":      result.Evidence,
		"reasons":       result.Reasons,
		"investigation": result.Investigation,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/advise", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ai service returned status %d", resp.StatusCode)
	}

	var advice services.Advice
	if err := json.NewDecoder(resp.Body).Decode(&advice); err != nil {
		return nil, fmt.Errorf("decode ai advice: %w", err)
	}
	return &advice, nil
}
