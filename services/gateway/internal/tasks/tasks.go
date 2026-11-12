package tasks

import (
	"context"
	"encoding/json"
	"log/slog"

	"astralis/gateway/internal/agent"

	"github.com/hibiken/asynq"
)

const TypeSummariseSession = "session:summarise"

type SummarisePayload struct {
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
}

func NewSummariseTask(sessionID, userID string) (*asynq.Task, error) {
	b, err := json.Marshal(SummarisePayload{SessionID: sessionID, UserID: userID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeSummariseSession, b, asynq.MaxRetry(3)), nil
}

// same consolidation as cmd/summarise (summary + person_facts), so the queue
// path and the cron path can never drift apart
func SummariseHandler(memory *agent.Memory) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p SummarisePayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return asynq.SkipRetry
		}
		if err := memory.Consolidate(ctx, p.SessionID, p.UserID); err != nil {
			return err
		}

		slog.Info("session summarised", "session_id", p.SessionID)
		return nil
	}
}
