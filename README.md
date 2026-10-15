# Opslira: Internal Infrastructure Monitoring

A single pane of glass for seven target categories: network devices, web apps, domains and
TLS certificates, databases, mail servers, S3 object storage, and Beszel servers.

## Stack

- **Backend:** Laravel 11 (PHP 8.3), modular monolith under `app/Modules/`
- **Frontend:** React 18 + TypeScript via Inertia.js, Tailwind CSS, framer-motion, ECharts, lucide-react, React Hook Form + Zod
- **Database:** PostgreSQL 16 (monthly partitions on `monitoring_logs`)
- **Cache and queue:** Redis 7 (three priority lanes: `alerts`, then `probes`, then `reports`)
- **PDF:** Gotenberg 8 (headless Chromium)
- **Deploy:** Docker Compose (7 services)

## Architecture

```
Scheduler (every minute) ──▶ queue:probes ──▶ ProbeAdapter (7 protocols) ──▶ PostgreSQL (partitions) + Redis state
                                                    │
                                          state change ──▶ queue:alerts ──▶ Email/Telegram/Slack
                                                    │
Midnight rollup ──▶ monitoring_daily_summaries
                                                    │
Report schedule ──▶ queue:reports ──▶ Gotenberg PDF + Excel ──▶ Email
```

## Running it

```bash
cp .env.example .env
# Set APP_KEY, APP_ENCRYPTION_KEY, DB_PASSWORD
docker-compose up -d --build
docker-compose exec monitoring-web php artisan key:generate
docker-compose exec monitoring-web php artisan migrate
docker-compose exec monitoring-web php artisan db:seed
```

Open <http://localhost:8080> and sign in with `admin@opslira.local` / `ChangeMe!2026`.

## The seven target protocols

| Target type | Protocol | Adapter |
|---|---|---|
| NETWORK_DEVICE | ICMP_SNMP | `NetworkSnmpProbeAdapter` |
| WEB_APP | HTTP_HEALTH | `HttpHealthProbeAdapter` |
| DOMAIN | WHOIS_SSL | `DomainSslProbeAdapter` |
| DATABASE | DB_NATIVE_PING | `DatabasePingProbeAdapter` |
| MAIL_SERVER | SMTP_DNSBL | `MailServerProbeAdapter` |
| S3_STORAGE | S3_HEAD | `S3BucketProbeAdapter` |
| BESZEL_SERVER | BESZEL_API | `BeszelApiProbeAdapter` |

## Documentation

- `AGENTS.md`: operating manual for the coding agent
- `docs/gemini/`: architecture, ERD and DDL, PRD
- `docs/referensi-design.md`: UI and UX specification
- `docs/Design.md`: design tokens

## Commands

```bash
php artisan monitoring:dispatch-probes     # dispatch probe jobs
php artisan monitoring:rollup              # daily aggregation
php artisan monitoring:dispatch-reports    # dispatch reports
php artisan monitoring:prune-partitions    # prune and create partitions
php artisan monitoring:create-partition YYYY-MM  # create one partition by hand
```
