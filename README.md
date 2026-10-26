# Opslira Go

Go API + React SPA split of Opslira (internal infrastructure monitoring).
One binary serves `/api/v1` plus the in-process scheduler (probe ticks,
report dispatch, daily rollup/prune) — no separate runner service.

## Contents

- `backend/` Go API (Chi + pgx/Postgres 16, Redis 7; see `backend/go.mod`)
- `frontend/` React SPA (Vite + Tailwind, calls API via `VITE_API_BASE_URL`)
- `docker-compose.yml` full stack: frontend + backend + postgres + redis + gotenberg
- `docker-compose.slim.yml` slim: frontend + backend only (external Postgres/Redis, optional Gotenberg)

## Quickstart

```bash
cp .env.example .env   # fill in secrets first
# full stack (all-in-one: postgres + redis + gotenberg included)
docker compose up -d --build
# slim (external Postgres/Redis via DB_*/REDIS_*)
docker compose -f docker-compose.slim.yml up -d --build
```

- Frontend: http://localhost (nginx :80)
- Backend: http://localhost:8080 (`/healthz` for checks, `/api/v1` for the API)
- `FRONTEND_ORIGIN` must be the exact SPA origin (compose default
  `http://localhost`; Vite dev needs `http://localhost:5173`), otherwise the
  browser blocks cross-origin requests.
- `VITE_API_BASE_URL` is baked at image build time (`.../api/v1`); changing it
  needs a frontend rebuild.

## Configuration

All backend settings come from the environment (see `.env.example`).
The important groups:

| Area | Keys |
| --- | --- |
| Listen / CORS / session | `API_LISTEN_ADDR`, `FRONTEND_ORIGIN`, `SESSION_TTL_SECONDS` |
| Postgres 16 | `DB_HOST`, `DB_PORT`, `DB_DATABASE`, `DB_USERNAME`, `DB_PASSWORD`, `DB_SSLMODE` |
| Redis 7 | `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `DB`, `REDIS_CACHE_DB` |
| PDF converter (optional) | `GOTENBERG_URL` — empty disables PDF (rows fail per-report, XLSX/HTML keep working) |
| Report files + S3 archive | `REPORT_STORAGE_DIR`, `REPORT_S3_ENDPOINT/BUCKET/ACCESS_KEY/SECRET_KEY/REGION/PREFIX` |
| Scheduler | `PROBE_CONCURRENCY`, `RUNNER_TICK_SECONDS`, `REPORT_TICK_SECONDS`, `ROLLUP_AT`, `PRUNE_AT`, `RUNNER_TIMEZONE` |
| Secrets | `APP_KEY`, `APP_ENCRYPTION_KEY`, `OPSLIRA_RUNNER_TOKEN` (required) |
| Alerts / mail | `ALERT_RECIPIENTS`, `MAIL_*`, `TELEGRAM_*`, `SLACK_WEBHOOK_URL` |
| Seed switch | `ADMIN_EMAIL/NAME/PASSWORD` — set `ADMIN_PASSWORD` for production seed (superadmin only); empty runs the dev seed |

Notes:

- Slim compose ignores bundled infra on purpose: point `DB_HOST`/`REDIS_HOST`
  at what you run (dev default `127.0.0.1`). Full compose pins the backend to
  `postgres`/`redis`/`gotenberg` services instead.
- ICMP ping probes need `NET_RAW` (already in both compose files for backend).
- Report S3 archive (PDF+XLSX, all schedules/periods) is best-effort: empty
  `REPORT_S3_ENDPOINT` skips upload, local files stay the source of truth.
  Check logs for `Report archived to S3` / `Report archive upload failed`.

## First boot / seeding

- `ADMIN_PASSWORD` set → production seed: superadmin only (from `ADMIN_*`).
- `ADMIN_PASSWORD` empty → dev seed: 4 users, 8 targets, 3 schedules.
- Default dev login: `admin@opslira.local` / `ChangeMe!2026` (also
  `sysadmin@`, `developer@`, `manager@opslira.local`).
- Migrations under `backend/migrations/` are idempotent (`IF NOT EXISTS` +
  constraint guards) so a legacy Laravel schema converges cleanly.

## Dev

```bash
# backend (needs DB/Redis reachable; env via root .env)
cd backend && go run ./cmd/api
# frontend (points at the local API)
VITE_API_BASE_URL=http://127.0.0.1:8080/api/v1 npm run dev --prefix frontend
```

- Backend: http://localhost:8080, frontend dev: http://localhost:5173
- Root `.env` + `frontend/.env` are gitignored local files with secrets —
  never commit them.
- Checks: `go build ./...`, `go test ./...` (backend); `npm run typecheck`
  (frontend).

## Project layout (backend)

- `cmd/api/main.go` — wiring: config → DB/Redis → migrations → seed → routes + scheduler
- `internal/config/` — env loading with validated defaults
- `internal/auth|users|targets|dashboard|incidents|reporting|scheduler|...` — domain modules
- `internal/reportarchive/` — optional S3 upload for generated PDF/XLSX
- `internal/reportrender/` — HTML/XLSX builders + Gotenberg PDF client
- `migrations/` + `seed.go` — embedded idempotent bootstrap
