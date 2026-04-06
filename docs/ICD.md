# Interface Control Document (ICD)
## Budgeteer — Personal Finance Tracker API

| Field        | Value                        |
|--------------|------------------------------|
| Document ID  | BUDGETEER-ICD-001            |
| Version      | 1.0                          |
| Date         | 2026-04-05                   |
| Status       | Released                     |

---

## 1. Purpose

This document defines all interfaces between the Budgeteer API and the systems it communicates with. It is the authoritative reference for:

- Clients consuming the GraphQL API
- Services providing data to the API (PostgreSQL, Redis)
- Infrastructure teams deploying or monitoring the system

---

## 2. System Overview

Budgeteer is a three-tier application:

```
┌─────────────────────────────────────────────┐
│                  Client                      │
│  (browser, GraphiQL IDE, API consumer)       │
└──────────────────┬──────────────────────────┘
                   │  HTTP/GraphQL  (Interface A)
┌──────────────────▼──────────────────────────┐
│              Budgeteer API                   │
│         Go 1.22 — port 8080                  │
└──────────┬──────────────────┬───────────────┘
           │  SQL (Interface B)│  Commands (Interface C)
┌──────────▼────────┐  ┌──────▼───────────────┐
│    PostgreSQL 16   │  │      Redis 7          │
│    port 5432       │  │      port 6379        │
└───────────────────┘  └──────────────────────┘
```

| Interface | Direction          | Protocol       | Description                        |
|-----------|--------------------|----------------|------------------------------------|
| A         | Client → API       | HTTP/GraphQL   | Public API — queries and mutations |
| B         | API → PostgreSQL   | TCP/SQL (lib/pq) | Persistent storage               |
| C         | API → Redis        | TCP/RESP       | Summary cache read/write/invalidate |

---

## 3. Interface A — HTTP / GraphQL (Client ↔ API)

### 3.1 Transport

| Property         | Value                                    |
|------------------|------------------------------------------|
| Protocol         | HTTP/1.1                                 |
| Default port     | `8080` (overridden by `PORT` env var)    |
| GraphQL endpoint | `POST /graphql`                          |
| IDE endpoint     | `GET /graphql` (GraphiQL browser UI)     |
| Content-Type     | `application/json`                       |
| Encoding         | UTF-8                                    |

### 3.2 Request Format

All GraphQL operations are sent as a JSON body to `POST /graphql`:

```json
{
  "query": "...",
  "variables": {}
}
```

### 3.3 Response Format

```json
{
  "data": { ... },
  "errors": [
    {
      "message": "human-readable error string"
    }
  ]
}
```

- `data` is present on success; individual fields may be `null` if that object was not found.
- `errors` is present only when one or more errors occurred. HTTP status is still `200` for GraphQL-level errors.

### 3.4 Scalar Types

| GraphQL Type | Go Type     | Format / Notes                              |
|--------------|-------------|---------------------------------------------|
| `String`     | `string`    | UTF-8                                       |
| `Int`        | `int`       | 32-bit signed integer                       |
| `Boolean`    | `bool`      | `true` / `false`                            |
| `date`       | `string`    | `YYYY-MM-DD` (e.g. `"2026-03-15"`)         |
| `createdAt`  | `string`    | RFC 3339 (e.g. `"2026-03-15T10:00:00Z"`)  |

### 3.5 Enum: Category

| Value           | Description                  |
|-----------------|------------------------------|
| `food`          | Groceries and dining         |
| `transport`     | Travel and fuel              |
| `housing`       | Rent, mortgage, utilities    |
| `entertainment` | Leisure and subscriptions    |
| `health`        | Medical and fitness          |
| `other`         | Anything else                |

### 3.6 Object Types

#### Transaction

| Field         | Type              | Nullable | Description                         |
|---------------|-------------------|----------|-------------------------------------|
| `id`          | `String!`         | No       | UUID v4                             |
| `description` | `String!`         | No       | Free-text label                     |
| `amountCents` | `Int!`            | No       | Amount in cents (e.g. 8450 = $84.50)|
| `category`    | `Category!`       | No       | Spending category enum              |
| `date`        | `String!`         | No       | Transaction date (`YYYY-MM-DD`)     |
| `createdAt`   | `String!`         | No       | Record creation timestamp (RFC 3339)|

#### Budget

| Field         | Type        | Nullable | Description                          |
|---------------|-------------|----------|--------------------------------------|
| `id`          | `String!`   | No       | UUID v4                              |
| `category`    | `Category!` | No       | Budget category                      |
| `limitCents`  | `Int!`      | No       | Monthly spending limit in cents      |
| `month`       | `Int!`      | No       | Month number (1–12)                  |
| `year`        | `Int!`      | No       | Four-digit year (2000–2100)          |

#### Summary

| Field         | Type        | Nullable | Description                                 |
|---------------|-------------|----------|---------------------------------------------|
| `category`    | `Category!` | No       | Spending category                           |
| `totalCents`  | `Int!`      | No       | Total spent in the given month/year         |
| `budgetCents` | `Int!`      | No       | Budget limit; `0` if no budget is set       |
| `month`       | `Int!`      | No       | Month number (1–12)                         |
| `year`        | `Int!`      | No       | Four-digit year                             |

---

### 3.7 Queries

#### `transaction(id: String!): Transaction`

Fetch a single transaction by ID.

| Argument | Type      | Required | Description   |
|----------|-----------|----------|---------------|
| `id`     | `String!` | Yes      | Transaction UUID |

**Returns:** `Transaction` or `null` if not found.

**Example:**
```graphql
query {
  transaction(id: "d290f1ee-6c54-4b01-90e6-d701748f0851") {
    id
    description
    amountCents
    category
    date
  }
}
```

---

#### `transactions(...): [Transaction]`

List transactions with optional filters. Returns newest-first (by date).

| Argument   | Type       | Required | Default | Constraints         |
|------------|------------|----------|---------|---------------------|
| `category` | `Category` | No       | all     |                     |
| `month`    | `Int`      | No       | all     | 1–12                |
| `year`     | `Int`      | No       | all     | 2000–2100           |
| `limit`    | `Int`      | No       | 50      | Max 100             |
| `offset`   | `Int`      | No       | 0       |                     |

**Returns:** Array of `Transaction`. Empty array if no matches.

**Example:**
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

---

#### `budgets(month: Int!, year: Int!): [Budget]`

List all budgets for a given month and year.

| Argument | Type   | Required | Constraints |
|----------|--------|----------|-------------|
| `month`  | `Int!` | Yes      | 1–12        |
| `year`   | `Int!` | Yes      | 2000–2100   |

**Returns:** Array of `Budget`. Empty array if no budgets set.

**Example:**
```graphql
query {
  budgets(month: 3, year: 2026) {
    category
    limitCents
  }
}
```

---

#### `summary(month: Int!, year: Int!): [Summary]`

Spending vs budget by category for a given month. Results are cached in Redis for 5 minutes. Returns one `Summary` entry per category that has either a transaction or a budget in the given period.

| Argument | Type   | Required | Constraints |
|----------|--------|----------|-------------|
| `month`  | `Int!` | Yes      | 1–12        |
| `year`   | `Int!` | Yes      | 2000–2100   |

**Returns:** Array of `Summary`.

**Cache behavior:** On the first call for a given `(month, year)`, the result is computed by PostgreSQL and stored in Redis. Subsequent calls within 5 minutes are served from Redis. Any write (createTransaction, deleteTransaction, setBudget) immediately invalidates the cache for the affected month/year.

**Example:**
```graphql
query {
  summary(month: 3, year: 2026) {
    category
    totalCents
    budgetCents
  }
}
```

---

### 3.8 Mutations

#### `createTransaction(...): Transaction`

Record a new spending transaction.

| Argument      | Type        | Required | Constraints                   |
|---------------|-------------|----------|-------------------------------|
| `description` | `String!`   | Yes      | Non-empty                     |
| `amountCents` | `Int!`      | Yes      | Must be > 0                   |
| `category`    | `Category!` | Yes      | Must be a valid Category enum |
| `date`        | `String`    | No       | `YYYY-MM-DD`; defaults to today (UTC) |

**Returns:** The created `Transaction`.

**Side effects:** Invalidates the Redis summary cache for the transaction's month/year.

**Example:**
```graphql
mutation {
  createTransaction(
    description: "Grocery run"
    amountCents: 8450
    category: food
    date: "2026-03-15"
  ) {
    id
    amountCents
    date
  }
}
```

---

#### `deleteTransaction(id: String!): Boolean`

Delete a transaction by ID.

| Argument | Type      | Required | Description      |
|----------|-----------|----------|------------------|
| `id`     | `String!` | Yes      | Transaction UUID |

**Returns:** `true` on success.

**Side effects:** Invalidates the Redis summary cache for the deleted transaction's month/year.

**Errors:** Returns an error if the transaction does not exist.

**Example:**
```graphql
mutation {
  deleteTransaction(id: "d290f1ee-6c54-4b01-90e6-d701748f0851")
}
```

---

#### `setBudget(...): Budget`

Set or update a monthly budget for a category. This operation is idempotent — calling it again for the same `(category, month, year)` overwrites the previous value.

| Argument     | Type        | Required | Constraints           |
|--------------|-------------|----------|-----------------------|
| `category`   | `Category!` | Yes      | Valid Category enum   |
| `limitCents` | `Int!`      | Yes      | Must be > 0           |
| `month`      | `Int!`      | Yes      | 1–12                  |
| `year`       | `Int!`      | Yes      | 2000–2100             |

**Returns:** The created or updated `Budget`.

**Side effects:** Invalidates the Redis summary cache for the affected month/year.

**Example:**
```graphql
mutation {
  setBudget(
    category: food
    limitCents: 60000
    month: 3
    year: 2026
  ) {
    id
    limitCents
  }
}
```

---

### 3.9 Validation Errors

When input fails validation the response contains an `errors` array. HTTP status remains `200`.

| Condition                          | Error Message                              |
|------------------------------------|--------------------------------------------|
| `description` is empty             | `description is required`                  |
| `amountCents` ≤ 0                  | `amount_cents must be greater than zero`   |
| `category` is not a valid enum     | `invalid category`                         |
| `date` not in `YYYY-MM-DD` format  | `date must be YYYY-MM-DD: ...`             |
| `limitCents` ≤ 0                   | `limit_cents must be greater than zero`    |
| `month` outside 1–12               | `month must be between 1 and 12`           |
| `year` outside 2000–2100           | `year is out of range`                     |
| Transaction ID not found           | `not found`                                |

---

### 3.10 Health Endpoints

These endpoints are plain HTTP (not GraphQL) and are intended for load balancer and orchestration health checks.

#### `GET /health`

Liveness probe. Returns `200 OK` if the process is running.

**Response:**
```json
{"status":"ok"}
```

#### `GET /ready`

Readiness probe. Returns `200 OK` only if the API can reach Redis.

**Response (healthy):**
```json
{"status":"ready","goroutines":0}
```

**Response (unhealthy):** `503 Service Unavailable`
```
{"status":"redis unavailable"}
```

---

### 3.11 Request Tracing

The API accepts and propagates a `X-Request-ID` header for distributed tracing. If the header is absent, one is generated. The value is logged with every request.

| Header         | Direction       | Description                     |
|----------------|-----------------|----------------------------------|
| `X-Request-ID` | Request/Response | Unique ID for log correlation    |

---

## 4. Interface B — PostgreSQL (API ↔ Database)

### 4.1 Connection

| Property   | Value                                                         |
|------------|---------------------------------------------------------------|
| Driver     | `lib/pq` (Go)                                                 |
| DSN source | `DATABASE_URL` environment variable                           |
| Default    | `postgres://postgres:postgres@localhost:5432/budgeteer?sslmode=disable` |
| SSL        | Disabled by default; set `sslmode=require` for production     |

### 4.2 Schema

#### Table: `transactions`

| Column        | Type          | Constraints                  | Description                |
|---------------|---------------|------------------------------|----------------------------|
| `id`          | `TEXT`        | PRIMARY KEY                  | UUID v4                    |
| `description` | `TEXT`        | NOT NULL                     | Free-text label            |
| `amount_cents`| `BIGINT`      | NOT NULL, CHECK > 0          | Amount in cents            |
| `category`    | `TEXT`        | NOT NULL                     | Category enum value        |
| `date`        | `DATE`        | NOT NULL                     | Transaction date           |
| `created_at`  | `TIMESTAMPTZ` | NOT NULL, DEFAULT NOW()      | Record creation timestamp  |

**Indexes:**
- `idx_transactions_category` on `(category)`
- `idx_transactions_date` on `(date DESC)`

#### Table: `budgets`

| Column        | Type          | Constraints                           | Description               |
|---------------|---------------|---------------------------------------|---------------------------|
| `id`          | `TEXT`        | PRIMARY KEY                           | UUID v4                   |
| `category`    | `TEXT`        | NOT NULL                              | Category enum value       |
| `limit_cents` | `BIGINT`      | NOT NULL, CHECK > 0                   | Monthly spending limit    |
| `month`       | `SMALLINT`    | NOT NULL, CHECK BETWEEN 1 AND 12      | Month number              |
| `year`        | `SMALLINT`    | NOT NULL, CHECK BETWEEN 2000 AND 2100 | Four-digit year           |
| `created_at`  | `TIMESTAMPTZ` | NOT NULL, DEFAULT NOW()               | Record creation timestamp |

**Constraints:**
- `UNIQUE (category, month, year)` — one budget per category per month

### 4.3 Notable Behaviors

- **UUID generation** uses `pgcrypto`'s `gen_random_uuid()`.
- **Budget upsert** uses `ON CONFLICT (category, month, year) DO UPDATE SET limit_cents = EXCLUDED.limit_cents, id = EXCLUDED.id`. This makes `setBudget` idempotent.
- **Summary query** is a SQL `LEFT JOIN` between `transactions` and `budgets` grouped by category, filtered by month and year.

---

## 5. Interface C — Redis (API ↔ Cache)

### 5.1 Connection

| Property   | Value                                   |
|------------|-----------------------------------------|
| Client     | `redis/go-redis` v9                     |
| URL source | `REDIS_URL` environment variable        |
| Default    | `redis://localhost:6379`                |

### 5.2 Key Schema

| Key Pattern               | Example                    | Description                        |
|---------------------------|----------------------------|------------------------------------|
| `summary:{year}:{month}`  | `summary:2026:3`           | Cached summary result for a period |

### 5.3 Operations

| Operation   | Trigger                                         | TTL     |
|-------------|-------------------------------------------------|---------|
| SET         | First `summary` query for a `(month, year)`     | 5 min   |
| GET         | Subsequent `summary` queries within TTL         | —       |
| DEL         | `createTransaction`, `deleteTransaction`, `setBudget` | — |

### 5.4 Cache-Aside Pattern

1. On `summary` query: check Redis for `summary:{year}:{month}`.
2. **Cache hit:** deserialize and return immediately.
3. **Cache miss:** query PostgreSQL, serialize result, write to Redis with 5-minute TTL, return result.
4. On any write that affects a month/year: delete `summary:{year}:{month}` from Redis (best-effort — a Redis failure does not fail the write).

### 5.5 Serialization

Cached values are JSON-encoded arrays of `Summary` objects.

---

## 6. Environment Variables

| Variable       | Required | Default                                                             | Description             |
|----------------|----------|---------------------------------------------------------------------|-------------------------|
| `PORT`         | No       | `8080`                                                              | HTTP listen port        |
| `DATABASE_URL` | No       | `postgres://postgres:postgres@localhost:5432/budgeteer?sslmode=disable` | PostgreSQL DSN      |
| `REDIS_URL`    | No       | `redis://localhost:6379`                                            | Redis connection URL    |

---

## 7. Startup Behavior

On start, the API:

1. Attempts to connect to PostgreSQL — retries up to **5 times** with a **3-second delay** between attempts. Exits with code 1 if all retries fail.
2. Attempts to connect to Redis — same retry policy.
3. Builds the GraphQL schema.
4. Starts the HTTP server.
5. Listens for `SIGINT` / `SIGTERM` and performs a graceful shutdown with a **10-second drain timeout**.

---

## 8. Revision History

| Version | Date       | Author | Description     |
|---------|------------|--------|-----------------|
| 1.0     | 2026-04-05 | —      | Initial release |
