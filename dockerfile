FROM golang:1.25.5-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags="-s -w" -o priceTracker

FROM alpine:latest
RUN apk add --no-cache chromium

ENV CHROME_BIN=/usr/bin/chromium-browser

WORKDIR /app
COPY --from=builder /app/priceTracker .

CMD ["./priceTracker"]