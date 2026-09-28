// Command worker consumes the Asynq queue. Separate binary, separate process,
// separate scaling knob — the API can stay small while background work grows.
package main

import (
	"context"
	"log/slog"
	"os"

	"astralis/gateway/internal/config"
	"astralis/gateway/internal/embed"
	"astralis/gateway/internal/llm"
	"astralis/gateway/internal/rag"
	"astralis/gateway/internal/store"
	"astralis/gateway/internal/tasks"

	"github.com/hibiken/asynq"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	if cfg.RedisAddr == "" {
		slog.Error("config", "err", "REDIS_ADDR is required for the queue worker; use cmd/summarise without Redis")
		os.Exit(1)
	}

	db, err := store.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	engine := rag.NewEngine(db,
		embed.NewOpenAI(cfg.OpenAIAPIKey, cfg.EmbedModel, cfg.EmbedDim),
		llm.NewAnthropic(cfg.AnthropicAPIKey, cfg.ClaudeModel), 5)

	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TypeSummariseSession, tasks.SummariseHandler(db, engine))

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr},
		asynq.Config{Concurrency: 5},
	)

	slog.Info("worker started")
	if err := srv.Run(mux); err != nil {
		slog.Error("worker", "err", err)
		os.Exit(1)
	}
}
