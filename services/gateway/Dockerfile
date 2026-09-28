# Multi-stage build. Compile with the full Go toolchain, ship a small image
# with four static binaries and nothing else of note.
FROM docker.io/library/golang:1.26-alpine AS build
WORKDIR /src

# Manifests first: this layer only rebuilds when dependencies change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Every entrypoint from one image, so the API and the jobs that share its
# schema assumptions are always the same commit.
#   api        the HTTP service
#   summarise  episodic memory, run from cron (no Redis needed)
#   announce   release notes to people who opted in
#   worker     the Redis/Asynq alternative to summarise, for later
RUN for cmd in api summarise announce worker; do \
      CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/$cmd ./cmd/$cmd || exit 1; \
    done

FROM docker.io/library/alpine:3.20
# ca-certificates: outbound TLS to Anthropic, OpenAI, Resend.
# wget comes with busybox and serves the container healthcheck.
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 astralis
COPY --from=build /out/ /app/
USER astralis
EXPOSE 8080
ENTRYPOINT ["/app/api"]
