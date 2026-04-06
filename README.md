# budgeteer

A personal finance tracker built in Go. Track transactions, set monthly budgets per category, and query your spending with a GraphQL API. Summary queries are cached in Redis to keep reads fast.

---

## Features

- **GraphQL API** with full query and mutation support, plus a built-in GraphiQL browser IDE at `/graphql`
- **PostgreSQL** backend with indexed queries and an upsert-based budget system
- **Redis caching** for summary aggregates — cache-aside pattern with automatic invalidation on writes
- **Spending summaries** — per-category spend vs budget, computed with a single SQL join
- **Docker + docker-compose** — one command brings up the API, Postgres, and Redis together
- **Kubernetes manifests** — Deployment, Service, HorizontalPodAutoscaler, liveness/readiness probes
- **Structured JSON logging** (`log/slog`), request ID propagation, panic recovery, graceful shutdown

---

## Project Layout

```
budgeteer/
├── cmd/
│   └── server/
│       └── main.go              # Entry point: wires Postgres, Redis, GraphQL, HTTP
├── internal/
│   ├── cache/
│   │   └── cache.go             # CachedStore: Redis cache-aside wrapper
│   ├── graph/
│   │   └── schema.go            # GraphQL schema, types, queries, mutations
│   ├── middleware/
│   │   └── middleware.go        # Logger, RequestID, Recover, Chain
│   ├── model/
│   │   └── model.go             # Domain types, validation
│   └── store/
│       └── postgres.go          # Store interface + PostgresStore implementation
├── k8s/
│   └── manifests.yaml           # Deployment, Service, HPA
├── scripts/
│   └── migrate.sql              # Schema setup
├── Dockerfile                   # Multi-stage → scratch image
├── docker-compose.yml           # API + Postgres + Redis
└── go.mod
```

---

## Getting Started

**Prerequisites:** Docker and docker-compose

```bash
git clone https://github.com/dakotadornbrack/budgeteer
cd budgeteer
docker compose up --build
```

The API starts on `http://localhost:8080`. Open `http://localhost:8080/graphql` in your browser for the GraphiQL IDE.

---

## GraphQL API

### Create a transaction

```graphql
mutation {
  createTransaction(
    description: "Grocery run"
    amountCents: 8450
    category: food
    date: "2026-03-15"
  ) {
    id
    description
    amountCents
    category
    date
  }
}
```

### Set a monthly budget

```graphql
mutation {
  setBudget(
    category: food
    limitCents: 60000
    month: 3
    year: 2026
  ) {
    id
    category
    limitCents
  }
}
```

### Get a spending summary (cached in Redis)

```graphql
query {
  summary(month: 3, year: 2026) {
    category
    totalCents
    budgetCents
  }
}
```

### List transactions with filters

```graphql
query {
  transactions(category: food, month: 3, year: 2026, limit: 10) {
    id
    description
    amountCents
    date
  }
}
```

### Delete a transaction

```graphql
mutation {
  deleteTransaction(id: "your-uuid-here")
}
```

---

## Categories

`food` · `transport` · `housing` · `entertainment` · `health` · `other`

---

## Environment Variables

| Variable       | Default                                                       | Description             |
|----------------|---------------------------------------------------------------|-------------------------|
| `PORT`         | `8080`                                                        | HTTP listen port        |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/budgeteer?sslmode=disable` | PostgreSQL DSN |
| `REDIS_URL`    | `redis://localhost:6379`                                      | Redis connection URL    |

---

## Kubernetes

```bash
# Create secrets first
kubectl create secret generic budgeteer-secrets \
  --from-literal=database-url='postgres://...' \
  --from-literal=redis-url='redis://...'

# Deploy
kubectl apply -f k8s/manifests.yaml
```

The HPA scales between 2 and 10 replicas based on CPU utilization.

---

## Design Decisions

**Cache-aside with explicit invalidation** — summary queries aggregate across all transactions for a month, which gets expensive as data grows. Redis caches the result for 5 minutes. Any write (new transaction, deleted transaction, updated budget) immediately invalidates the relevant cache key so reads never return stale data.

**Store interface** — `PostgresStore` implements a `Store` interface. Swapping in a different backend (SQLite for tests, a mock for unit tests) requires zero changes to GraphQL resolvers or handlers.

**Amounts in cents** — money is stored as integers (cents) to avoid floating-point precision errors. `$84.50` is stored as `8450`.

**Upsert budgets** — `ON CONFLICT (category, month, year) DO UPDATE` means `setBudget` is idempotent. Callers don't need to check whether a budget exists before setting one.

---

## What a Production Version Would Add

- JWT authentication middleware
- Per-user data isolation (user_id on every row)
- `golang-migrate` for versioned schema migrations
- Prometheus metrics at `/metrics`
- OpenTelemetry tracing
- Pagination cursors instead of limit/offset
- Google Cloud SQL + Cloud Memorystore (Redis) on GCP

---

## License

MIT
