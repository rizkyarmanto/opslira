# AGENTS.md: Opslira

Internal infrastructure monitoring application. Laravel 11 backend, React 18 + Inertia
frontend, PostgreSQL 16, Redis 7, Gotenberg for PDF.

## Layout

- `app/Modules/` holds the domain code, one folder per module: `Dashboard`, `Targets`,
  `Probes`, `Incidents`, `Reporting`. Each module owns its `Controllers/`, `Models/`,
  `Services/`, `Jobs/`, and `Routes/`. A module's routes are registered from
  `bootstrap/app.php`, not from `routes/web.php`.
- `resources/js/` holds the frontend: `pages/` (Inertia page components), `components/`,
  `lib/`, `types/`.
- `resources/views/reports/` holds the light-theme Blade templates that Gotenberg turns
  into PDF. These are the only Blade views that render content.

## Backend rules

- Target configuration is polymorphic: one `monitoring_targets` table with a
  `config_payload` JSONB column, validated per `target_type` in the application layer.
- Probe logic lives behind `ProbeAdapterInterface`. Adding a target category means adding
  an adapter, not editing the scheduler.
- Credentials are encrypted with AES-256-GCM before they reach the database and are never
  returned in plain text in any response. Mask them as `****`.
- Deletes are soft: `is_active = false`. Nothing hard-deletes a target.
- `monitoring_logs` is range-partitioned by month. Partition creation and pruning run from
  the scheduled commands in `app/Console/Commands/`.
- Migrations must stay reversible. A migration that drops or reshapes a partitioned table
  needs a matching `down()`.

## Frontend rules

- Read `DESIGN.md` for direction and `antislop.md` (core) plus `skills/antislop-ui/SKILL.md`
  and `skills/antislop-copywriting/SKILL.md` as the filter before any UI or copy change.
- Pages are Inertia components under `resources/js/pages/`, named to match the controller's
  `Inertia::render()` call.
- Use Inertia's `useForm` for form submission. It carries the CSRF token; a raw
  `<form method="POST">` without a token returns 419.
- Status is rendered with `StatusBadge`, which always pairs an icon and a text label with
  the color. Never signal status with color alone.
- Every screen that shows data needs an empty state and an error state, not only the
  populated one.
- Tailwind semantic tokens (`noc-bg`, `noc-surface`, `noc-text`, `noc-muted`, `noc-accent`)
  come from `tailwind.config.js`. Prefer them over raw zinc values in new code.
- Text must clear WCAG AA contrast. `noc-muted` is the floor for text; darker zinc shades
  are decoration only.

## Commands

```bash
composer install
npm install
php artisan migrate --force
php artisan db:seed --force
npm run dev            # Vite on 5173
php artisan serve      # Laravel on 8000
npm run build          # production bundle
npx tsc --noEmit       # type check
```

## Local environment note

This machine sets `variables_order = GPCS` in `php.ini`, so `$_ENV` is empty, and the
shell environment leaks empty `APP_KEY`, a stale `DB_PORT`, `MAIL_MAILER=log` and
`MAIL_SCHEME=implicit` into the PHP child process. Laravel's Dotenv is immutable and will
not overwrite them, which produces a `MissingAppKeyException`, a wrong-port connection
failure, or mail that is written to the log because the `log` mailer was pinned (Symfony
also rejects any scheme other than `smtp` or `smtps`, so `implicit` makes every send fail).

Start the server with a clean environment. The framework router takes its public path
from `getcwd()`, so the command must run with `public` as the working directory; running
it from the repo root with `-t public` makes it require a non-existent root `index.php`:

```bash
cd public
env -u APP_KEY -u APP_ENV -u DB_PORT -u DB_HOST -u DB_USERNAME -u DB_PASSWORD -u DB_DATABASE \
  -u MAIL_MAILER -u MAIL_SCHEME \
  php -S 127.0.0.1:8000 ../vendor/laravel/framework/src/Illuminate/Foundation/resources/server.php
```

Do not commit `.env`. Fix credentials in `.env` locally instead.

## Rules

- Run the build and click through the changed screens before calling work done. Report the
  click-through.
- No fabricated data. A number on screen traces to a probe, the database, or a seed row.
  When there is nothing real to show, show an empty state.
- No em dashes in UI text, code comments, or docs you author.

<!-- antislop:start -->
## antislop
For UI, copy, people, mobile layout, or code comments work, read `antislop.md` (core) and then the skill for the task:
- UI / visual: `skills/antislop-ui/SKILL.md`
- Copy & text: `skills/antislop-copywriting/SKILL.md`
- People: `skills/antislop-human/SKILL.md`
- Mobile / responsive: `skills/antislop-layoutmobile/SKILL.md`
- Code comments: `skills/antislop-code/SKILL.md`
Before starting, follow the core's "Two Usage Modes" section in strict order: explicit session instruction first, then global preference, then ask. A session instruction always wins. For a resolved mode, say `antislop active: <mode> (session override).` or `antislop active: <mode> (global preference).` once before presenting findings or making edits, using the actual mode and source. Acknowledging the user's request without naming the source does not replace this notice.
Only an explicit choice of antislop during or after selects a session mode. A request to review, audit, or avoid file edits does not select a mode; read the global preference in that case. Another skill's mode does not select antislop's mode.
If the mode is unresolved, ask during/after and end the response; wait for the answer before any UI review, planning, or concept. For read-only tasks, put the active-mode notice only at the start of the final answer, never in progress messages. For editing tasks, announce before the first edit and omit it from the final answer.
To update antislop later: download `antislop.md` again, or run `npx antislop-ai --update` if it was installed as skill folders.
<!-- antislop:end -->
