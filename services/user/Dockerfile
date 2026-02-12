# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o user-service cmd/api/main.go

# Run stage
FROM alpine:3.18
WORKDIR /app
COPY --from=builder /app/user-service .
ENV USER_SERVICE_PORT=8001
EXPOSE 8001
CMD ["./user-service"]
