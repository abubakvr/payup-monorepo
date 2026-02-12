# Build stage (context: ./services/kyc — proto is in-repo at proto/kyc/)
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o kyc-http cmd/api/main.go
RUN go build -o kyc-grpc ./internal/grpc

# Run stage
FROM alpine:3.18
WORKDIR /app
COPY --from=builder /app/kyc-http .
COPY --from=builder /app/kyc-grpc .
ENV KYC_SERVICE_PORT=8002
EXPOSE 8002 9002
CMD ["sh", "-c", "./kyc-http & ./kyc-grpc & wait"]