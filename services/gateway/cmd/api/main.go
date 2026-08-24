package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"astralis/gateway/internal/api"
	"astralis/gateway/internal/config"
	"astralis/gateway/internal/embed"
	"astralis/gateway/internal/llm"
	"astralis/gateway/internal/rag"
	"astralis/gateway/internal/safety"
	"astralis/gateway/internal/store"

	"github.com/hibiken/asynq"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	db, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	embedder := embed.NewOpenAI(cfg.OpenAIAPIKey, cfg.EmbedModel, cfg.EmbedDim)
	claude := llm.NewAnthropic(cfg.AnthropicAPIKey, cfg.ClaudeModel)
	engine := rag.NewEngine(db, embedder, claude, 5)

	// NewHTTPClassifier returns nil when unconfigured; the filter handles that.
	var clf safety.Classifier
	if h := safety.NewHTTPClassifier(cfg.SafetyClassifierURL); h != nil {
		clf = h
	}
	filter := safety.NewFilter(clf, cfg.SafetyThreshold)

	queue := asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisAddr})
	defer queue.Close()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.NewServer(cfg, db, engine, filter, queue).Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("api listening", "port", cfg.Port, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown: finish in-flight requests instead of dropping them
	// when Render restarts the container.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown", "err", err)
	}
}
