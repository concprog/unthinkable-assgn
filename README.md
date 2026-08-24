# Last-Mile Delivery Tracker

**Live demo: https://lastmile-tracker.up.railway.app**

Two-service system — a Go Fiber API holding all business logic, and a Next.js app that renders the UI and calls it over HTTPS:

- `backend/` — Go Fiber API. Layered: `handlers/ → services/ → repository/`, with `db/` for the pgx pool and `middleware/` for Clerk JWT auth + role checks.
- `frontend/` — Next.js app (`pnpm dev`). Calls the backend over HTTPS with the Clerk JWT in the `Authorization` header.
- `schema.sql` — PostgreSQL schema (3NF): nine normalized tables plus an append-only `order_status_history` guarded by database triggers.

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

One project, three services: `backend`, `frontend`, `postgres`. Each service sets its own root directory and watch path, so a change in one directory does not rebuild the other. Railway injects `DATABASE_URL` automatically when the backend links to the Postgres service.

Backend env vars: `DATABASE_URL`, `CLERK_JWKS_URL`, `EMAIL_PROVIDER_API_KEY`.
Frontend env vars: `NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY`, `NEXT_PUBLIC_API_URL` (baked at image build).

```sh
# after changing backend code
docker build -t concprog/lastmile-backend:latest ./backend && docker push concprog/lastmile-backend:latest
railway redeploy -s backend --yes

# after changing frontend code (NEXT_PUBLIC_API_URL bakes at build time)
docker build --build-arg NEXT_PUBLIC_API_URL=https://<backend-domain> -t concprog/lastmile-frontend:latest ./frontend
docker push concprog/lastmile-frontend:latest
railway redeploy -s frontend --yes
```

The backend applies `schema.sql` automatically on first boot (`db.Migrate`, skipped if tables exist).

## Design rationale

- `SYSTEM_DESIGN_REPORT.md` — point-form design rationale: rate engine, zone detection, assignment strategy, failure handling, notification providers, and the Go-over-FastAPI stack justification

## Architecture at a glance

- **Charge calculation** runs eight fixed steps per order: zone detection (pincode → zone table lookup), volumetric weight `(L×B×H)/5000`, chargeable weight (max of actual vs volumetric), rate-card lane lookup, base charge, COD surcharge, fuel surcharge, GST. Every order snapshots the breakdown at creation.
- **Auto-assignment** queries AVAILABLE agents in the pickup zone, ranks them by Haversine distance to the pickup address, assigns the nearest, and writes an immutable history row.
- **Status lifecycle**: `CREATED → CONFIRMED → ASSIGNED → PICKED_UP → IN_TRANSIT → OUT_FOR_DELIVERY → DELIVERED`, with `FAILED → RESCHEDULED → ASSIGNED` loop-back and `CANCELLED` before pickup. Admin overrides bypass the machine but always log `actor_type = 'ADMIN'`.
