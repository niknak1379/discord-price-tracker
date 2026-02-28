FROM golang:1.25.5-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY *.go ./
RUN go build -ldflags="-s -w" -o priceTracker

FROM chromedp/headless-shell:latest

# Install CA certificates
USER root
RUN apt-get update && \
  apt-get install -y --no-install-recommends ca-certificates wget && \
  rm -rf /var/lib/apt/lists/*
ENV TZ=America/Los_Angeles
WORKDIR /app
COPY --from=builder /app/priceTracker .

ENTRYPOINT []
CMD ["./priceTracker"]
