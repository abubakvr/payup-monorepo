# Build stage (context: repo root so replace ../../proto resolves)
FROM golang:1.25-alpine AS builder
WORKDIR /app
# replace => ../../proto expects /proto when WORKDIR is /app
COPY proto /proto
COPY services/user/go.mod services/user/go.sum ./
RUN go mod download
COPY services/user/ ./
# -p 1 reduces peak memory to avoid OOM kill in constrained Docker builds
RUN CGO_ENABLED=0 GOOS=linux go build -p 1 -ldflags="-s -w" -o user-service cmd/api/main.go

# Run stage
FROM alpine:3.18
WORKDIR /app
COPY --from=builder /app/user-service .
ENV USER_SERVICE_PORT=8001
EXPOSE 8001
CMD ["./user-service"]
