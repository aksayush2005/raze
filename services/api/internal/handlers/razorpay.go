package handlers

import (
	"net/http"

	"github.com/aksayush2005/raze/services/api/internal/models"
)

// handleSyncRazorpay imports settlements from Razorpay TEST MODE. When the
// provider is not configured, the system clearly reports that it is running on
// synthetic data instead (the demo never requires real credentials).
func (a *App) handleSyncRazorpay(w http.ResponseWriter, r *http.Request) {
	if a.razorpay == nil {
		writeError(w, http.StatusServiceUnavailable,
			"Razorpay is not configured (set RAZORPAY_KEY_ID/SECRET with RAZORPAY_MODE=test). "+
				"The demo runs on synthetic settlement data.")
		return
	}
	records, err := a.razorpay.FetchSettlements(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway,
			"Razorpay Test Mode fetch failed: "+err.Error()+". No records were imported.")
		return
	}
	imported := 0
	for _, rec := range records {
		if rec.Currency == "" {
			rec.Currency = "INR"
		}
		rec.NetMinor = rec.AmountMinor - rec.FeeMinor - rec.TaxMinor
		if _, err := a.store.UpsertRecord(r.Context(), rec); err != nil {
			writeError(w, http.StatusInternalServerError, "store record: "+err.Error())
			return
		}
		imported++
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"imported":  imported,
		"source":    "razorpay_test_mode",
		"synthetic": false,
		"note":      "Razorpay TEST MODE data. Test-mode settlement behavior is not identical to production.",
	})
}

var _ = models.Record{}
