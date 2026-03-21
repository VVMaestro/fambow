FROM golang:1.24-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/fambow ./cmd/bot

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

RUN useradd --system --create-home --home-dir /app --shell /usr/sbin/nologin fambow \
    && mkdir -p /data

COPY --from=builder /out/fambow /app/fambow
COPY migrations /app/migrations

RUN chown -R fambow:fambow /app /data

USER fambow

ENV DATABASE_PATH=/data/fambow.db
ENV MIGRATIONS_DIR=/app/migrations

ENTRYPOINT ["/app/fambow"]
