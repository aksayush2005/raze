package handlers

import (
	"net/http"
	"strconv"

	"github.com/aksayush2005/raze/services/api/internal/repositories"
)

type createJobRequest struct {
	Name   string         `json:"name"`
	Config map[string]any `json:"config"`
}

func (a *App) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req createJobRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if req.Name == "" {
		req.Name = "reconciliation"
	}
	job, err := a.recon.CreateJob(r.Context(), req.Name, req.Config)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create job: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

func (a *App) handleListJobs(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	jobs, err := a.store.ListJobs(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list jobs: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func (a *App) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	job, err := a.store.GetJob(r.Context(), id)
	if mapStoreErr(w, err, "job not found") {
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (a *App) handleListJobItems(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	if _, err := a.store.GetJob(r.Context(), id); mapStoreErr(w, err, "job not found") {
		return
	}
	limit, offset := pagination(r)
	status := r.URL.Query().Get("status")
	items, err := a.store.ListItemsDetailed(r.Context(), id, status, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list items: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *App) handleListRecords(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	kind := r.URL.Query().Get("kind")
	records, err := a.store.ListRecords(r.Context(), kind, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list records: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"records": records})
}

func pagination(r *http.Request) (limit, offset int) {
	limit = 100
	offset = 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}

var _ = repositories.ErrNotFound
