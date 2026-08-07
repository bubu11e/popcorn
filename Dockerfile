FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Commit is supplied at build time (the build context has no .git, so the Go
# toolchain cannot embed vcs.revision itself). CI passes the Woodpecker built-in
# CI_COMMIT_SHA via build_args_from_env; a manual
# `docker build --build-arg COMMIT=$(git rev-parse HEAD)` works too.
ARG CI_COMMIT_SHA=""
ARG COMMIT="${CI_COMMIT_SHA}"

# Static binary; templates and static assets are embedded via go:embed.
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X github.com/bubu11e/popcorn/version.Commit=${COMMIT}" \
    -o /bin/popcorn .

# ---

FROM alpine:3.24

# ca-certificates: HTTPS calls to allocine.fr. tzdata + TZ: showtimes are
# Europe/Paris wall-clock times, so the container must resolve that zone.
# hadolint ignore=DL3018
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1000 app && \
    mkdir -p /app/data && chown app:app /app/data

USER app
WORKDIR /app

COPY --from=builder /bin/popcorn /bin/popcorn
COPY config.example.yaml /app/config.yaml

# Europe/Paris: showtimes are local wall-clock times.
# /app/data holds push subscriptions (when push is enabled); mount a volume
# there to persist them across container restarts.
ENV TZ=Europe/Paris \
    POPCORN_PUSH_SUBSCRIPTIONS_FILE=/app/data/subscriptions.json

VOLUME ["/app/data"]

EXPOSE 5000

# Shell form so ${POPCORN_PORT} is honoured when the port is overridden.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- "http://localhost:${POPCORN_PORT:-5000}/health" >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/bin/popcorn"]
