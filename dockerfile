FROM golang:1.25.5-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o priceTracker

# Stage 2: Runtime
FROM alpine:latest

COPY --from=builder /app/priceTracker /priceTracker

CMD ["/priceTracker"]