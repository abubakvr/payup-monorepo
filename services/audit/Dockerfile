# Build stage (context: repo root so replace ../../proto resolves)
FROM golang:1.25-alpine AS builder
WORKDIR /app
# replace => ../../proto expects /proto when WORKDIR is /app
COPY proto /proto
COPY services/audit/go.mod services/audit/go.sum ./
RUN go mod download
COPY services/audit/ ./
# -p 1 reduces peak memory to avoid OOM kill in constrained Docker builds
RUN CGO_ENABLED=0 GOOS=linux go build -p 1 -ldflags="-s -w" -o audit-service ./cmd

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/audit-service .
EXPOSE 8080
CMD ["./audit-service"]
