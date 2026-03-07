# Build stage (context: repo root so replace ../../proto resolves)
FROM golang:1.25-alpine AS builder
WORKDIR /app
# replace => ../../proto expects /proto when WORKDIR is /app
COPY proto /proto
COPY services/kyc/go.mod services/kyc/go.sum ./
RUN go mod download
COPY services/kyc/ ./
# -p 1 and GOGC reduce peak memory to avoid OOM kill in constrained Docker builds
ENV GOGC=50
RUN CGO_ENABLED=0 GOOS=linux go build -p 1 -ldflags="-s -w" -o kyc-http cmd/api/main.go

# Run stage (kyc-http serves both HTTP and gRPC in-process; kyc-grpc excluded to avoid port conflict and proto duplicate)
FROM alpine:3.18
WORKDIR /app
COPY --from=builder /app/kyc-http .
RUN chmod +x /app/kyc-http
ENV KYC_SERVICE_PORT=8002
EXPOSE 8002 9002
CMD ["./kyc-http"]