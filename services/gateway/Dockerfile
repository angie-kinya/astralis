# Multi-stage build. Compile with the full Go toolchain, ship a ~20MB image
# containing two static binaries and nothing else -- no shell for an attacker
# to land in, no package manager, no CVE surface from a distro base.
FROM golang:1.26-alpine AS build
WORKDIR /src

# Manifests first: this layer only rebuilds when dependencies change, so day-to-day
# code edits reuse the cached `go mod download`.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Both entrypoints from one image. The deploy target picks which to run, so the
# API and the worker are guaranteed to be the same commit.
#   -s -w strips the symbol table and DWARF info (smaller binary, no debugger).
#   CGO_ENABLED=0 makes it fully static so it runs on a scratch-like base.
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/api    ./cmd/api && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 astralis
COPY --from=build /out/api    /app/api
COPY --from=build /out/worker /app/worker
USER astralis
EXPOSE 8080
ENTRYPOINT ["/app/api"]
