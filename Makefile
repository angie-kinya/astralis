.PHONY: help up down migrate api worker research classifier web test lint

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

up:         ## Start Postgres + Redis
	docker compose up -d
down:       ## Stop infrastructure
	docker compose down
migrate:    ## Apply SQL migrations
	docker compose exec -T postgres psql -U astralis -d astralis < db/migrations/0001_init.sql
api:        ## Run Go API
	cd services/gateway && go run ./cmd/api
worker:     ## Run Go background worker
	cd services/gateway && go run ./cmd/worker
research:   ## Run one research ingestion pass
	cd services/research && python main.py --query "occupational burnout banking employees" --limit 25
classifier: ## Serve safety classifier
	cd services/safety-classifier && uvicorn app:app --port 8000 --reload
web:        ## Run Next.js frontend
	cd web && npm run dev
test:       ## Run all tests
	cd services/gateway && go test ./... -race -cover
	cd services/safety-classifier && pytest -q
lint:
	cd services/gateway && go vet ./... && gofmt -l .
