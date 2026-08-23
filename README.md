# Last-Mile Delivery Tracker

Two-service system per `references/ARCHITECTURE.md`:

- `backend/` — Go Fiber API. Layered: `handlers/ → services/ → repository/`, with `db/` for the pgx pool and `middleware/` for Clerk JWT auth + role checks.
- `frontend/` — Next.js app (`pnpm dev`). Calls the backend over HTTPS with the Clerk JWT in the `Authorization` header.
- `schema.sql` — PostgreSQL schema (3NF), copied from `references/schema.sql`.

## Layout

```
backend/
├── Dockerfile
├── go.mod
├── main.go
├── handlers/       # Fiber route handlers
│   ├── orders.go
│   ├── agents.go
│   ├── admin.go
│   └── clerk_webhook.go
├── services/       # Business logic
│   ├── rate_engine.go        # volumetric + chargeable weight, charge breakdown
│   ├── zone_detector.go      # pincode → zone, Haversine distance
│   ├── assignment_engine.go  # nearest AVAILABLE agent in pickup zone
│   └── notifier.go           # notification log writes + email send
├── repository/     # SQL via pgx, no ORM
│   ├── orders_repo.go
│   ├── zones_repo.go
│   ├── rate_cards_repo.go
│   ├── agents_repo.go
│   └── users_repo.go         # Clerk webhook sync
├── middleware/
│   └── auth.go               # ClerkAuth() + RequireRole()
└── db/
    └── db.go                 # pool setup + Postgres-backed limiter store

frontend/
├── Dockerfile
├── proxy.ts        # role-based route guard (Next.js 16 renamed middleware.ts → proxy.ts)
└── app/
```

## Run

Backend:

```sh
cd backend
cp ../.env.example .env   # fill DATABASE_URL etc.
go mod tidy
go run .
```

Frontend:

```sh
cd frontend
pnpm install
pnpm dev
```

Database: run `schema.sql` once against Postgres.

## Deploy (Railway)

One project, three services: `backend` (root `/backend`), `frontend` (root `/frontend`), `postgres`. See ARCHITECTURE.md §12 for env vars and watch paths.
