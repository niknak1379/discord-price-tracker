FROM golang:1.25.5-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o priceTracker

FROM chromedp/headless-shell:latest
WORKDIR /app
COPY --from=builder /app/priceTracker .

ENV CHROME_LOG_FILE=/dev/null

CMD ["sh", "-c", "./priceTracker 2>&1 | grep -v 'DevTools\\|WARNING:sandbox'"]