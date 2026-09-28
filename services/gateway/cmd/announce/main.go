// Command announce emails release notes to the people who asked for them —
// and only them. It is the single place in Astralis that reads email
// addresses in bulk, and it does nothing else with them.
//
//	go run ./cmd/announce -subject "Astralis 0.2" -body notes.txt           # dry run
//	go run ./cmd/announce -subject "Astralis 0.2" -body notes.txt -send     # for real
//
// Dry run is the default because an email cannot be unsent.
package main

import (
	"context"
	"flag"
	"fmt"
	"html"
	"log/slog"
	"os"
	"strings"
	"time"

	"astralis/gateway/internal/auth"
	"astralis/gateway/internal/config"
	"astralis/gateway/internal/mail"
	"astralis/gateway/internal/store"
)

func main() {
	subject := flag.String("subject", "", "email subject, e.g. \"Astralis 0.2 is out\"")
	bodyPath := flag.String("body", "", "path to a plain-text file with the release notes")
	send := flag.Bool("send", false, "actually send; without this it only prints who would receive it")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if *subject == "" || *bodyPath == "" {
		fmt.Fprintln(os.Stderr, "usage: announce -subject ... -body notes.txt [-send]")
		os.Exit(2)
	}
	raw, err := os.ReadFile(*bodyPath)
	if err != nil {
		slog.Error("read body", "err", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	db, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	to, err := db.ReleaseSubscribers(ctx)
	if err != nil {
		slog.Error("subscribers", "err", err)
		os.Exit(1)
	}

	footer := "\n\n—\nYou are getting this because you asked for release notes when you signed up.\n" +
		"Turn them off any time: Astralis → You → Release notes.\n" +
		"Your address is never shown to other users, shared, or used for anything else."
	text := strings.TrimSpace(string(raw)) + footer
	htmlBody := "<pre style=\"font-family:ui-monospace,Menlo,monospace;white-space:pre-wrap\">" +
		html.EscapeString(text) + "</pre>"

	if !*send {
		slog.Info("DRY RUN — nothing sent", "recipients", len(to), "subject", *subject)
		for _, e := range to {
			fmt.Println("  would send to", auth.MaskEmail(e))
		}
		return
	}
	if cfg.ResendAPIKey == "" {
		slog.Error("RESEND_API_KEY is not set; refusing to 'send' to a log")
		os.Exit(1)
	}
	sender := mail.NewResend(cfg.ResendAPIKey, cfg.MailFrom)

	// One message per person, never a shared To/CC line: putting every
	// subscriber on one email would hand each of them everyone else's address.
	var sent, failed int
	for _, e := range to {
		if err := sender.Send(ctx, mail.Message{To: e, Subject: *subject, Text: text, HTML: htmlBody}); err != nil {
			slog.Error("send", "to", auth.MaskEmail(e), "err", err)
			failed++
			continue
		}
		sent++
		// Stay well under the provider's per-second limit.
		time.Sleep(600 * time.Millisecond)
	}
	slog.Info("announce complete", "sent", sent, "failed", failed)
}
