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

	"astralis/gateway/internal/agent"
	"astralis/gateway/internal/api"
	"astralis/gateway/internal/config"
	"astralis/gateway/internal/embed"
	"astralis/gateway/internal/llm"
	"astralis/gateway/internal/mail"
	"astralis/gateway/internal/safety"
	"astralis/gateway/internal/store"

	"github.com/hibiken/asynq"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err == nil {
		err = cfg.ValidateAPI()
	}
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

	// NewHTTPClassifier returns nil when unconfigured; the filter handles that.
	var clf safety.Classifier
	if h := safety.NewHTTPClassifier(cfg.SafetyClassifierURL); h != nil {
		clf = h
	}
	filter := safety.NewFilter(clf, cfg.SafetyThreshold)

	brain := agent.New(db, embedder, claude, filter, agent.Config{
		AnalystModel:  cfg.AnalystModel,
		RespondEffort: cfg.RespondEffort,
		MinSimilarity: cfg.MinSimilarity,
	})

	// The queue is optional. On a single droplet there is no Redis, and
	// cmd/summarise does the same work from cron.
	var queue *asynq.Client
	if cfg.RedisAddr != "" {
		queue = asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisAddr})
		defer queue.Close()
	}

	// Real email in production (ValidateAPI guarantees the key is present);
	// codes printed to this terminal on a laptop.
	var mailer mail.Sender = mail.Log{}
	if cfg.ResendAPIKey != "" {
		mailer = mail.NewResend(cfg.ResendAPIKey, cfg.MailFrom)
	}
	slog.Info("mail", "provider", map[bool]string{true: "resend", false: "log (dev only)"}[cfg.ResendAPIKey != ""])

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.NewServer(cfg, db, brain, filter, queue, mailer).Router(),
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
