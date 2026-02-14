# PayUp Backend

Backend services for PayUp: user service, KYC service, Postgres, and Redpanda (Kafka).

## Services

| Service        | Port(s)   | Description              |
|----------------|-----------|--------------------------|
| user-service   | 8001      | User API (HTTP)          |
| kyc-service    | 8002, 9002| KYC API (HTTP + gRPC)    |
| Redpanda       | 9092      | Kafka-compatible broker  |
| Postgres       | 5432      | Database                 |

## Run with Docker

```bash
docker compose up -d
```

View logs for app services only:

```bash
docker compose logs -f user-service kyc-service
```

## Local development

From the repo root, run each service from its directory (e.g. `cd services/user && go run ./cmd/api`). Ensure Postgres and Redpanda are running (or use Docker for infra only).
