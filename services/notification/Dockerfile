FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -p 1 -ldflags="-s -w" -o notification-service ./cmd

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/notification-service .
EXPOSE 8080
CMD ["./notification-service"]
