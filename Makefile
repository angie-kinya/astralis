# Podman on Fedora; override for Docker with:  make up COMPOSE=docker-compose
COMPOSE ?= podman-compose
GATEWAY  = services/gateway

.PHONY: help up down logs migrate psql api worker research classifier web \
        test test-go test-py lint fmt tidy check clean

help:  ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

# ------------------------------------------------------------ infrastructure
up:         ## Start Postgres + Redis
	$(COMPOSE) up -d
down:       ## Stop infrastructure (data survives)
	$(COMPOSE) down
logs:       ## Tail infrastructure logs
	$(COMPOSE) logs -f
migrate:    ## Apply every SQL migration in order
	@for f in db/migrations/*.sql; do \
	  echo "applying $$f"; \
	  $(COMPOSE) exec -T postgres psql -U astralis -d astralis -v ON_ERROR_STOP=1 < $$f || exit 1; \
	done
psql:       ## Open a psql shell
	$(COMPOSE) exec postgres psql -U astralis -d astralis

# ------------------------------------------------------------ services
api:        ## Run the Go API on :8080
	cd $(GATEWAY) && go run ./cmd/api
worker:     ## Run the Go background worker
	cd $(GATEWAY) && go run ./cmd/worker
classifier: ## Serve the safety classifier on :8000
	cd services/safety-classifier && uvicorn app:app --port 8000 --reload
research:   ## Run one research ingestion pass over the standing queries
	cd services/research && python main.py --all
web:        ## Run the Next.js frontend on :3000
	cd web && npm run dev

# ------------------------------------------------------------ quality
test: test-go test-py   ## Run every test suite
test-go:    ## Go tests with the race detector
	cd $(GATEWAY) && go test ./... -race -cover
test-py:    ## Safety classifier behavioural tests
	cd services/safety-classifier && pytest -q
lint:       ## Vet + gofmt check (same rules as CI)
	cd $(GATEWAY) && go vet ./... && test -z "$$(gofmt -l .)" || (gofmt -l $(GATEWAY); exit 1)
fmt:        ## Format Go code in place
	cd $(GATEWAY) && gofmt -w .
tidy:       ## Sync go.mod/go.sum with actual imports
	cd $(GATEWAY) && go mod tidy
check: lint test  ## What CI runs. Run it before you push.

clean:      ## Remove build artefacts and the trained model
	rm -rf $(GATEWAY)/bin services/safety-classifier/models web/.next
