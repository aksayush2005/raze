// Package api_test exercises the full control-plane API against a real
// Postgres database. Run with a migrated DB: DATABASE_URL=postgres://... go test ./...
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aksayush2005/raze/services/api/internal/engine"
	"github.com/aksayush2005/raze/services/api/internal/handlers"
	"github.com/aksayush2005/raze/services/api/internal/repositories"
	"github.com/aksayush2005/raze/services/api/internal/services"
)

// newTestApp boots the full app (control plane + worker) against a real DB.
func newTestApp(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	store := repositories.New(pool)
	eng := engine.New(store)
	recon := services.NewReconciler(store, eng, nil, 2) // deterministic-only mode
	review := services.NewReviewService(store)
	app := handlers.NewApp(store, recon, review, nil)

	ctx, cancel := context.WithCancel(context.Background())
	go recon.RunWorker(ctx)

	ts := httptest.NewServer(app.Routes())
	cleanup := func() { cancel(); ts.Close(); pool.Close() }
	t.Cleanup(cleanup)
	return ts, cleanup
}

func doJSON(t *testing.T, ts *httptest.Server, method, path string, body any, key string) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req, err := http.NewRequest(method, ts.URL+path, &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode %s %s (%d): %v", method, path, resp.StatusCode, err)
	}
	return resp.StatusCode, out
}

func waitJobComplete(t *testing.T, ts *httptest.Server, id int64) map[string]any {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		code, job := doJSON(t, ts, "GET", fmt.Sprintf("/api/v1/jobs/%d", id), nil, "")
		if code != http.StatusOK {
			t.Fatalf("get job: HTTP %d", code)
		}
		if job["status"] == "COMPLETED" {
			return job
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("job %d did not complete in time", id)
	return nil
}

func TestEndToEndReconciliation(t *testing.T) {
	ts, _ := newTestApp(t)

	// Import a deterministic dataset covering every decision band:
	//   SET-1/PAY-1  exact reference, fully verified          -> RECONCILED
	//   SET-2/PAY-F  fuzzy (no reference), fully verified      -> REVIEW
	//   SET-3/PAY-3  exact reference, gross mismatch           -> ESCALATED
	//   SET-4/PAY-4  exact reference, arithmetic inconsistency -> ESCALATED
	//   PAY-M        references a missing settlement           -> ESCALATED
	//   PAY-O        orphan, no candidate                      -> ESCALATED
	records := []map[string]any{
		{"source": "synthetic", "is_synthetic": true, "external_id": "SET-1", "kind": "settlement",
			"amount_minor": 10000, "fee_minor": 150, "tax_minor": 27, "net_minor": 9823, "occurred_at": "2026-08-01T10:00:00Z"},
		{"source": "synthetic", "is_synthetic": true, "external_id": "PAY-1", "kind": "payment",
			"amount_minor": 10000, "fee_minor": 0, "tax_minor": 0, "net_minor": 10000, "occurred_at": "2026-08-01T10:05:00Z", "ref_external_id": "SET-1"},
		{"source": "synthetic", "is_synthetic": true, "external_id": "SET-2", "kind": "settlement",
			"amount_minor": 3000, "fee_minor": 45, "tax_minor": 8, "net_minor": 2947, "occurred_at": "2026-08-02T10:00:00Z"},
		{"source": "synthetic", "is_synthetic": true, "external_id": "PAY-F", "kind": "payment",
			"amount_minor": 3000, "fee_minor": 0, "tax_minor": 0, "net_minor": 3000, "occurred_at": "2026-08-02T10:30:00Z"},
		{"source": "synthetic", "is_synthetic": true, "external_id": "SET-3", "kind": "settlement",
			"amount_minor": 5000, "fee_minor": 75, "tax_minor": 13, "net_minor": 4912, "occurred_at": "2026-08-03T10:00:00Z"},
		{"source": "synthetic", "is_synthetic": true, "external_id": "PAY-3", "kind": "payment",
			"amount_minor": 4800, "fee_minor": 0, "tax_minor": 0, "net_minor": 4800, "occurred_at": "2026-08-03T10:05:00Z", "ref_external_id": "SET-3"},
		{"source": "synthetic", "is_synthetic": true, "external_id": "SET-4", "kind": "settlement",
			"amount_minor": 8000, "fee_minor": 120, "tax_minor": 21, "net_minor": 7000, "occurred_at": "2026-08-04T10:00:00Z"},
		{"source": "synthetic", "is_synthetic": true, "external_id": "PAY-4", "kind": "payment",
			"amount_minor": 8000, "fee_minor": 0, "tax_minor": 0, "net_minor": 8000, "occurred_at": "2026-08-04T10:05:00Z", "ref_external_id": "SET-4"},
		{"source": "synthetic", "is_synthetic": true, "external_id": "PAY-M", "kind": "payment",
			"amount_minor": 7000, "fee_minor": 0, "tax_minor": 0, "net_minor": 7000, "occurred_at": "2026-08-05T10:00:00Z", "ref_external_id": "SET-MISSING"},
		{"source": "synthetic", "is_synthetic": true, "external_id": "PAY-O", "kind": "payment",
			"amount_minor": 1111, "fee_minor": 0, "tax_minor": 0, "net_minor": 1111, "occurred_at": "2026-08-06T10:00:00Z"},
	}

	// Idempotency: same key + same body replays; same key + different body -> 409.
	body := map[string]any{"records": records}
	code, first := doJSON(t, ts, "POST", "/api/v1/records/import", body, "imp-key-1")
	if code != http.StatusOK || first["imported"] != float64(len(records)) {
		t.Fatalf("import = HTTP %d %v, want %d imported", code, first, len(records))
	}
	code, replay := doJSON(t, ts, "POST", "/api/v1/records/import", body, "imp-key-1")
	if code != http.StatusOK || replay["imported"] != first["imported"] {
		t.Fatalf("idempotent replay = HTTP %d %v, want replay of %v", code, replay, first)
	}
	code, _ = doJSON(t, ts, "POST", "/api/v1/records/import", map[string]any{"records": records[:1]}, "imp-key-1")
	if code != http.StatusConflict {
		t.Fatalf("key reuse with different body = HTTP %d, want 409", code)
	}

	// Create and run the reconciliation job.
	code, job := doJSON(t, ts, "POST", "/api/v1/jobs", map[string]any{"name": "integration"}, "")
	if code != http.StatusCreated {
		t.Fatalf("create job = HTTP %d %v", code, job)
	}
	jobID := int64(job["id"].(float64))
	done := waitJobComplete(t, ts, jobID)

	for _, tc := range []struct {
		want int64
		key  string
	}{
		{2, "matched"},
		{2, "review"},
		{6, "escalated"},
		{10, "total_records"},
	} {
		if got := int64(done[tc.key].(float64)); got != tc.want {
			t.Errorf("job %s = %d, want %d", tc.key, got, tc.want)
		}
	}

	// Verify a RECONCILED item carries deterministic evidence and a match.
	_, items := doJSON(t, ts, "GET", fmt.Sprintf("/api/v1/jobs/%d/items?status=RESOLVED", jobID), nil, "")
	got := []map[string]any{}
	raw, _ := json.Marshal(items["items"])
	_ = json.Unmarshal(raw, &got)
	if len(got) != 2 {
		t.Fatalf("RESOLVED items = %d, want 2", len(got))
	}
	code, detail := doJSON(t, ts, "GET", fmt.Sprintf("/api/v1/items/%d", int64(got[0]["id"].(float64))), nil, "")
	if code != http.StatusOK {
		t.Fatalf("get item detail = HTTP %d", code)
	}
	ev := detail["evidence"].([]any)
	types := map[string]bool{}
	for _, e := range ev {
		types[e.(map[string]any)["type"].(string)] = true
	}
	for _, wantType := range []string{"EXACT_REFERENCE_MATCH", "AMOUNT_EXPLAINED", "SETTLEMENT_ARITHMETIC_VALID"} {
		if !types[wantType] {
			t.Errorf("reconciled item missing evidence %s (got %v)", wantType, types)
		}
	}
	if detail["match_record"] == nil {
		t.Error("reconciled item missing match_record")
	}

	// Human review: accept a REVIEW item.
	_, rev := doJSON(t, ts, "GET", fmt.Sprintf("/api/v1/jobs/%d/items?status=REVIEW", jobID), nil, "")
	var revItems []map[string]any
	raw, _ = json.Marshal(rev["items"])
	_ = json.Unmarshal(raw, &revItems)
	if len(revItems) != 2 {
		t.Fatalf("REVIEW items = %d, want 2", len(revItems))
	}
	itemID := int64(revItems[0]["id"].(float64))
	code, reviewed := doJSON(t, ts, "POST", fmt.Sprintf("/api/v1/items/%d/review", itemID),
		map[string]any{"action": "ACCEPTED_AGENT_MATCH", "actor": "operator@test"}, "")
	if code != http.StatusOK {
		t.Fatalf("accept review = HTTP %d %v", code, reviewed)
	}
	if reviewed["status"] != "RESOLVED" || reviewed["decision"] != "RECONCILED" {
		t.Fatalf("accept review -> %v", reviewed)
	}
	code, detail = doJSON(t, ts, "GET", fmt.Sprintf("/api/v1/items/%d", itemID), nil, "")
	if code != http.StatusOK {
		t.Fatalf("get reviewed item = HTTP %d", code)
	}
	if detail["item"].(map[string]any)["status"] != "RESOLVED" {
		t.Error("reviewed item not RESOLVED after accept")
	}
	foundAudit := false
	for _, a := range detail["audit"].([]any) {
		am := a.(map[string]any)
		if am["action"] == "ACCEPTED_AGENT_MATCH" && am["actor"] == "human:operator@test" {
			foundAudit = true
		}
	}
	if !foundAudit {
		t.Error("no human review audit event recorded")
	}

	// The gross-mismatch and arithmetic-inconsistency cases must carry evidence
	// that proves why they were escalated.
	_, esc := doJSON(t, ts, "GET", fmt.Sprintf("/api/v1/jobs/%d/items?status=ESCALATED", jobID), nil, "")
	var escItems []map[string]any
	raw, _ = json.Marshal(esc["items"])
	_ = json.Unmarshal(raw, &escItems)
	if len(escItems) != 6 {
		t.Fatalf("ESCALATED items = %d, want 6", len(escItems))
	}
	seen := map[string]bool{}
	for _, it := range escItems {
		_, d := doJSON(t, ts, "GET", fmt.Sprintf("/api/v1/items/%d", int64(it["id"].(float64))), nil, "")
		// Structural escalations (missing reference, no candidate) carry reasons
		// but no evidence; only mismatch-class escalations emit evidence.
		ev, ok := d["evidence"].([]any)
		if !ok {
			continue
		}
		for _, e := range ev {
			seen[e.(map[string]any)["type"].(string)] = true
		}
	}
	for _, wantType := range []string{"AMOUNT_UNEXPLAINED", "ARITHMETIC_INCONSISTENT"} {
		if !seen[wantType] {
			t.Errorf("no escalated item carries %s evidence", wantType)
		}
	}
}
