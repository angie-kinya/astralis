package tasks

import (
	"context"
	"encoding/json"
	"log/slog"

	"astralis/gateway/internal/rag"
	"astralis/gateway/internal/store"

	"github.com/hibiken/async"
	"github.com/hibiken/asynq"
)

const TypeSummariseSession = "session:summarise"

type SummarisePayload struct {
	SessionID string `json:"session_id"`
	UserID string `json:"user_id"`
}

func NewSummariseTask(sessionID, userID string) (*asynq.Task, error) {
	b, err := json.Marshal(SummarisePayload{SessionID: sessionID, UserID: :userID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeSummariseSession, b, asynq.MaxRetry(3)), nil
}

func SummariseHandler(s *store.Store, engine *rag.Engine) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p SummarisePayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return asynq.SkipRetry
		}

		msgs, err := s.RecentMessages(ctx, p.SessionID, 50)
		if err != nil {
			return err
		}
		if len(msgs) < 4 {
			return nil
		}

		summary, err := engine.Summarise(ctx, msgs)
		if err != nil {
			return err
		}
		if err := s.SaveSummary(ctx, p.SessionID, p.UserID, summary); err != nil {
			return err
		}

		slog.Info("session summarised", "session_id", p.SessionID)
		return nil
	}
}
