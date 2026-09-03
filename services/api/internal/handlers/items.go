package handlers

import (
	"errors"
	"net/http"

	"github.com/aksayush2005/raze/services/api/internal/models"
	"github.com/aksayush2005/raze/services/api/internal/services"
)

// itemDetail is the full workspace view for one reconciliation case.
type itemDetail struct {
	Item        *models.Item         `json:"item"`
	Record      *models.Record       `json:"record"`
	Match       *models.Record       `json:"match_record,omitempty"`
	Candidates  []*models.Candidate  `json:"candidates"`
	Evidence    []*models.Evidence   `json:"evidence"`
	Audit       []*models.AuditEvent `json:"audit"`
	AIDecisions []*models.AIDecision `json:"ai_decisions"`
}

func (a *App) handleGetItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid item id")
		return
	}
	ctx := r.Context()
	item, err := a.store.GetItem(ctx, id)
	if mapStoreErr(w, err, "item not found") {
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	d := itemDetail{Item: item}
	if d.Record, err = a.store.GetRecord(ctx, item.RecordID); err != nil {
		writeError(w, http.StatusInternalServerError, "load record: "+err.Error())
		return
	}
	if item.MatchRecordID != nil {
		d.Match, _ = a.store.GetRecord(ctx, *item.MatchRecordID)
	}
	if d.Candidates, err = a.store.ListCandidatesByItem(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "load candidates: "+err.Error())
		return
	}
	if d.Evidence, err = a.store.ListEvidenceByItem(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "load evidence: "+err.Error())
		return
	}
	if d.Audit, err = a.store.ListAuditByItem(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "load audit: "+err.Error())
		return
	}
	if d.AIDecisions, err = a.store.ListAIDecisionsByItem(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "load ai decisions: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, d)
}

type reviewRequest struct {
	Action         string `json:"action"`
	Actor          string `json:"actor"`
	Note           string `json:"note"`
	TargetRecordID *int64 `json:"target_record_id"`
}

func (a *App) handleReviewItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid item id")
		return
	}
	var req reviewRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if req.Actor == "" {
		req.Actor = "operator"
	}

	item, err := a.review.Apply(r.Context(), id, req.Action, req.Actor, req.Note, req.TargetRecordID)
	switch {
	case errors.Is(err, services.ErrInvalidState), errors.Is(err, services.ErrInvalidAction):
		writeError(w, http.StatusBadRequest, err.Error())
		return
	case errors.Is(err, services.ErrMissingTarget):
		writeError(w, http.StatusBadRequest, err.Error())
		return
	case mapStoreErr(w, err, "item not found"):
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}
