package handlers

import (
	"net/http"

	"github.com/aksayush2005/raze/services/api/internal/middleware"
	"github.com/aksayush2005/raze/services/api/internal/razorpay"
	"github.com/aksayush2005/raze/services/api/internal/repositories"
	"github.com/aksayush2005/raze/services/api/internal/services"
	"github.com/aksayush2005/raze/services/api/internal/web"
)

// App bundles the service dependencies behind the HTTP API.
type App struct {
	store    *repositories.Store
	recon    *services.Reconciler
	review   *services.ReviewService
	razorpay *razorpay.Client
}

// NewApp wires an App. rp may be nil (Razorpay disabled).
func NewApp(store *repositories.Store, recon *services.Reconciler, review *services.ReviewService, rp *razorpay.Client) *App {
	return &App{store: store, recon: recon, review: review, razorpay: rp}
}

// Routes returns the HTTP handler tree.
func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	mux.HandleFunc("POST /api/v1/records/import", middleware.Idempotent(a.store, a.handleImportRecords))
	mux.HandleFunc("POST /api/v1/records/sync/razorpay", middleware.Idempotent(a.store, a.handleSyncRazorpay))

	mux.HandleFunc("POST /api/v1/jobs", middleware.Idempotent(a.store, a.handleCreateJob))
	mux.HandleFunc("GET /api/v1/jobs", a.handleListJobs)
	mux.HandleFunc("GET /api/v1/jobs/{id}", a.handleGetJob)
	mux.HandleFunc("GET /api/v1/jobs/{id}/items", a.handleListJobItems)

	mux.HandleFunc("GET /api/v1/records", a.handleListRecords)

	mux.HandleFunc("GET /api/v1/items/{id}", a.handleGetItem)
	mux.HandleFunc("POST /api/v1/items/{id}/review", middleware.Idempotent(a.store, a.handleReviewItem))

	mux.Handle("/", web.Handler())

	return withCommon(mux)
}

// withCommon applies logging and recovery middleware.
func withCommon(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				writeError(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
