package services

import (
	"context"
	"errors"
	"time"

	"github.com/aksayush2005/raze/services/api/internal/models"
	"github.com/aksayush2005/raze/services/api/internal/repositories"
)

// Review service errors.
var (
	ErrInvalidState  = errors.New("item is not in a reviewable state")
	ErrInvalidAction = errors.New("unknown review action")
	ErrMissingTarget = errors.New("target_record_id required for manual_link")
)

// ReviewService applies operator decisions to workflow items. Every action is
// recorded as structured feedback for learning.
type ReviewService struct {
	store *repositories.Store
}

// NewReviewService builds a ReviewService.
func NewReviewService(store *repositories.Store) *ReviewService {
	return &ReviewService{store: store}
}

// Action identifiers (matching the feedback taxonomy in the spec).
const (
	ActionAccept           = "ACCEPTED_AGENT_MATCH"
	ActionReject           = "REJECTED_CANDIDATE"
	ActionManualLink       = "MANUALLY_LINKED_RECORDS"
	ActionConfirmException = "CONFIRMED_EXCEPTION"
	ActionEscalate         = "ESCALATED"
)

// Apply executes a review action on an item.
func (s *ReviewService) Apply(ctx context.Context, itemID int64, action, actor, note string, manualTargetID *int64) (*models.Item, error) {
	item, err := s.store.GetItem(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if item.Status != models.StatusReview && item.Status != models.StatusEscalated {
		return nil, ErrInvalidState
	}

	now := time.Now().UTC()
	reconciled := models.DecisionReconciled
	escalate := models.DecisionEscalate

	switch action {
	case ActionAccept:
		item.Status = models.StatusResolved
		item.Decision = &reconciled
		item.ResolvedAt = &now
	case ActionReject:
		item.Status = models.StatusEscalated
		item.Decision = &escalate
		item.ResolvedAt = &now
	case ActionManualLink:
		if manualTargetID == nil {
			return nil, ErrMissingTarget
		}
		item.Status = models.StatusResolved
		item.Decision = &reconciled
		item.MatchRecordID = manualTargetID
		item.ResolvedAt = &now
	case ActionConfirmException, ActionEscalate:
		item.Status = models.StatusEscalated
		item.Decision = &escalate
		item.ResolvedAt = &now
	default:
		return nil, ErrInvalidAction
	}

	reviewAction := &models.HumanReviewAction{
		ItemID:   item.ID,
		Action:   action,
		Actor:    actor,
		Note:     note,
		Evidence: map[string]any{"decision": item.Decision, "match_record_id": item.MatchRecordID},
	}
	if err := s.store.ApplyReview(ctx, reviewAction, item); err != nil {
		return nil, err
	}
	return item, nil
}
