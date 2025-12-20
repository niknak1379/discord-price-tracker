# Build stage
FROM golang:1.25.5-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags="-s -w" -o priceTracker

FROM chromedp/headless-shell:latest

WORKDIR /app
COPY --from=builder /app/priceTracker .
CMD ["./priceTracker"]