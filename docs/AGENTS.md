# AGENTS.md — Opslira Operating Manual

Operating manual for AI coding agents working in this repository.

## Project Identity

- **Name:** Opslira — Sistem Monitoring Infrastruktur & Layanan Internal (Single Pane of Glass)
- **Purpose:** Monitor 7 target categories — Network devices, Web apps, Domains/SSL, Databases, Mail servers, S3 storage, Beszel servers.
- **Repository root:** `opslira/` (this directory)

## Stack

| Layer | Technology |
|---|---|
| Backend | Laravel 11 (PHP 8.3), modular monolith under `app/Modules/` |
| Frontend | React 18 + TypeScript via Inertia.js, Tailwind CSS, framer-motion, ECharts, lucide-react, React Hook Form + Zod |
| Database | PostgreSQL 16 (declarative monthly partitions on `monitoring_logs`) |
| Cache/Queue | Redis 7 (3 priority lanes: alerts → probes → reports) |
| PDF | Gotenberg 8 (headless Chromium) |
| Deploy | Docker Compose (7 services) |

## Design References

- `docs/gemini/` — architecture, ERD (DDL), PRD (source of truth for behaviour).
- `docs/referensi-design.md` — UI/UX specs: dark NOC theme, status colors, component catalogue, guardrails.
- `docs/Design.md` — Wise design tokens (spacing, elevation, rounded, typography).

## Architecture Rules

- **Namespaces:** `App\Modules\{Module}\` — each module is self-contained with `Controllers/`, `Models/`, `Requests/`, `Resources/`, `Routes/`.
- **Modules:** Dashboard, Targets, Probes, Incidents, Reporting.
- **Secrets:** Backend MUST NEVER return plain credentials in API responses or Inertia props. Mask as `••••`. Credentials are AES-256-GCM encrypted (`CredentialEncryptionService`), stored as ciphertext + IV + tag in `target_credentials`.
- **Queue lanes:** `alerts` (priority 1), `probes` (priority 2, 3 replicas), `reports` (priority 3, backoff 60/300/900).
- **Redis keys:** `target:state:{uuid}` (JSON status), `target:failures:{id}` (consecutive failure counter).
- **Log inserts:** `MonitoringLog` rows are inserted via `DB::table()` raw insert (not Eloquent) for partition performance.
- **Anti-flapping:** DOWN only after `retry_threshold` (default 3) consecutive failures. Recovery requires a successful check after a DOWN state.

## Database Rules

- **UUID PK** for core entities (`users`, `monitoring_targets`, `incidents`, `report_schedules`, `generated_reports`, `email_delivery_logs`). `BIGSERIAL` identity for append-only (`monitoring_logs`, `monitoring_daily_summaries`).
- **Partitions:** `monitoring_logs` is `PARTITION BY RANGE (checked_at)`, monthly. Scheduler pre-creates next 2 months and drops partitions > 90 days. Manual fallback: `php artisan monitoring:create-partition {YYYY-MM}`.
- **Timestamps:** stored as `timestamptz` (UTC) in DB; convert to `Asia/Jakarta` in the application layer.
- **Encryption key:** `APP_ENCRYPTION_KEY` env var, AES-256-GCM.

## Frontend Rules

- **Theme:** dark NOC by default (`dark` class on `<html>`). Tokens: `bg-zinc-950` base, status colors emerald (UP) / amber (DEGRADED) / rose (DOWN) / sky (MAINTENANCE).
- **Design tokens** from `docs/Design.md`: spacing scale (`xxs`…`band`), ring-shadow-only elevation (no drop shadow), rounded scale (`xs`…`full`), typography (Geist display weight 900, Inter body, JetBrains Mono code).
- **Component libraries** (5 sources, see `docs/referensi-design.md`):
  - `ui-layouts` — BentoGrid, AnimatedTabs, SlideOverDrawer
  - `motion-primitives` — SlidingNumber, BorderTrail, TransitionPanel
  - `beautiful-ui` — StatusCard, ExpiryCountdownCard, ProtocolBadge, CommandPalette
  - `uiverse` — PulseDot, MaintenanceSwitch, RadarButtonLoader
  - `transitions.dev` — View Transitions API
- **`BorderTrail`** MUST be conditionally rendered — only when `status === 'DOWN'`. Never mounted on UP (CPU/GPU guardrail).
- **Status badges** MUST include an icon + explicit text label (lucide-react: `CheckCircle2` / `AlertTriangle` / `XOctagon`), never color alone.
- **`font-mono tabular-nums`** (`.tabular`) is mandatory for IPs, ports, SLA %, response times, SSL days remaining, and logs.
- **PDF reports:** light theme, Blade templates in `resources/views/reports/`, static SVG (no framer-motion), `data-pdf-ready="true"` attribute.
- **Page Visibility Throttling:** when `document.hidden`, pause `SlidingNumber` animations and slow polling (`refetchIntervalInBackground: false`).

## Development Commands

```bash
docker-compose up -d --build                              # start all services
docker-compose exec monitoring-web php artisan migrate    # run migrations
docker-compose exec monitoring-web php artisan db:seed    # seed demo data
docker-compose exec monitoring-web php artisan test       # run tests
docker-compose exec monitoring-web php artisan monitoring:dispatch-probes   # manual probe dispatch
docker-compose exec monitoring-web php artisan monitoring:rollup            # daily rollup
docker-compose exec monitoring-web php artisan monitoring:dispatch-reports  # report dispatch
docker-compose exec monitoring-web php artisan monitoring:prune-partitions  # partition maintenance
docker-compose exec monitoring-web npm run dev            # Vite dev server (HMR)
docker-compose exec monitoring-web npm run build          # production build
docker-compose exec monitoring-web php artisan route:list # list routes
```

## Testing Rules

- **Backend:** Pest PHP. Feature tests per module. Probe adapters are tested against mocks — never hit real endpoints in CI.
- **Frontend:** Vitest + React Testing Library. Cover status badges (UP/DEGRADED/DOWN/MAINTENANCE), Zod validation per `target_type`, and dashboard rendering.
- **E2E (optional):** Playwright for dashboard load, CRUD flow, and report generation.

## Key File Map

- Probe engine: `app/Modules/Probes/Contracts/ProbeAdapterInterface.php` + 7 adapters in `app/Modules/Probes/Adapters/`.
- Probe orchestration: `app/Modules/Probes/Services/ProbeExecutionService.php`.
- State/cache: `app/Modules/Probes/Services/TargetStateService.php`.
- Reporting: `app/Modules/Reporting/Services/SlaCalculatorService.php`, `Builders/`, `Jobs/GenerateAndSendReportJob.php`.
- Frontend pages: `resources/js/pages/{Dashboard,Targets,Reports}/`.
- Design components: `resources/js/components/{ui-layouts,motion-primitives,beautiful-ui,uiverse,shared}/`.

## Dependencies

- **PHP:** `inertiajs/inertia-laravel`, `aws/aws-sdk-php`, `predis/predis`.
- **JS:** `@inertiajs/react`, `react`, `react-dom`, `framer-motion`, `echarts`, `echarts-for-react`, `lucide-react`, `react-hook-form`, `zod`, `@hookform/resolvers`, `@tanstack/react-query`, `tailwindcss`, `@vitejs/plugin-react`, `typescript`.
