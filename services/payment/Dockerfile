# Build stage (context: repo root so replace ../../proto resolves)
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY proto /proto
COPY services/payment/go.mod services/payment/go.sum ./
RUN go mod download
COPY services/payment/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -p 1 -ldflags="-s -w" -o payment-http cmd/api/main.go

FROM alpine:3.18
WORKDIR /app
COPY --from=builder /app/payment-http .
RUN chmod +x /app/payment-http
ENV PAYMENT_SERVICE_PORT=8006
ENV PAYMENT_GRPC_PORT=9004
EXPOSE 8006 9004
CMD ["./payment-http"]
