# Build stage (context: repo root so replace ../../proto resolves)
FROM golang:1.25-alpine AS builder
WORKDIR /app
# replace => ../../proto expects /proto when WORKDIR is /app
COPY proto /proto
COPY services/admin/go.mod services/admin/go.sum ./
RUN go mod download
COPY services/admin/ ./
# -p 1 reduces peak memory to avoid OOM kill in constrained Docker builds
RUN CGO_ENABLED=0 GOOS=linux go build -p 1 -ldflags="-s -w" -o admin-service cmd/api/main.go

FROM alpine:3.18
WORKDIR /app
COPY --from=builder /app/admin-service .
ENV ADMIN_SERVICE_PORT=8005
EXPOSE 8005
CMD ["./admin-service"]
