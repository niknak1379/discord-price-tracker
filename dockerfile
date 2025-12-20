FROM golang:1.25.5-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o priceTracker

FROM alpine:latest AS runtime

RUN apk add --no-cache chromium

ENV CHROME_BIN=/usr/bin/chromium-browser

COPY --from=builder /app/priceTracker /priceTracker

CMD ["/priceTracker"]