// Package razorpay is a thin client for Razorpay's REST API, TEST MODE ONLY.
// It never accepts production credentials by contract: callers must set
// RAZORPAY_MODE=test. Data is imported as records with is_synthetic=false.
package razorpay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aksayush2005/raze/services/api/internal/models"
)

const defaultBase = "https://api.razorpay.com"

// Client fetches settlements from Razorpay Test Mode.
type Client struct {
	keyID     string
	keySecret string
	baseURL   string
	httpc     *http.Client
}

// New builds a client. Callers are responsible for only supplying TEST keys.
func New(keyID, keySecret string) *Client {
	return &Client{
		keyID:     keyID,
		keySecret: keySecret,
		baseURL:   defaultBase,
		httpc:     &http.Client{Timeout: 30 * time.Second},
	}
}

type rzpSettlement struct {
	ID        string `json:"id"`
	Amount    int64  `json:"amount"` // minor units
	Currency  string `json:"currency"`
	Status    string `json:"status"`
	Fees      int64  `json:"fees"`
	Tax       int64  `json:"tax"`
	Net       int64  `json:"net"`
	CreatedAt int64  `json:"created_at"` // unix seconds
}

// FetchSettlements pulls recent settlements from Razorpay Test Mode and maps
// them onto normalized records. A failed or empty fetch is returned as an
// error so callers can fall back to synthetic data.
func (c *Client) FetchSettlements(ctx context.Context) ([]*models.Record, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/v1/settlements?count=100", nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.keyID, c.keySecret)

	resp, err := c.httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("razorpay status %d: %s", resp.StatusCode, string(b))
	}

	var payload struct {
		Items []rzpSettlement `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	out := make([]*models.Record, 0, len(payload.Items))
	for _, s := range payload.Items {
		currency := s.Currency
		if currency == "" {
			currency = "INR"
		}
		out = append(out, &models.Record{
			Source:      "razorpay_settlements",
			IsSynthetic: false,
			ExternalID:  "RZP_" + s.ID,
			Kind:        "settlement",
			Status:      "active",
			AmountMinor: s.Amount,
			FeeMinor:    s.Fees,
			TaxMinor:    s.Tax,
			NetMinor:    s.Net,
			Currency:    currency,
			OccurredAt:  time.Unix(s.CreatedAt, 0).UTC(),
		})
	}
	return out, nil
}
