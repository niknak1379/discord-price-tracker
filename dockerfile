FROM golang:1.25.5-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o priceTracker

FROM chromedp/headless-shell:latest

# Install CA certificates
USER root
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
ENV TZ=America/Los_Angeles
WORKDIR /app
COPY --from=builder /app/priceTracker .

ENTRYPOINT []
CMD ["./priceTracker"]
