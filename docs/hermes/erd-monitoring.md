# ERD + Skema Database Monitoring — PostgreSQL + TimescaleDB

> Bahasa: Indonesia. DB: PostgreSQL 15+ + TimescaleDB. Timezone: UTC di DB, konversi di aplikasi.

---

## 1. Daftar Tabel

| # | Tabel | Jenis | Fungsi |
|---|-------|-------|--------|
| 1 | `users` | Relasional | Akun pengguna, role RBAC |
| 2 | `target_groups` | Relasional | Grup/folder target |
| 3 | `targets` | Relasional | Endpoint/host yang dimonitor |
| 4 | `target_credentials` | Relasional | Kredensial per target (basic/bearer/TLS) |
| 5 | `checks` | Hypertable (time-series) | Hasil cek metrik per interval |
| 6 | `incidents` | Relasional | Insiden down/degraded (dibuka dari checks) |
| 7 | `alert_rules` | Relasional | Aturan alert per target/grup |
| 8 | `alert_logs` | Relasional (partisi bulanan) | Jejak pengiriman alert |
| 9 | `reports` | Relasional | Laporan SLA yang sudah digenerate |
| 10 | `report_schedules` | Relasional | Jadwal laporan otomatis |
| 11 | `mail_queue` | Relasional | Antrean email keluar |
| 12 | `mail_logs` | Relasional (partisi bulanan) | Log hasil kirim email |
| 13 | `maintenance_windows` | Relasional | Jendela maintenance (menekan alert) |
| 14 | `audit_logs` | Relasional (partisi bulanan) | Jejak aksi pengguna |

---

## 2. Skema Detail per Tabel

### 2.1 `users`

| Kolom | Tipe | Constraint | Ket. |
|-------|------|------------|------|
| `id` | `UUID` | **PK**, DEFAULT `gen_random_uuid()` | — |
| `email` | `CITEXT` | UNIQUE NOT NULL | login |
| `password_hash` | `TEXT` | NOT NULL | argon2/bcrypt |
| `name` | `TEXT` | NOT NULL | — |
| `role` | `TEXT` | NOT NULL DEFAULT `'viewer'` CHECK (`role` IN ('admin','operator','viewer')) | RBAC |
| `is_active` | `BOOLEAN` | NOT NULL DEFAULT true | — |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT `now()` | — |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT `now()` | — |

### 2.2 `target_groups`

| Kolom | Tipe | Constraint | Ket. |
|-------|------|------------|------|
| `id` | `UUID` | **PK** | — |
| `name` | `TEXT` | NOT NULL UNIQUE | — |
| `description` | `TEXT` | NULL | — |
| `parent_id` | `UUID` | **FK → `target_groups.id`** ON DELETE SET NULL | hirarki |
| `created_by` | `UUID` | **FK → `users.id`** ON DELETE SET NULL | — |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT `now()` | — |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT `now()` | — |

### 2.3 `targets`

| Kolom | Tipe | Constraint | Ket. |
|-------|------|------------|------|
| `id` | `UUID` | **PK** | — |
| `group_id` | `UUID` | **FK → `target_groups.id`** ON DELETE SET NULL | — |
| `name` | `TEXT` | NOT NULL | — |
| `type` | `TEXT` | NOT NULL CHECK (`type` IN ('http','https','tcp','icmp','dns')) | — |
| `url` | `TEXT` | NOT NULL | URL/IP:port |
| `method` | `TEXT` | NOT NULL DEFAULT `'GET'` | untuk http/https |
| `expected_status` | `INT` | NULL DEFAULT 200 | — |
| `expected_body_regex` | `TEXT` | NULL | keyword match |
| `check_interval_s` | `INT` | NOT NULL DEFAULT 60 CHECK (`check_interval_s` >= 10) | — |
| `timeout_ms` | `INT` | NOT NULL DEFAULT 10000 | — |
| `is_active` | `BOOLEAN` | NOT NULL DEFAULT true | — |
| `created_by` | `UUID` | **FK → `users.id`** ON DELETE SET NULL | — |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT `now()` | — |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT `now()` | — |

UNIQUE (`url`, `method`).

### 2.4 `target_credentials`

| Kolom | Tipe | Constraint | Ket. |
|-------|------|------------|------|
| `id` | `UUID` | **PK** | — |
| `target_id` | `UUID` | **FK → `targets.id`** ON DELETE CASCADE, UNIQUE | 1:1 |
| `auth_type` | `TEXT` | NOT NULL CHECK (`auth_type` IN ('none','basic','bearer','mtls','header')) | — |
| `username` | `TEXT` | NULL | basic |
| `secret_ref` | `TEXT` | NULL | referensi vault/KMS, bukan plaintext |
| `tls_cert_ref` | `TEXT` | NULL | untuk mtls |
| `extra_headers` | `JSONB` | NOT NULL DEFAULT `'{}'` | — |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT `now()` | — |

> Secret asli simpan di vault (Vault/KMS). DB hanya simpan referensi.

### 2.5 `checks` (hypertable, time-series)

| Kolom | Tipe | Constraint | Ket. |
|-------|------|------------|------|
| `time` | `TIMESTAMPTZ` | NOT NULL | waktu cek, partition key |
| `target_id` | `UUID` | **FK → `targets.id`** ON DELETE CASCADE, NOT NULL | — |
| `status` | `TEXT` | NOT NULL CHECK (`status` IN ('up','down','degraded','timeout','error')) | — |
| `latency_ms` | `INTEGER` | NULL | — |
| `http_code` | `SMALLINT` | NULL | — |
| `dns_ms` | `INTEGER` | NULL | breakdown opsional |
| `tls_ms` | `INTEGER` | NULL | breakdown opsional |
| `ttfb_ms` | `INTEGER` | NULL | breakdown opsional |
| `bytes` | `INTEGER` | NULL | ukuran respons |
| `error` | `TEXT` | NULL | pesan error |
| `region` | `TEXT` | NULL DEFAULT `'local'` | multi-prober |

PK: (`time`, `target_id`).

### 2.6 `incidents`

| Kolom | Tipe | Constraint | Ket. |
|-------|------|------------|------|
| `id` | `UUID` | **PK** | — |
| `target_id` | `UUID` | **FK → `targets.id`** ON DELETE CASCADE, NOT NULL | — |
| `opened_at` | `TIMESTAMPTZ` | NOT NULL | — |
| `resolved_at` | `TIMESTAMPTZ` | NULL | NULL = masih open |
| `status` | `TEXT` | NOT NULL DEFAULT `'open'` CHECK (`status` IN ('open','acknowledged','resolved')) | — |
| `cause` | `TEXT` | NULL | down/timeout/error |
| `opened_by_rule_id` | `UUID` | **FK → `alert_rules.id`** ON DELETE SET NULL | — |
| `ack_by` | `UUID` | **FK → `users.id`** ON DELETE SET NULL | — |
| `ack_at` | `TIMESTAMPTZ` | NULL | — |
| `mttr_s` | `INTEGER` | NULL | `resolved_at - opened_at`, isi via trigger/aplikasi |

### 2.7 `alert_rules`

| Kolom | Tipe | Constraint | Ket. |
|-------|------|------------|------|
| `id` | `UUID` | **PK** | — |
| `name` | `TEXT` | NOT NULL | — |
| `scope` | `TEXT` | NOT NULL CHECK (`scope` IN ('target','group','global')) | — |
| `target_id` | `UUID` | **FK → `targets.id`** ON DELETE CASCADE, NULL | isi bila scope=target |
| `group_id` | `UUID` | **FK → `target_groups.id`** ON DELETE CASCADE, NULL | isi bila scope=group |
| `condition` | `JSONB` | NOT NULL | cth `{"consecutive_fail":3,"latency_p95_ms":2000}` |
| `severity` | `TEXT` | NOT NULL CHECK (`severity` IN ('info','warning','critical')) | — |
| `channels` | `JSONB` | NOT NULL DEFAULT `'["email"]'` | email/webhook/telegram |
| `cooldown_s` | `INT` | NOT NULL DEFAULT 900 | anti-spam |
| `is_active` | `BOOLEAN` | NOT NULL DEFAULT true | — |
| `created_by` | `UUID` | **FK → `users.id`** ON DELETE SET NULL | — |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT `now()` | — |

CHECK: tepat satu dari (`target_id`,`group_id`) terisi sesuai scope; scope=global keduanya NULL.

### 2.8 `alert_logs`

| Kolom | Tipe | Constraint | Ket. |
|-------|------|------------|------|
| `id` | `BIGINT` | **PK** GENERATED ALWAYS AS IDENTITY | — |
| `rule_id` | `UUID` | **FK → `alert_rules.id`** ON DELETE SET NULL | — |
| `incident_id` | `UUID` | **FK → `incidents.id`** ON DELETE SET NULL | — |
| `target_id` | `UUID` | **FK → `targets.id`** ON DELETE SET NULL | denormalisasi query cepat |
| `channel` | `TEXT` | NOT NULL | email/webhook/telegram |
| `payload` | `JSONB` | NULL | isi pesan |
| `status` | `TEXT` | NOT NULL CHECK (`status` IN ('sent','failed','suppressed')) | suppressed = maintenance/cooldown |
| `error` | `TEXT` | NULL | — |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT `now()` | partition key bulanan |

### 2.9 `reports`

| Kolom | Tipe | Constraint | Ket. |
|-------|------|------------|------|
| `id` | `UUID` | **PK** | — |
| `schedule_id` | `UUID` | **FK → `report_schedules.id`** ON DELETE SET NULL | NULL = manual |
| `period_start` | `DATE` | NOT NULL | — |
| `period_end` | `DATE` | NOT NULL | — |
| `format` | `TEXT` | NOT NULL CHECK (`format` IN ('pdf','csv','xlsx')) | — |
| `file_path` | `TEXT` | NULL | lokasi file |
| `generated_by` | `UUID` | **FK → `users.id`** ON DELETE SET NULL | — |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT `now()` | — |

### 2.10 `report_schedules`

| Kolom | Tipe | Constraint | Ket. |
|-------|------|------------|------|
| `id` | `UUID` | **PK** | — |
| `name` | `TEXT` | NOT NULL | — |
| `cron` | `TEXT` | NOT NULL | cth `0 7 1 * *` bulanan |
| `recipients` | `JSONB` | NOT NULL | daftar email |
| `target_ids` | `UUID[]` | NULL | NULL = semua target |
| `format` | `TEXT` | NOT NULL DEFAULT `'pdf'` | — |
| `is_active` | `BOOLEAN` | NOT NULL DEFAULT true | — |
| `created_by` | `UUID` | **FK → `users.id`** ON DELETE SET NULL | — |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT `now()` | — |

### 2.11 `mail_queue`

| Kolom | Tipe | Constraint | Ket. |
|-------|------|------------|------|
| `id` | `BIGINT` | **PK** GENERATED ALWAYS AS IDENTITY | — |
| `to_email` | `TEXT` | NOT NULL | — |
| `subject` | `TEXT` | NOT NULL | — |
| `body_html` | `TEXT` | NULL | — |
| `body_text` | `TEXT` | NULL | — |
| `related_alert_id` | `BIGINT` | **FK → `alert_logs.id`** ON DELETE SET NULL | — |
| `related_report_id` | `UUID` | **FK → `reports.id`** ON DELETE SET NULL | — |
| `status` | `TEXT` | NOT NULL DEFAULT `'pending'` CHECK (`status` IN ('pending','sending','sent','failed')) | — |
| `attempts` | `INT` | NOT NULL DEFAULT 0 | — |
| `next_retry_at` | `TIMESTAMPTZ` | NULL | backoff |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT `now()` | — |

### 2.12 `mail_logs`

| Kolom | Tipe | Constraint | Ket. |
|-------|------|------------|------|
| `id` | `BIGINT` | **PK** GENERATED ALWAYS AS IDENTITY | — |
| `queue_id` | `BIGINT` | **FK → `mail_queue.id`** ON DELETE SET NULL | — |
| `to_email` | `TEXT` | NOT NULL | — |
| `status` | `TEXT` | NOT NULL CHECK (`status` IN ('sent','failed')) | — |
| `provider_msg_id` | `TEXT` | NULL | ID dari SMTP/provider |
| `error` | `TEXT` | NULL | — |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT `now()` | partition key bulanan |

### 2.13 `maintenance_windows`

| Kolom | Tipe | Constraint | Ket. |
|-------|------|------------|------|
| `id` | `UUID` | **PK** | — |
| `target_id` | `UUID` | **FK → `targets.id`** ON DELETE CASCADE, NULL | NULL + group NULL = global |
| `group_id` | `UUID` | **FK → `target_groups.id`** ON DELETE CASCADE, NULL | — |
| `starts_at` | `TIMESTAMPTZ` | NOT NULL | — |
| `ends_at` | `TIMESTAMPTZ` | NOT NULL CHECK (`ends_at` > `starts_at`) | — |
| `reason` | `TEXT` | NULL | — |
| `created_by` | `UUID` | **FK → `users.id`** ON DELETE SET NULL | — |

### 2.14 `audit_logs`

| Kolom | Tipe | Constraint | Ket. |
|-------|------|------------|------|
| `id` | `BIGINT` | **PK** GENERATED ALWAYS AS IDENTITY | — |
| `actor_id` | `UUID` | **FK → `users.id`** ON DELETE SET NULL | — |
| `action` | `TEXT` | NOT NULL | cth `target.create` |
| `entity` | `TEXT` | NOT NULL | cth `targets` |
| `entity_id` | `TEXT` | NULL | — |
| `diff` | `JSONB` | NULL | before/after |
| `ip` | `INET` | NULL | — |
| `created_at` | `TIMESTAMPTZ` | NOT NULL DEFAULT `now()` | partition key bulanan |

---

## 3. Relasi Antar Tabel

- `users` 1—N `target_groups` (created_by), `targets` (created_by), `alert_rules`, `report_schedules`, `reports`, `maintenance_windows`; 1—N `incidents` (ack_by); 1—N `audit_logs` (actor).
- `target_groups` self-ref 1—N (`parent_id`); 1—N `targets`; 1—N `alert_rules` (scope=group); 1—N `maintenance_windows`.
- `targets` 1—1 `target_credentials`; 1—N `checks`; 1—N `incidents`; 1—N `alert_rules` (scope=target); 1—N `maintenance_windows`.
- `alert_rules` 1—N `alert_logs`, 1—N `incidents` (opened_by_rule).
- `incidents` 1—N `alert_logs`.
- `report_schedules` 1—N `reports`.
- `reports` 1—N `mail_queue` (related_report); `alert_logs` 1—N `mail_queue`; `mail_queue` 1—N `mail_logs`.

---

## 4. Diagram ER (ASCII)

```
                    +-------+
                    | users |
                    +---+---+
                        | 1
                        | N
  +---------------------+----------------------+
  |                     |                      |
  v                     v                      v
+--------------+  +----------+  +------------------+
|target_groups |  | targets  |  | report_schedules |
+---+----------+  +----+-----+  +--------+---------+
  | 1 ^ N parent|       | 1             | 1
  |             | N     | N             | N
  v             v       v               v
 (grup)     (target) +------+      +---------+
  |             |    |checks|      | reports |
  |             |    +------+      +----+----+
  |             |       :               | 1
  |             |       : N             | N
  |             v       v               v
  |      +------------+          +------------+
  |      | incidents  |          | mail_queue |
  |      +-----+------+          +-----+------+
  |            | 1                     | 1
  |            | N                     | N
  |            v                       v
  |      +------------+          +------------+
  +----->| alert_rules|          | mail_logs  |
  |      +-----+------+          +------------+
  |            | 1
  |            | N
  |            v
  |      +------------+
  |      | alert_logs |
  |      +------------+
  |  +------------------+
  +->|maintenance_windows|
     +------------------+
  +------------------+
  |target_credentials| 1-1 ke targets
  +------------------+
  +------------+
  | audit_logs | N-1 ke users
  +------------+
```

---

## 5. DDL Inti (ringkas)

```sql
CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS citext;

-- checks hypertable
CREATE TABLE checks (
  time TIMESTAMPTZ NOT NULL,
  target_id UUID NOT NULL,
  status TEXT NOT NULL,
  latency_ms INT,
  http_code SMALLINT,
  error TEXT,
  region TEXT DEFAULT 'local',
  PRIMARY KEY (time, target_id)
);
SELECT create_hypertable('checks', 'time', chunk_time_interval => INTERVAL '1 day');
ALTER TABLE checks SET (timescaledb.compress,
  timescaledb.compress_segmentby = 'target_id',
  timescaledb.compress_orderby = 'time DESC');
SELECT add_compression_policy('checks', INTERVAL '7 days');

-- partisi bulanan native untuk log
CREATE TABLE alert_logs (
  id BIGINT GENERATED ALWAYS AS IDENTITY,
  rule_id UUID, incident_id UUID, target_id UUID,
  channel TEXT NOT NULL, payload JSONB,
  status TEXT NOT NULL, error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);
-- buat partisi per bulan: alert_logs_y2026m10, dst.
-- pola sama untuk mail_logs, audit_logs.
```

---

## 6. Index

```sql
-- checks: query per target + rentang waktu (paling sering)
CREATE INDEX ON checks (target_id, time DESC);
-- agregat status
CREATE INDEX ON checks (target_id, status, time DESC);
-- insiden open per target
CREATE INDEX ON incidents (target_id, status) WHERE resolved_at IS NULL;
-- alert_logs per incident/target
CREATE INDEX ON alert_logs (incident_id, created_at DESC);
CREATE INDEX ON alert_logs (target_id, created_at DESC);
-- mail_queue worker
CREATE INDEX ON mail_queue (status, next_retry_at) WHERE status IN ('pending','failed');
-- maintenance lookup
CREATE INDEX ON maintenance_windows (target_id, starts_at, ends_at);
CREATE INDEX ON maintenance_windows (group_id, starts_at, ends_at);
-- audit
CREATE INDEX ON audit_logs (entity, entity_id, created_at DESC);
CREATE INDEX ON audit_logs (actor_id, created_at DESC);
```

> FK dari `checks` ke `targets`: bila ingest >5k row/s, lepas FK fisik, ganti validasi di aplikasi + job orphan-cleanup. Di bawah itu FK aman.

---

## 7. Partisi Time-Series

| Tabel | Metode | Interval chunk/partisi | Alasan |
|-------|--------|------------------------|--------|
| `checks` | Timescale hypertable | chunk 1 hari | write-append, query rentang pendek |
| `alert_logs` | partisi native RANGE bulanan | 1 bulan | volume sedang, drop mudah |
| `mail_logs` | partisi native RANGE bulanan | 1 bulan | sama |
| `audit_logs` | partisi native RANGE bulanan | 1 bulan | sama |

Kompresi `checks` aktif setelah 7 hari (segmentby `target_id`). Continuous aggregate untuk rollup harian (lihat §9).

---

## 8. Retensi

| Data | Retensi | Mekanisme |
|------|---------|-----------|
| `checks` mentah | 90 hari | `add_retention_policy('checks', INTERVAL '90 days')` |
| `checks` agregat harian (`checks_daily`) | 1 tahun | continuous aggregate + retention 365 hari |
| `alert_logs` | 1 tahun | drop partisi >12 bulan |
| `mail_logs` | 1 tahun | drop partisi >12 bulan |
| `audit_logs` | 1 tahun | drop partisi >12 bulan |
| `incidents`, `reports` | permanen (atau 3 tahun) | metadata kecil, jangan hapus (butuh SLA historis) |
| `mail_queue` sent/failed | 30 hari | cleanup job |

```sql
SELECT add_retention_policy('checks', INTERVAL '90 days');
-- cagg harian retensi 1 tahun
CREATE MATERIALIZED VIEW checks_daily
WITH (timescaledb.continuous) AS
SELECT time_bucket('1 day', time) AS day, target_id,
  count(*) AS total,
  count(*) FILTER (WHERE status = 'up') AS up_cnt
FROM checks GROUP BY 1, 2;
SELECT add_retention_policy('checks_daily', INTERVAL '365 days');
```

---

## 9. Contoh Query SLA Uptime %

```sql
-- 9a. SLA per target, 30 hari terakhir, exclude maintenance
SELECT
  t.id, t.name,
  COUNT(c.*) AS total_checks,
  COUNT(*) FILTER (WHERE c.status = 'up') AS up_checks,
  ROUND(100.0 * COUNT(*) FILTER (WHERE c.status = 'up') / NULLIF(COUNT(*),0), 3) AS uptime_pct,
  PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY c.latency_ms) AS p95_latency_ms
FROM targets t
LEFT JOIN checks c ON c.target_id = t.id
  AND c.time >= now() - INTERVAL '30 days'
  AND NOT EXISTS (
    SELECT 1 FROM maintenance_windows m
    WHERE (m.target_id = t.id OR m.group_id = t.group_id OR (m.target_id IS NULL AND m.group_id IS NULL))
      AND c.time BETWEEN m.starts_at AND m.ends_at
  )
WHERE t.is_active
GROUP BY t.id, t.name
ORDER BY uptime_pct;
```

```sql
-- 9b. SLA dari agregat harian (cepat, untuk rentang 1 tahun)
SELECT target_id,
  SUM(total) AS total_checks,
  SUM(up_cnt) AS up_checks,
  ROUND(100.0 * SUM(up_cnt) / NULLIF(SUM(total),0), 3) AS uptime_pct
FROM checks_daily
WHERE day >= CURRENT_DATE - INTERVAL '90 days'
GROUP BY target_id;
```

```sql
-- 9c. MTTR per target, 90 hari
SELECT target_id,
  COUNT(*) AS incidents,
  ROUND(AVG(EXTRACT(EPOCH FROM (resolved_at - opened_at))/60),1) AS mttr_min
FROM incidents
WHERE opened_at >= now() - INTERVAL '90 days'
  AND resolved_at IS NOT NULL
GROUP BY target_id;
```
