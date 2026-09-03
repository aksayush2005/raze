// Command raze-tui is a terminal UI for the RAZE control plane.
//
// It is a pure HTTP client: every piece of data it shows comes from the API,
// and every action it takes goes through the same endpoints the web SPA uses
// (POST /api/v1/jobs, POST /api/v1/items/{id}/review, ...). It never reads the
// database and never mutates state outside the API.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aksayush2005/raze/services/api/internal/models"
)

// client is a thin HTTP client for the RAZE control plane.
type client struct {
	baseURL string
	httpc   *http.Client
}

func newClient(baseURL string) *client {
	return &client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpc:   &http.Client{Timeout: 15 * time.Second},
	}
}

// apiError surfaces the server's {"error": "..."} body to the UI.
type apiError struct {
	Status int
	Msg    string
}

func (e *apiError) Error() string {
	if e.Msg != "" {
		return fmt.Sprintf("api %d: %s", e.Status, e.Msg)
	}
	return fmt.Sprintf("api status %d", e.Status)
}

// do performs a request. body==nil means GET. idem, when non-empty, is sent as
// the Idempotency-Key header (required by the API for all mutations).
func (c *client) do(ctx context.Context, method, path string, body any, idem string, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	resp, err := c.httpc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(data, &e)
		return &apiError{Status: resp.StatusCode, Msg: e.Error}
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

// idemKey returns a fresh idempotency key for one mutation.
func idemKey() string { return fmt.Sprintf("tui-%d", time.Now().UnixNano()) }

// itemDetail mirrors the GET /api/v1/items/{id} workspace payload.
type itemDetail struct {
	Item        *models.Item         `json:"item"`
	Record      *models.Record       `json:"record"`
	MatchRecord *models.Record       `json:"match_record,omitempty"`
	Candidates  []*models.Candidate  `json:"candidates"`
	Evidence    []*models.Evidence   `json:"evidence"`
	Audit       []*models.AuditEvent `json:"audit"`
	AIDecisions []*models.AIDecision `json:"ai_decisions"`
}

type jobsResp struct {
	Jobs []*models.Job `json:"jobs"`
}

type itemsResp struct {
	Items []*models.ItemView `json:"items"`
}

type recordsResp struct {
	Records []*models.Record `json:"records"`
}

type importReq struct {
	Records []*models.Record `json:"records"`
}

type importResp struct {
	Imported int `json:"imported"`
}

type createJobReq struct {
	Name   string         `json:"name"`
	Config map[string]any `json:"config"`
}

type reviewReq struct {
	Action         string `json:"action"`
	Actor          string `json:"actor"`
	Note           string `json:"note"`
	TargetRecordID *int64 `json:"target_record_id"`
}

// Health checks GET /healthz.
func (c *client) Health(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/healthz", nil, "", nil)
}

// ListJobs returns recent reconciliation jobs.
func (c *client) ListJobs(ctx context.Context, limit int) ([]*models.Job, error) {
	var out jobsResp
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/jobs?limit=%d", limit), nil, "", &out); err != nil {
		return nil, err
	}
	return out.Jobs, nil
}

// CreateJob starts a new reconciliation run.
func (c *client) CreateJob(ctx context.Context, name string, config map[string]any) (*models.Job, error) {
	if name == "" {
		name = "reconciliation"
	}
	if config == nil {
		config = map[string]any{}
	}
	var job models.Job
	if err := c.do(ctx, http.MethodPost, "/api/v1/jobs", createJobReq{Name: name, Config: config}, idemKey(), &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// GetJob returns one job by id.
func (c *client) GetJob(ctx context.Context, id int64) (*models.Job, error) {
	var job models.Job
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/jobs/%d", id), nil, "", &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// ListJobItems lists the reconciliation items of a job, optionally filtered by
// item status ("" = all).
func (c *client) ListJobItems(ctx context.Context, jobID int64, status string, limit int) ([]*models.ItemView, error) {
	var out itemsResp
	path := fmt.Sprintf("/api/v1/jobs/%d/items?limit=%d", jobID, limit)
	if status != "" {
		path += "&status=" + status
	}
	if err := c.do(ctx, http.MethodGet, path, nil, "", &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

// GetItem loads the full workspace for one item (record, match, candidates,
// evidence, audit, AI decisions).
func (c *client) GetItem(ctx context.Context, id int64) (*itemDetail, error) {
	var d itemDetail
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/items/%d", id), nil, "", &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// ReviewItem applies a human review action. target is required for
// MANUALLY_LINKED_RECORDS, ignored otherwise.
func (c *client) ReviewItem(ctx context.Context, id int64, action, actor string, target *int64) (*models.Item, error) {
	var item models.Item
	req := reviewReq{Action: action, Actor: actor, TargetRecordID: target}
	if err := c.do(ctx, http.MethodPost, fmt.Sprintf("/api/v1/items/%d/review", id), req, idemKey(), &item); err != nil {
		return nil, err
	}
	return &item, nil
}

// ListRecords returns recent normalized records.
func (c *client) ListRecords(ctx context.Context, limit int) ([]*models.Record, error) {
	var out recordsResp
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/records?limit=%d", limit), nil, "", &out); err != nil {
		return nil, err
	}
	return out.Records, nil
}

// ImportRecords upserts records (idempotent on source+external_id).
func (c *client) ImportRecords(ctx context.Context, recs []*models.Record) (int, error) {
	var out importResp
	if err := c.do(ctx, http.MethodPost, "/api/v1/records/import", importReq{Records: recs}, idemKey(), &out); err != nil {
		return 0, err
	}
	return out.Imported, nil
}
