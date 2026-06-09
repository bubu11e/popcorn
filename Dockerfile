FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Static binary; templates and static assets are embedded via go:embed.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/popcorn .

# ---

FROM alpine:3.23

# ca-certificates: HTTPS calls to allocine.fr. tzdata + TZ: showtimes are
# Europe/Paris wall-clock times, so the container must resolve that zone.
# hadolint ignore=DL3018
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1000 app

USER app
WORKDIR /app

COPY --from=builder /bin/popcorn /bin/popcorn
COPY config.example.yaml /app/config.yaml

ENV TZ=Europe/Paris

EXPOSE 5000

# Shell form so ${POPCORN_PORT} is honoured when the port is overridden.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- "http://localhost:${POPCORN_PORT:-5000}/health" >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/bin/popcorn"]
