// Package engine implements the deterministic reconciliation engine.
// It establishes financial correctness; it never uses floating point for money
// and never lets AI confidence override arithmetic.
package engine

import (
	"context"
	"fmt"

	"github.com/aksayush2005/raze/services/api/internal/models"
	"github.com/aksayush2005/raze/services/api/internal/repositories"
)

// Result is the deterministic outcome of reconciling one item.
type Result struct {
	Decision        string
	Confidence      float64
	MatchRecordID   *int64
	MatchExternalID string
	Candidates      []*models.Candidate
	Evidence        []*models.Evidence
	Reasons         []string
	Investigation   map[string]any

	// unexported scoring state
	score  float64
	strong int
}

// Engine performs deterministic reconciliation.
type Engine struct {
	store *repositories.Store
}

// New builds an Engine backed by the store.
func New(store *repositories.Store) *Engine { return &Engine{store: store} }

// ProcessItem reconciles a single workflow item deterministically.
func (e *Engine) ProcessItem(ctx context.Context, item *models.Item) (*Result, error) {
	rec, err := e.store.GetRecord(ctx, item.RecordID)
	if err != nil {
		return nil, fmt.Errorf("load record: %w", err)
	}
	res := &Result{Investigation: map[string]any{}, score: 0.10}

	switch rec.Kind {
	case "settlement":
		e.reconcileSettlement(ctx, rec, res)
	case "payment":
		e.reconcilePayment(ctx, rec, res)
	default:
		res.Decision = models.DecisionEscalate
		addReason(res, "UNSUPPORTED_RECORD_KIND:"+rec.Kind)
	}
	return res, nil
}

// --- settlement item ------------------------------------------------------------

func (e *Engine) reconcileSettlement(ctx context.Context, s *models.Record, res *Result) {
	payments, err := e.store.PaymentsForSettlement(ctx, s.ExternalID)
	if err != nil {
		res.Decision = models.DecisionEscalate
		addReason(res, "ENGINE_ERROR")
		return
	}
	if len(payments) == 0 {
		e.fuzzySingle(ctx, s, res)
		return
	}

	for _, p := range payments {
		addCandidate(res, p, "exact_reference", 1.0)
	}
	addEvidence(res, "EXACT_REFERENCE_MATCH", 0.35, map[string]any{
		"batch":    true,
		"payments": externalIDs(payments),
	})
	e.verifyAggregate(ctx, s, payments, res)
	res.Confidence = clamp01(res.score)
	applyPolicy(res)
}

func (e *Engine) reconcilePayment(ctx context.Context, p *models.Record, res *Result) {
	if p.RefExternalID != nil && *p.RefExternalID != "" {
		s, err := e.store.SettlementByExternalID(ctx, *p.RefExternalID)
		if err != nil {
			res.Decision = models.DecisionEscalate
			addReason(res, "MISSING_SETTLEMENT:"+*p.RefExternalID)
			res.Investigation["note"] = "Payment references a settlement that does not exist in the records."
			return
		}
		addCandidate(res, s, "exact_reference", 1.0)
		addEvidence(res, "EXACT_REFERENCE_MATCH", 0.35, map[string]any{"batch": false, "settlement": s.ExternalID})

		batch, err := e.store.PaymentsForSettlement(ctx, s.ExternalID)
		if err != nil {
			res.Decision = models.DecisionEscalate
			addReason(res, "ENGINE_ERROR")
			return
		}
		e.verifyAggregate(ctx, s, batch, res)
		res.Confidence = clamp01(res.score)
		applyPolicy(res)
		return
	}

	e.fuzzySingle(ctx, p, res)
}

// fuzzySingle reconciles a record with no explicit reference by matching
// against amount-proximate opposite-kind records.
func (e *Engine) fuzzySingle(ctx context.Context, rec *models.Record, res *Result) {
	fuzzy, err := e.store.FuzzyCandidates(ctx, rec, amountTol(rec.AmountMinor), 7*24, 10)
	if err != nil || len(fuzzy) == 0 {
		res.Decision = models.DecisionEscalate
		addReason(res, "NO_CANDIDATE_FOUND")
		res.Investigation["note"] = "No referenced record and no amount-proximate candidate found."
		return
	}
	best := fuzzy[0]
	for _, f := range fuzzy[1:] {
		if abs64(f.AmountMinor-rec.AmountMinor) < abs64(best.AmountMinor-rec.AmountMinor) {
			best = f
		}
	}
	addCandidate(res, best, "amount_similarity", 1.0)
	// Fuzzy (no explicit reference): the amount-similarity hit is evidence but
	// the match still requires human confirmation — it never auto-reconciles.
	addEvidence(res, "AMOUNT_SIMILARITY_MATCH", 0.20, map[string]any{
		"candidate": best.ExternalID, "amount": best.AmountMinor,
	})
	if rec.Kind == "settlement" {
		e.verifyAggregate(ctx, rec, []*models.Record{best}, res)
	} else {
		e.verifyAggregate(ctx, best, []*models.Record{rec}, res)
	}
	res.Confidence = clamp01(res.score)
	applyPolicy(res)
}

// --- verification ----------------------------------------------------------------

// verifyAggregate verifies a settlement against its payment set, emitting
// structured evidence and adjusting res.score. Deterministic checks:
//  1. gross equals the sum of payment amounts
//  2. the settlement's own arithmetic is valid: net == amount - fee - tax
//
// Fees/taxes are charged on the settlement, so internal arithmetic consistency
// is the check that catches fee and tax discrepancies.
func (e *Engine) verifyAggregate(ctx context.Context, s *models.Record, payments []*models.Record, res *Result) {
	var gross int64
	for _, p := range payments {
		gross += p.AmountMinor
	}
	expectedNet := s.AmountMinor - s.FeeMinor - s.TaxMinor
	grossDelta := abs64(s.AmountMinor - gross)
	netDelta := abs64(s.NetMinor - expectedNet)

	if grossDelta == 0 {
		addEvidence(res, "AMOUNT_EXPLAINED", 0.25, map[string]any{"payments_sum": gross, "settlement": s.AmountMinor})
	} else {
		addReason(res, fmt.Sprintf("GROSS_DELTA:%d", grossDelta))
		addEvidence(res, "AMOUNT_UNEXPLAINED", -0.25, map[string]any{"payments_sum": gross, "settlement": s.AmountMinor, "delta": grossDelta})
	}

	if netDelta == 0 {
		addEvidence(res, "SETTLEMENT_ARITHMETIC_VALID", 0.25, map[string]any{
			"amount": s.AmountMinor, "fee": s.FeeMinor, "tax": s.TaxMinor, "net": s.NetMinor,
		})
	} else {
		addReason(res, fmt.Sprintf("ARITHMETIC_DELTA:%d", netDelta))
		addEvidence(res, "ARITHMETIC_INCONSISTENT", -0.25, map[string]any{
			"expected_net": expectedNet, "declared_net": s.NetMinor, "delta": netDelta,
		})
	}

	if grossDelta == 0 && netDelta == 0 {
		res.MatchExternalID = s.ExternalID
		id := s.ID
		res.MatchRecordID = &id
		res.strong = 1
		if len(payments) > 1 {
			addEvidence(res, "BATCH_EXPLAINED", 0.0, map[string]any{"payments": len(payments)})
		}
	}
}

// --- policy ------------------------------------------------------------------------

// applyPolicy maps deterministic confidence to a decision. The ambiguity guard
// (>=2 equally strong matches) forces REVIEW; strong arithmetic can still hit
// the AUTO threshold, but only after every deterministic check has passed.
func applyPolicy(res *Result) {
	if res.strong >= 2 {
		res.Decision = models.DecisionReview
		addReason(res, "AMBIGUOUS_CANDIDATES")
		res.Confidence = minf(res.Confidence, 0.70)
		return
	}
	switch {
	case res.Confidence >= 0.95:
		res.Decision = models.DecisionReconciled
	case res.Confidence >= 0.75:
		res.Decision = models.DecisionReview
	default:
		res.Decision = models.DecisionEscalate
	}
}

// --- helpers ------------------------------------------------------------------------

func addCandidate(res *Result, rec *models.Record, strategy string, similarity float64) {
	res.Candidates = append(res.Candidates, &models.Candidate{
		TargetRecordID: rec.ID, Strategy: strategy, Similarity: similarity, Score: similarity,
	})
}

func addEvidence(res *Result, typ string, weight float64, details map[string]any) {
	// The evidence weight IS the score contribution: res.score starts at the
	// 0.10 base and every evidence adds to or subtracts from it. Centralizing
	// the arithmetic here (rather than at each call site) is what guarantees a
	// fully-verified exact match reaches the auto-reconcile threshold and a
	// deterministic mismatch can never be overridden by confidence.
	res.score += weight
	res.Evidence = append(res.Evidence, &models.Evidence{Type: typ, Weight: weight, Details: details})
}

func addReason(res *Result, reason string) { res.Reasons = append(res.Reasons, reason) }

func externalIDs(rs []*models.Record) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.ExternalID)
	}
	return out
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 0.99 {
		return 0.99
	}
	return v
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// amountTol is the fuzzy amount tolerance (1% of the amount, minimum 1 minor unit).
func amountTol(amountMinor int64) int64 {
	t := amountMinor / 100
	if t < 1 {
		return 1
	}
	return t
}
