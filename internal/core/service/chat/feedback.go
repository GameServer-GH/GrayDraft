package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/repository/postgres"
	"github.com/graydraft/core/service/ragops"
)

// FeedbackService applies chat up/down votes to KB quality scores.
type FeedbackService struct {
	repo   *postgres.ChunkRepo
	cfg    *config.Config
	healer *ragops.Healer
}

// NewFeedbackService creates feedback service.
func NewFeedbackService(repo *postgres.ChunkRepo, cfg *config.Config, healer *ragops.Healer) *FeedbackService {
	return &FeedbackService{repo: repo, cfg: cfg, healer: healer}
}

const (
	// VoteTypeQuality is the default vote type (up/down on answer quality).
	VoteTypeQuality = "quality"
	// VoteTypeCoverage indicates knowledge base coverage gap.
	VoteTypeCoverage = "coverage"
)

// ApplyFeedback handles vote up (boost confidence) or down (penalize, may delete).
func (s *FeedbackService) ApplyFeedback(ctx context.Context, req model.ChatFeedbackRequest) (*model.ChatFeedbackResponse, error) {
	// Coverage votes don't alter chunk quality — they feed the coverage gap pipeline
	if req.VoteType == VoteTypeCoverage {
		return s.applyCoverage(ctx, req)
	}

	switch req.Vote {
	case "down":
		return s.applyDown(ctx, req)
	case "up":
		return s.applyUp(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported vote: %s (use up or down)", req.Vote)
	}
}

func (s *FeedbackService) applyCoverage(ctx context.Context, req model.ChatFeedbackRequest) (*model.ChatFeedbackResponse, error) {
	// Coverage gap votes are logged for admin analysis
	if s.healer != nil {
		s.healer.OnDownvote(ctx, req, nil) // reuse healer path for logging
	}
	return &model.ChatFeedbackResponse{
		Vote:     req.Vote,
		Accepted: true,
		Message:  "coverage gap recorded (no quality changes applied)",
	}, nil
}

func (s *FeedbackService) resolveChunkIDs(ctx context.Context, req model.ChatFeedbackRequest) ([]string, error) {
	chunkIDs := append([]string{}, req.ChunkIDs...)
	if len(chunkIDs) == 0 && len(req.DocIDs) > 0 {
		for _, docID := range req.DocIDs {
			ids, err := s.repo.ListChunkIDsByDoc(ctx, req.KBType, docID)
			if err != nil {
				return nil, err
			}
			chunkIDs = append(chunkIDs, ids...)
		}
	}
	return chunkIDs, nil
}

func (s *FeedbackService) applyDown(ctx context.Context, req model.ChatFeedbackRequest) (*model.ChatFeedbackResponse, error) {
	chunkIDs, err := s.resolveChunkIDs(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(chunkIDs) == 0 {
		return &model.ChatFeedbackResponse{
			Vote:       "down",
			Accepted:   false,
			Message:    "no knowledge sources to penalize (answer may be pure LLM without KB hits)",
			MinQuality: s.deleteBelow(),
		}, nil
	}

	penalty := s.cfg.Chat.FeedbackDownvotePenalty
	deleteBelow := s.deleteBelow()
	dbRes, err := s.repo.ApplyDownvote(ctx, req.KBType, chunkIDs, penalty, deleteBelow)
	if err != nil {
		return nil, err
	}
	if s.healer != nil {
		s.healer.OnDownvote(ctx, req, chunkIDs)
	}
	return s.buildResponse("down", dbRes, deleteBelow, func(d postgres.VoteDocResult) string {
		if d.Deleted {
			return fmt.Sprintf("doc %s deleted (quality %.2f)", shortID(d.DocID), d.NewQuality)
		}
		return fmt.Sprintf("doc %s quality %.2f→%.2f", shortID(d.DocID), d.OldQuality, d.NewQuality)
	}), nil
}

func (s *FeedbackService) applyUp(ctx context.Context, req model.ChatFeedbackRequest) (*model.ChatFeedbackResponse, error) {
	chunkIDs, err := s.resolveChunkIDs(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(chunkIDs) == 0 {
		return &model.ChatFeedbackResponse{
			Vote:     "up",
			Accepted: false,
			Message:  "no knowledge sources to boost (answer may be pure LLM without KB hits)",
		}, nil
	}

	boost := s.cfg.Chat.FeedbackUpvoteBoost
	dbRes, err := s.repo.ApplyUpvote(ctx, req.KBType, chunkIDs, boost)
	if err != nil {
		return nil, err
	}
	return s.buildResponse("up", dbRes, 0, func(d postgres.VoteDocResult) string {
		return fmt.Sprintf("doc %s quality %.2f→%.2f", shortID(d.DocID), d.OldQuality, d.NewQuality)
	}), nil
}

func (s *FeedbackService) buildResponse(
	vote string,
	dbRes *postgres.VoteFeedbackResult,
	minQuality float64,
	formatDoc func(postgres.VoteDocResult) string,
) *model.ChatFeedbackResponse {
	resp := &model.ChatFeedbackResponse{
		Vote:          vote,
		Accepted:      true,
		ChunksUpdated: dbRes.ChunksUpdated,
		DeletedDocIDs: dbRes.DeletedDocIDs,
		MinQuality:    minQuality,
	}
	var parts []string
	for _, d := range dbRes.Docs {
		resp.Docs = append(resp.Docs, model.ChatFeedbackDocResult{
			DocID:         d.DocID,
			OldQuality:    d.OldQuality,
			NewQuality:    d.NewQuality,
			Deleted:       d.Deleted,
			ChunksTouched: d.ChunksTouched,
		})
		parts = append(parts, formatDoc(d))
	}
	if len(parts) > 0 {
		resp.Message = strings.Join(parts, "; ")
	} else {
		verb := "penalized"
		if vote == "up" {
			verb = "boosted"
		}
		resp.Message = fmt.Sprintf("%s %d chunk(s)", verb, dbRes.ChunksUpdated)
	}
	return resp
}

func (s *FeedbackService) deleteBelow() float64 {
	if s.cfg != nil && s.cfg.Chat.FeedbackDeleteBelow > 0 {
		return s.cfg.Chat.FeedbackDeleteBelow
	}
	return 0.35
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

// ValidateFeedbackRequest checks vote payload.
func ValidateFeedbackRequest(req model.ChatFeedbackRequest) error {
	if req.Vote != "down" && req.Vote != "up" {
		return fmt.Errorf("vote must be up or down")
	}
	if len(req.ChunkIDs) == 0 && len(req.DocIDs) == 0 {
		return fmt.Errorf("chunk_ids or doc_ids required")
	}
	return nil
}
