// Package services orchestrates the reconciliation workflow. It is the only
// place that drives authoritative state transitions.
package services

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/aksayush2005/raze/services/api/internal/engine"
	"github.com/aksayush2005/raze/services/api/internal/models"
	"github.com/aksayush2005/raze/services/api/internal/repositories"
)

// Advice is an advisory recommendation from an investigation layer (AI).
// It is never applied directly; the control plane decides.
type Advice struct {
	Recommendation string         `json:"recommendation"`
	Confidence     float64        `json:"confidence"`
	Investigation  map[string]any `json:"investigation"`
	ModelVersion   string         `json:"model_version"`
}

// Investigator is implemented by the AI service client (or nil when disabled).
type Investigator interface {
	Advise(ctx context.Context, item *models.Item, result *engine.Result) (*Advice, error)
}

// Reconciler drives the async reconciliation workflow.
type Reconciler struct {
	store   *repositories.Store
	engine  *engine.Engine
	ai      Investigator
	workers int
	wg      sync.WaitGroup
}

// NewReconciler builds a Reconciler. ai may be nil (deterministic-only mode).
func NewReconciler(store *repositories.Store, eng *engine.Engine, ai Investigator, workers int) *Reconciler {
	if workers < 1 {
		workers = 1
	}
	return &Reconciler{store: store, engine: eng, ai: ai, workers: workers}
}

// CreateJob starts a reconciliation job over all current records.
func (r *Reconciler) CreateJob(ctx context.Context, name string, config map[string]any) (*models.Job, error) {
	job, err := r.store.CreateJob(ctx, name, config)
	if err != nil {
		return nil, err
	}
	ids, err := r.store.AllRecordIDs(ctx)
	if err != nil {
		return nil, err
	}
	n, err := r.store.CreateItems(ctx, job.ID, ids)
	if err != nil {
		return nil, err
	}
	if err := r.store.SetJobStatus(ctx, job.ID, job.Version, models.StatusRunning); err != nil {
		return nil, err
	}
	if err := r.store.SetJobTotal(ctx, job.ID, int64(n)); err != nil {
		return nil, err
	}
	job.Status = models.StatusRunning
	job.TotalRecords = int64(n)

	if err := r.store.AuditJob(ctx, job.ID, "JOB_CREATED", map[string]any{"name": name, "records": n}); err != nil {
		log.Printf("reconciler: audit job created: %v", err)
	}
	return job, nil
}

// RunWorker consumes items until the context is cancelled. It is intended to
// run as a goroutine for the lifetime of the service.
func (r *Reconciler) RunWorker(ctx context.Context) {
	for i := 0; i < r.workers; i++ {
		r.wg.Add(1)
		go r.workerLoop(ctx)
	}
	r.wg.Wait()
}

func (r *Reconciler) workerLoop(ctx context.Context) {
	defer r.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		item, err := r.store.ClaimItem(ctx)
		if err == repositories.ErrNotFound {
			_ = r.completeFinishedJobs(ctx)
			select {
			case <-ctx.Done():
				return
			case <-time.After(150 * time.Millisecond):
			}
			continue
		}
		if err != nil {
			log.Printf("reconciler: claim: %v", err)
			time.Sleep(250 * time.Millisecond)
			continue
		}
		r.process(ctx, item)
	}
}

// process runs one item through the deterministic engine (and optionally the
// investigation layer) and persists the outcome.
func (r *Reconciler) process(ctx context.Context, item *models.Item) {
	res, err := r.engine.ProcessItem(ctx, item)
	if err != nil {
		r.failItem(ctx, item, err)
		return
	}

	// Advisory AI refinement for non-deterministic outcomes (fails closed: on
	// any AI error the deterministic outcome stands and routes to review).
	if r.ai != nil && (res.Decision == models.DecisionReview || res.Decision == models.DecisionEscalate) {
		advice, aerr := r.ai.Advise(ctx, item, res)
		if aerr == nil {
			r.persistAIDecision(ctx, item, res, advice)
		} else {
			log.Printf("reconciler: ai advise failed (fail-closed): %v", aerr)
			// Deterministic decision already routes to REVIEW/ESCALATE.
		}
	}

	if err := r.persistOutcome(ctx, item, res); err != nil {
		r.failItem(ctx, item, err)
	}
}

func (r *Reconciler) persistOutcome(ctx context.Context, item *models.Item, res *engine.Result) error {
	for _, c := range res.Candidates {
		c.ItemID = item.ID
	}
	for _, e := range res.Evidence {
		e.ItemID = item.ID
	}
	if len(res.Candidates) > 0 {
		if err := r.store.CreateCandidates(ctx, res.Candidates); err != nil {
			return err
		}
	}
	if len(res.Evidence) > 0 {
		if err := r.store.CreateEvidence(ctx, res.Evidence); err != nil {
			return err
		}
	}

	now := time.Now().UTC()
	item.Confidence = &res.Confidence
	item.Decision = &res.Decision
	item.MatchRecordID = res.MatchRecordID

	switch res.Decision {
	case models.DecisionReconciled:
		item.Status = models.StatusResolved
		item.ResolvedAt = &now
		if err := r.store.IncJobStat(ctx, item.JobID, "matched", 1); err != nil {
			log.Printf("reconciler: inc matched: %v", err)
		}
	case models.DecisionReview:
		item.Status = models.StatusReview
		if err := r.store.IncJobStat(ctx, item.JobID, "review", 1); err != nil {
			log.Printf("reconciler: inc review: %v", err)
		}
	case models.DecisionEscalate:
		item.Status = models.StatusEscalated
		item.ResolvedAt = &now
		if err := r.store.IncJobStat(ctx, item.JobID, "escalated", 1); err != nil {
			log.Printf("reconciler: inc escalated: %v", err)
		}
	default:
		return errors.New("reconciler: invalid decision " + res.Decision)
	}

	if err := r.store.UpdateItem(ctx, item); err != nil {
		return err
	}
	_ = r.store.AuditItem(ctx, item.ID, "system", "RECONCILED", map[string]any{
		"decision":   res.Decision,
		"confidence": res.Confidence,
		"match":      res.MatchExternalID,
		"reasons":    res.Reasons,
	})
	return nil
}

func (r *Reconciler) persistAIDecision(ctx context.Context, item *models.Item, res *engine.Result, advice *Advice) {
	d := &models.AIDecision{
		ItemID:         item.ID,
		Recommendation: advice.Recommendation,
		Confidence:     advice.Confidence,
		CandidateRankings: []map[string]any{
			{"target_record_id": res.MatchRecordID, "reason": res.Reasons},
		},
		Investigation: advice.Investigation,
		ModelVersion:  advice.ModelVersion,
	}
	if _, err := r.store.CreateAIDecision(ctx, d); err != nil {
		log.Printf("reconciler: persist ai decision: %v", err)
	}
}

func (r *Reconciler) failItem(ctx context.Context, item *models.Item, cause error) {
	now := time.Now().UTC()
	item.Status = models.StatusFailed
	item.ResolvedAt = &now
	if err := r.store.UpdateItem(ctx, item); err != nil {
		log.Printf("reconciler: fail item update: %v", err)
	}
	_ = r.store.AuditItem(ctx, item.ID, "system", "FAILED", map[string]any{"error": cause.Error()})
	log.Printf("reconciler: item %d failed: %v", item.ID, cause)
}

// completeFinishedJobs marks RUNNING jobs as COMPLETED once no item is active.
func (r *Reconciler) completeFinishedJobs(ctx context.Context) error {
	jobs, err := r.store.ListRunningJobs(ctx)
	if err != nil {
		return err
	}
	for _, j := range jobs {
		active, err := r.store.CountActiveItems(ctx, j.ID)
		if err != nil {
			continue
		}
		if active == 0 {
			if err := r.store.CompleteJob(ctx, j.ID, j.Version); err == nil {
				_ = r.store.AuditJob(ctx, j.ID, "JOB_COMPLETED", map[string]any{
					"matched": j.Matched, "review": j.Review, "escalated": j.Escalated,
				})
			}
		}
	}
	return nil
}
