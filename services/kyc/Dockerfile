# Build stage (context: repo root so replace ../../proto resolves)
FROM golang:1.25-alpine AS builder
WORKDIR /app
# replace => ../../proto expects /proto when WORKDIR is /app
COPY proto /proto
COPY services/kyc/go.mod services/kyc/go.sum ./
RUN go mod download
COPY services/kyc/ ./
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