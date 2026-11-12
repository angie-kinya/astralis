package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Env  string
	Port string

	DatabaseURL string
	// RedisAddr is optional. Empty means no queue: the API skips enqueueing
	// and cmd/summarise writes episodic memory on a schedule instead.
	RedisAddr string

	// AuthSecret keys the HMAC over sign-in codes. It no longer has to match
	// anything in the frontend — Go owns sign-in end to end, so the
	// shared-secret-mismatch failure mode is gone.
	AuthSecret string

	ResendAPIKey string
	MailFrom     string
	// DevEchoCode returns the sign-in code in the API response so scripts can
	// sign in without an inbox. Refused outright in production.
	DevEchoCode bool

	// TrustedProxies are the addresses allowed to set X-Forwarded-For. Behind
	// Caddy this must be set, or every request appears to come from Caddy and
	// the per-IP sign-in limit throttles everyone as one person.
	TrustedProxies []string

	AnthropicAPIKey string
	ClaudeModel     string

	// AnalystModel reads every message before the reply is written
	// a small model: it runs on every turn and only fills in a fixed JSON shape
	AnalystModel string
	// RespondEffort is passed to the reply model as output_config.effort
	// Empty leaves it out, which every model accepts
	RespondEffort string
	// AgentRepair allows one revision pass on a reply that fails the style
	// lint (AGENT_REPAIR, default true). false = log-only: violations recorded, nothing revised
	AgentRepair bool
	// AgentVariants are the voices the bandit chooses between (AGENT_VARIANTS,
	// default "steady,terse"). one name turns the bandit off
	AgentVariants []string
	// MinSimilarity is the cosine similarity below which a passage is too loosely related to show the model
	MinSimilarity float64
	// AdminEmailHashes are lowercase hex SHA-256 digests of the emails allowed
	// to read /api/v1/admin. hashes rather than addresses, so the list can live in the public compose file without publishing anyone's email
	AdminEmailHashes []string

	EmbedProvider string
	OpenAIAPIKey  string
	EmbedModel    string
	EmbedDim      int

	SafetyClassifierURL string
	SafetyThreshold     float64

	RateLimitPerMin int
	// Sign-in attempts per client IP per minute.
	AuthRateLimitPerMin int
}

func (c *Config) Production() bool { return c.Env == "production" }

// Load reads configuration shared by every binary. Requirements specific to
// the HTTP API live in ValidateAPI, so cmd/summarise does not need mail
// credentials it will never use.
func Load() (*Config, error) {
	_ = godotenv.Load("../../.env", ".env")

	c := &Config{
		Env:                 get("ENV", "development"),
		Port:                get("PORT", "8080"),
		DatabaseURL:         get("DATABASE_URL", ""),
		RedisAddr:           get("REDIS_ADDR", ""),
		AuthSecret:          get("AUTH_SECRET", ""),
		ResendAPIKey:        get("RESEND_API_KEY", ""),
		MailFrom:            get("MAIL_FROM", "Astralis <onboarding@resend.dev>"),
		DevEchoCode:         get("AUTH_DEV_ECHO_CODE", "") == "true",
		AnthropicAPIKey:     get("ANTHROPIC_API_KEY", ""),
		ClaudeModel:         get("CLAUDE_MODEL", "claude-sonnet-5"),
		AnalystModel:        get("ANALYST_MODEL", "claude-haiku-4-5"),
		RespondEffort:       get("RESPOND_EFFORT", ""),
		EmbedProvider:       get("EMBED_PROVIDER", "openai"),
		OpenAIAPIKey:        get("OPENAI_API_KEY", ""),
		EmbedModel:          get("EMBED_MODEL", "text-embedding-3-small"),
		SafetyClassifierURL: get("SAFETY_CLASSIFIER_URL", ""),
		AgentRepair:         get("AGENT_REPAIR", "true") != "false",
	}
	for _, v := range strings.Split(get("AGENT_VARIANTS", "steady,terse"), ",") {
		if v = strings.TrimSpace(v); v != "" {
			c.AgentVariants = append(c.AgentVariants, v)
		}
	}
	for _, p := range strings.Split(get("TRUSTED_PROXIES", "127.0.0.1,::1,172.16.0.0/12,10.0.0.0/8"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			c.TrustedProxies = append(c.TrustedProxies, p)
		}
	}

	for _, h := range strings.Split(get("ADMIN_EMAIL_SHA256", ""), ",") {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			c.AdminEmailHashes = append(c.AdminEmailHashes, h)
		}
	}

	var err error
	if c.EmbedDim, err = strconv.Atoi(get("EMBED_DIM", "1536")); err != nil {
		return nil, fmt.Errorf("EMBED_DIM must be an integer: %w", err)
	}
	if c.SafetyThreshold, err = strconv.ParseFloat(get("SAFETY_THRESHOLD", "0.55"), 64); err != nil {
		return nil, fmt.Errorf("SAFETY_THRESHOLD must be a float: %w", err)
	}
	if c.RateLimitPerMin, err = strconv.Atoi(get("RATE_LIMIT_PER_MIN", "20")); err != nil {
		return nil, fmt.Errorf("RATE_LIMIT_PER_MIN must be an integer: %w", err)
	}
	if c.AuthRateLimitPerMin, err = strconv.Atoi(get("AUTH_RATE_LIMIT_PER_MIN", "10")); err != nil {
		return nil, fmt.Errorf("AUTH_RATE_LIMIT_PER_MIN must be an integer: %w", err)
	}
	if c.MinSimilarity, err = strconv.ParseFloat(get("MIN_SIMILARITY", "0.25"), 64); err != nil {
		return nil, fmt.Errorf("MIN_SIMILARITY must be a float: %w", err)
	}

	for name, v := range map[string]string{
		"DATABASE_URL":      c.DatabaseURL,
		"ANTHROPIC_API_KEY": c.AnthropicAPIKey,
	} {
		if v == "" {
			return nil, fmt.Errorf("missing required env var %s", name)
		}
	}
	return c, nil
}

// ValidateAPI fails fast on anything that would make sign-in unsafe. Every
// check here is a mistake that would otherwise surface as a security problem
// rather than an error message.
func (c *Config) ValidateAPI() error {
	if len(c.AuthSecret) < 32 {
		return errors.New("AUTH_SECRET must be at least 32 characters (openssl rand -base64 48)")
	}
	if c.Production() {
		if c.ResendAPIKey == "" {
			// Without a provider the API falls back to printing codes to the
			// log — fine on a laptop, a live credential leak on a server.
			return errors.New("RESEND_API_KEY is required in production")
		}
		if c.DevEchoCode {
			return errors.New("AUTH_DEV_ECHO_CODE must never be set in production")
		}
	}
	return nil
}

func get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
