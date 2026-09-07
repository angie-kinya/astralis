package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Env  string
	Port string

	DatabaseURL string
	RedisAddr   string

	BackendJWTSecret string
	CORSOrigin       string

	AnthropicAPIKey string
	ClaudeModel     string

	EmbedProvider string
	OpenAIAPIKey  string
	EmbedModel    string
	EmbedDim      int

	SafetyClassifierURL string
	SafetyThreshold     float64
}

func Load() (*Config, error) {
	_ = godotenv.Load("../../.env", ".env")

	c := &Config{
		Env:                 get("ENV", "development"),
		Port:                get("PORT", "8080"),
		DatabaseURL:         get("DATABASE_URL", ""),
		RedisAddr:           get("REDIS_ADDR", "localhost:6379"),
		BackendJWTSecret:    get("BACKEND_JWT_SECRET", ""),
		CORSOrigin:          get("CORS_ORIGIN", "http://localhost:3000"),
		AnthropicAPIKey:     get("ANTHROPIC_API_KEY", ""),
		ClaudeModel:         get("CLAUDE_MODEL", "claude-sonnet-5"),
		EmbedProvider:       get("EMBED_PROVIDER", "openai"),
		OpenAIAPIKey:        get("OPENAI_API_KEY", ""),
		EmbedModel:          get("EMBED_MODEL", "text-embedding-3-small"),
		SafetyClassifierURL: get("SAFETY_CLASSIFIER_URL", ""),
	}

	dim, err := strconv.Atoi(get("EMBED_DIM", "1536"))
	if err != nil {
		return nil, fmt.Errorf("EMBED_DIM must be an integer: %w", err)
	}
	c.EmbedDim = dim

	th, err := strconv.ParseFloat(get("SAFETY_THRESHOLD", "0.55"), 64)
	if err != nil {
		return nil, fmt.Errorf("SAFETY_THRESHOLD must be a float: %w", err)
	}
	c.SafetyThreshold = th

	for name, v := range map[string]string{
		"DATABASE_URL":       c.DatabaseURL,
		"BACKEND_JWT_SECRET": c.BackendJWTSecret,
		"ANTHROPIC_API_KEY":  c.AnthropicAPIKey,
	} {
		if v == "" {
			return nil, fmt.Errorf("missing required env var %s", name)
		}
	}
	return c, nil
}

func get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
