package handlers

import (
	"net/http"

	"github.com/aksayush2005/raze/services/api/internal/models"
)

var validKinds = map[string]bool{
	"payment": true, "settlement": true, "refund": true,
	"fee": true, "tax": true, "chargeback": true,
}

type importRequest struct {
	Records []models.Record `json:"records"`
}

// handleImportRecords upserts normalized records. Duplicate (source,
// external_id) pairs are updated in place — re-import is idempotent.
func (a *App) handleImportRecords(w http.ResponseWriter, r *http.Request) {
	var req importRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if len(req.Records) == 0 {
		writeError(w, http.StatusBadRequest, "no records supplied")
		return
	}

	imported := 0
	for i := range req.Records {
		rec := &req.Records[i]
		if rec.Kind == "" || !validKinds[rec.Kind] {
			writeError(w, http.StatusBadRequest, "invalid kind: "+rec.Kind)
			return
		}
		if rec.Source == "" || rec.ExternalID == "" {
			writeError(w, http.StatusBadRequest, "source and external_id are required")
			return
		}
		if rec.Currency == "" {
			rec.Currency = "INR"
		}
		// Preserve the provider's declared net so fee/tax discrepancies (where
		// net disagrees with amount-fee-tax) survive import and are caught by
		// the engine's arithmetic check. Only derive net when it was omitted
		// (net == 0 on a non-zero amount).
		if rec.NetMinor == 0 && rec.AmountMinor != 0 {
			rec.NetMinor = rec.AmountMinor - rec.FeeMinor - rec.TaxMinor
		}
		if _, err := a.store.UpsertRecord(r.Context(), rec); err != nil {
			writeError(w, http.StatusInternalServerError, "store record: "+err.Error())
			return
		}
		imported++
	}
	writeJSON(w, http.StatusOK, map[string]any{"imported": imported})
}
