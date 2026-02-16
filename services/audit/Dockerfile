FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o audit-service ./cmd

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/audit-service .
EXPOSE 8080
CMD ["./audit-service"]
