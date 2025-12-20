FROM golang:1.25.5-alpine

WORKDIR /app
COPY . .

RUN go build -o pricetracker

CMD ["./pricetracker"]