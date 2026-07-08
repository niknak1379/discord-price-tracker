FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o priceTracker

FROM debian:bookworm-slim

USER root
RUN apt-get update && \
  apt-get install -y --no-install-recommends \
    chromium \
    ca-certificates \
    fonts-liberation \
    libnss3 \
    libxss1 \
    libasound2 \
    libatk-bridge2.0-0 \
    libgtk-3-0 \
    wget && \
  rm -rf /var/lib/apt/lists/*
ENV TZ=America/Los_Angeles
WORKDIR /app
COPY --from=builder /app/priceTracker .

ENTRYPOINT []
CMD ["./priceTracker"]
