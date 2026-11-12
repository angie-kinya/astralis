// Command summarise writes episodic memory for conversations that have gone
// quiet, then exits: a short summary and the person_facts drawn from it, both
// on the analyst model (agent.Memory).
//
// It is the queue-free counterpart to cmd/worker: same work, same prompt, same
// table, but driven by a scheduled run rather than by an enqueued event. Run
// it from cron — GitHub Actions does the job for nothing — and the whole
// system needs no Redis, which is the difference between a free deployment and
// a paid one.
//
// It is also self-healing in a way the queue is not: a summary missed because
// a browser never fired `pagehide` is simply picked up on the next run.
//
//	go run ./cmd/summarise                 # sessions idle 30m+, up to 25
//	go run ./cmd/summarise -idle 2h -n 50
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"astralis/gateway/internal/agent"
	"astralis/gateway/internal/config"
	"astralis/gateway/internal/embed"
	"astralis/gateway/internal/llm"
	"astralis/gateway/internal/store"
)

func main() {
	idle := flag.Duration("idle", 30*time.Minute, "only summarise sessions quiet for at least this long")
	limit := flag.Int("n", 25, "maximum sessions per run")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	// A generous ceiling: this is a batch job, not a request. But it is a
	// ceiling, so a hung provider cannot leave a scheduled run pinned forever.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	db, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	memory := agent.NewMemory(db,
		llm.NewAnthropic(cfg.AnthropicAPIKey, cfg.ClaudeModel),
		embed.NewOpenAI(cfg.OpenAIAPIKey, cfg.EmbedModel, cfg.EmbedDim),
		cfg.AnalystModel)

	pending, err := db.SessionsNeedingSummary(ctx, *idle, *limit)
	if err != nil {
		slog.Error("find pending", "err", err)
		os.Exit(1)
	}
	slog.Info("pending sessions", "count", len(pending))

	var done, failed int
	for _, p := range pending {
		if err := memory.Consolidate(ctx, p.SessionID, p.UserID); err != nil {
			// One bad session must not abort the run. The next scheduled pass
			// will try it again.
			slog.Error("summarise", "session_id", p.SessionID, "err", err)
			failed++
			continue
		}
		done++
	}
	slog.Info("run complete", "summarised", done, "failed", failed)

	// Housekeeping rides along with the periodic job. Nothing depends on it —
	// every lookup checks expiry — it just keeps the tables small.
	if n, err := db.PurgeExpiredAuth(ctx); err != nil {
		slog.Error("purge expired auth", "err", err)
	} else if n > 0 {
		slog.Info("purged expired auth rows", "count", n)
	}

	// Exit non-zero only if everything failed — that is a real outage worth a
	// red tick in Actions. A single failure among many is not.
	if failed > 0 && done == 0 {
		os.Exit(1)
	}
}
