# Arsitektur Aplikasi Web — Monitoring Internal

> Bahasa: Indonesia. Stack: FE Next.js, BE NestJS/Laravel, PostgreSQL+TimescaleDB, Redis+BullMQ, Worker Poller/Alert/Report/Mail, ingest Beszel API.

## 1. Stack Final + Alasan

| Lapisan | Pilihan | Alasan |
|---|---|---|
| FE | Next.js (React + TS) | SSR/SSG untuk dasbor cepat, routing bawaan, mudah deploy Docker, integrasi API/WS baik |
| BE utama | NestJS (Node.js + TS) | Struktur modular, DI, dukungan BullMQ/Redis/schedule bawaan, satu bahasa dengan FE, cocok untuk worker |
| BE alternatif | Laravel (PHP) | Opsional bila tim kuat PHP; Eloquent + Queue + Scheduler matang. Pilih satu saja, jangan dua-duanya |
| DB utama | PostgreSQL + TimescaleDB | Relasional (user, target, jadwal) + time-series (hasil check, metrik) dalam satu DB, hypertable + kompresi + retensi otomatis |
| Cache + Queue | Redis + BullMQ | Cache status terakhir, job queue dengan retry/backoff/delay/repeat, rate-limit |
| Worker | Node.js (NestJS standalone / BullMQ worker) | Poller, Alert, Report, Mail jalan async, scalable horizontal |
| Ingest metrik | Beszel API (agent + hub) | Ringan, sudah ada CPU/RAM/disk/network/container, tinggal pull via API |
| Protokol check | ICMP, SNMP, HTTP(S), TCP, DNS, TLS, SMTP/IMAP, SQL ping, S3 API, Beszel API | Cakup network + service + cert + mail + DB + storage + infra. Telnet di-drop (tidak aman) |
| Deploy | Docker Compose di VPS internal | Sederhana, reproducible, cukup untuk internal; tanpa K8s |
| Reverse proxy | Nginx / Caddy | TLS terminate, routing FE/BE, basic WAF/rate-limit |

Keputusan kunci:
- Satu DB time-series (TimescaleDB), bukan Influx/Prometheus terpisah — kurangi operasional.
- Satu Redis untuk cache + queue — kurangi komponen.
- BE NestJS direkomendasikan; Laravel hanya bila tim backend PHP.
- Telnet dibuang: plaintext, diganti SSH/TCP check bila perlu.

## 2. Diagram Arsitektur ASCII

```
                        +----------------+
                        |    Client      |
                        | Browser (HTTPS)|
                        +-------+--------+
                                |
                                v
                        +-------+--------+
                        | Reverse Proxy  |
                        | Nginx / Caddy  |
                        | :80 / :443 TLS |
                        +---+--------+---+
                            |        |
                +-----------+        +-----------+
                |                                |
                v                                v
     +----------+---------+           +----------+---------+
     |   FE Next.js       |           |   BE API Service   |
     |   :3000            |<--------->|   NestJS / Laravel |
     | SSR + Dashboard    |  REST/WS  |   :8000 / :3001    |
     +--------------------+           +-----+----+---+----+
                                            |    |   |
                          +-----------------+    |   +-----------------+
                          |                      |                     |
                          v                      v                     v
                 +--------+-------+    +---------+--------+   +--------+-------+
                 | PostgreSQL     |    | Redis + BullMQ   |   | Workers        |
                 | + TimescaleDB  |    | :6379            |   | (Node.js)      |
                 | :5432          |    |                  |   |                |
                 | - tabel relasi |    | queues:          |   | - Poller       |
                 | - hypertable   |    |  checks          |   | - Alert        |
                 |   check_result |<---|  alerts          |<-->| - Report       |
                 |   metrics      |    |  reports         |   | - Mail         |
                 +-------+--------+    |  mail            |   +---+---+---+----+
                         ^         |    +------------------+       |   |   |    |
                         |         |                               |   |   |    |
                         |         +-----------+-------------------+   |   |    |
                         |                     |                       |   |    |
                 +-------+--------+    +-------+-------+     +---------+   |    +----------+
                 | Beszel Hub     |    | Target check  |     | SMTP/IMAP relay | | S3 / SQL |
                 | (ingest API)   |    | ICMP/SNMP/    |     | kirim alert &   | | DB target|
                 | <- Beszel agent|    | HTTP/TCP/DNS/ |     | laporan         | +----------+
                 +----------------+    | TLS/SQL/S3    |     +-----------------+
                                       +---------------+
```

Aliran baca: Client -> Proxy -> FE/BE -> DB/Redis/Worker -> Target eksternal.

## 3. Penjelasan Tiap Komponen

### 3.1 API Service (BE)
- Framework: NestJS (atau Laravel). Satu pintu REST (+ WebSocket/SSE untuk status live).
- Tanggung jawab:
  - AuthN/AuthZ, RBAC, API key untuk agent.
  - CRUD: target/monitor, grup, jadwal, maintenance window, user, channel notifikasi, template laporan.
  - Validasi definisi check (protokol, interval, timeout, threshold, retry).
  - Enqueue job check ke BullMQ, baca status terakhir dari Redis, riwayat dari TimescaleDB.
  - Endpoint ringkas: `POST /targets`, `GET /targets/:id/status`, `GET /metrics?target=&from=&to=`, `POST /reports/run`, dsb.
- Stateless; skala horizontal di belakang proxy.

### 3.2 Poller (Worker)
- Konsumsi queue `checks` (repeat per interval target: 30 dtk / 1 mnt / 5 mnt).
- Satu worker menangani semua protokol via modul check:
  - ICMP ping (latency, loss), TCP connect, HTTP(S) (status, body match, latency), DNS resolve, TLS expiry, SNMP get/walk, SMTP (banner+STARTTLS+login), IMAP (login+select), SQL ping (`SELECT 1`), S3 API (`HeadBucket`/`ListObjects`), Beszel API (lihat 3.6).
- Concurrency dibatasi per protokol (mis. SNMP lambat, jangan blokir HTTP).
- Timeout per check (default 10 dtk), retry cepat maks 1–2x sebelum tandai DOWN agar hindari false positive.
- Output: tulis ke `check_results` (TimescaleDB) + update Redis `target:{id}:last`.

### 3.3 Alert (Worker)
- Konsumsi event hasil check / queue `alerts`.
- Rule engine: threshold (mis. 3 gagal beruntun → DOWN), flapping detection, dependensi (host down → child muted), maintenance window (mute).
- Aksi: buat/ubah incident, enqueue ke `mail` + webhook (Telegram/Slack opsional).
- State incident di Postgres (`incidents`), status: firing → acknowledged → resolved.
- Eska­lasi: bila tak di-ack N menit → kirim ke level atas.

### 3.4 Report (Worker)
- Konsumsi queue `reports` (cron harian/mingguan/bulanan per BullMQ repeatable).
- Ambil agregat TimescaleDB: uptime %, MTTR/MTBF, insiden, SLA per grup/target.
- Generate PDF/Excel (Puppeteer / ExcelJS) + simpan ke storage lokal/S3 internal.
- Daftarkan hasil di tabel `reports`, lalu enqueue job kirim ke Mail worker.

### 3.5 Mail (Worker)
- Konsumsi queue `mail`: alert + laporan terjadwal.
- Provider: SMTP internal / relay (Postfix / Google Workspace / Resend opsional).
- Template HTML+text, lampiran PDF laporan, throttling agar tidak spam (digest per 5 menit bila flap).
- Log pengiriman di DB untuk audit.

### 3.6 Beszel Ingest (Worker/Cron)
- Beszel agent terpasang di server yang dipantau → lapor ke Beszel Hub.
- Ingest worker pull Beszel Hub API berkala (mis. tiap 60 dtk): CPU, RAM, disk, net, container, uptime.
- Normalisasi → tulis ke hypertable `beszel_metrics`; korelasikan dengan `target_id` via mapping tabel.
- Bila Hub down: backoff + tandai gap, jangan tandai semua host DOWN hanya dari ingest gagal.

## 4. Alur Check -> Simpan -> Alert

```
Scheduler (BullMQ repeat)
  |  job { targetId, protocol, config }
  v
Poller ambil job
  |  eksekusi protokol (timeout + retry 1x)
  v
Tulis check_results -> TimescaleDB hypertable
  |  { ts, target_id, protocol, status UP/DOWN/DEGRADED, latency_ms, detail JSONB }
  v
Update Redis target:{id}:last + publish event
  v
Alert worker evaluasi rule
  |- OK / 1x gagal -> catat saja
  |- N gagal beruntun + bukan maintenance -> buat incident (firing)
  v
Enqueue mail/webhook -> kirim notifikasi
  v
User ack via FE/API -> incident acknowledged -> auto resolve saat UP kembali
```

Contoh status Redis: `target:42:last = { status:"DOWN", latency_ms:null, ts:"...", consecutiveFails:3 }` TTL 2x interval.

Skema inti:

```sql
targets(id, name, group_id, protocol, config JSONB, interval_s, timeout_ms, enabled)
check_results(ts TIMESTAMPTZ, target_id, protocol, status, latency_ms, detail JSONB) -- hypertable
beszel_metrics(ts, host_id, target_id, cpu, mem, disk, net_in, net_out) -- hypertable
incidents(id, target_id, status, started_at, ack_at, resolved_at, summary)
report_schedules(id, name, cron, recipients, template)
```

## 5. Alur Schedule Report -> Generate -> Email Queue

```
BullMQ repeat (cron dari report_schedules)
  |  job { scheduleId, period }
  v
Report worker
  |  1. query agregat TimescaleDB (uptime, insiden, top slow)
  |  2. render template -> PDF/Excel
  |  3. simpan file + row reports(status=ready)
  v
Enqueue mail job { to, subject, body, attachment }
  v
Mail worker kirim via SMTP relay (retry 3x, backoff)
  v
Update reports(status=sent/failed) + log mail_deliveries
```

Idempotency key: `{scheduleId}:{period}` agar job ganda tidak kirim email dua kali.

## 6. Keamanan

- RBAC: role `admin / operator / viewer`. Admin kelola user+secrets, operator kelola target+ack incident, viewer baca saja. Enforcement di guard BE + filter query per grup.
- Secrets: simpan di env file / Docker secrets / Vault; tak pernah di repo. Kredensial SNMP community, DB password untuk SQL ping, S3 key, SMTP pass, Beszel API token — semua via secrets manager, rotasi berkala.
- TLS: HTTPS wajib di proxy (Caddy auto-TLS / Nginx + cert internal). Komunikasi BE->DB/Redis di network Docker internal, tanpa expose port ke publik. Check TLS verifikasi chain; peringatan expiry H-30/H-7/H-1.
- Proteksi API: JWTHttpOnly / session, rate-limit login + API, audit log (siapa ubah target/matikan alert), validasi input protokol (cegah SSRF — batasi target ke allowlist/IP internal bila perlu).
- Isolasi: worker poller jalan dengan user non-root + capability minimal (ping butuh raw socket — gunakan `ping` setcap atau modul TCP fallback). SNMP v3 authPriv bila bisa.
- Backup: dump Postgres harian + snapshot volume; retensi backup 30 hari.

## 7. Deployment Docker Compose + Topologi VPS Internal

Topologi:

```
Internet/VPN (wireguard/tailscale opsional)
  |
[ VPS internal 4-8 vCPU / 16 GB ]
  |  UFW: buka 80/443 (+22 SSH kunci saja)
  |- proxy (Nginx/Caddy) :80/:443 -> fe:3000, be:8000
  |- fe (Next.js standalone)
  |- be (NestJS/Laravel API)
  |- worker-poller (replica 2-3)
  |- worker-alert / worker-report / worker-mail (1 tiap jenis)
  |- postgres:15-timescale (volume pgdata)
  |- redis:7-alpine (volume redisdata, AOF on)
  |- beszel-hub (atau hub eksternal yang sudah ada)
  |- target internal: server/db/switch via SNMP/ICMP aman di LAN/VPN
```

Contoh `docker-compose.yml` (ringkas):

```yaml
services:
  proxy:
    image: caddy:2-alpine
    ports: ["80:80", "443:443"]
    volumes: [./Caddyfile:/etc/caddy/Caddyfile, caddydata:/data]
    depends_on: [fe, be]
  fe:
    build: ./fe
    environment: { NEXT_PUBLIC_API_URL: https://mon.internal/api }
  be:
    build: ./be
    env_file: [.env]
    depends_on: [postgres, redis]
  poller:
    build: ./be
    command: ["node", "dist/worker-poller.js"]
    deploy: { replicas: 2 }
    env_file: [.env]
    depends_on: [redis, postgres]
  alert:
    build: ./be
    command: ["node", "dist/worker-alert.js"]
    env_file: [.env]
  report:
    build: ./be
    command: ["node", "dist/worker-report.js"]
    env_file: [.env]
  mail:
    build: ./be
    command: ["node", "dist/worker-mail.js"]
    env_file: [.env]
  postgres:
    image: timescale/timescaledb:latest-pg15
    volumes: [pgdata:/var/lib/postgresql/data]
  redis:
    image: redis:7-alpine
    command: ["redis-server", "--appendonly", "yes"]
    volumes: [redisdata:/data]
volumes: { pgdata:, redisdata:, caddydata: }
```

Catatan: tanpa K8s; scale via `replicas` poller + pisah VPS bila beban naik.

## 8. Skalabilitas & Retensi

- Skalabilitas:
  - Poller stateless → tambah replica; partisi queue per protokol/grup bila ribuan target.
  - Cache Redis untuk status terakhir; dasbor tak pukul DB tiap detik (poll 10–30 dtk / WS push).
  - TimescaleDB continuous aggregates untuk grafik (1 mnt → 1 jam → 1 hari).
  - Pisah worker berat (report PDF) dari poller agar check tak tersendat.
- Retensi (disarankan awal):
  - `check_results` mentah: 30 hari, lalu downsample ke 5-menit/1-jam.
  - Agregat jam: 12 bulan; agregat harian: 24 bulan (untuk SLA tahunan).
  - `beszel_metrics` mentah: 14 hari; agregat jam: 6 bulan.
  - Laporan PDF: 12 bulan di storage internal.
  - Gunakan TimescaleDB compression + retention policy otomatis.

## 9. Failure Handling / Retry

| Gagal | Penanganan |
|---|---|
| Check timeout sesekali | Retry 1x cepat; DOWN hanya setelah N gagal beruntun (default 3) |
| Poller crash | BullMQ retry 3x backoff eksponensial; job lain tak hilang (persist di Redis) |
| Redis down | Worker pause + backoff; BE baca last-known dari DB; alert Redis-down ke admin via jalur cadangan |
| DB down | Poller buffer ke Redis stream / disk sementara, replay saat DB pulih; API kembalikan 503 + cache |
| Mail gagal | Retry 3x + DLQ `mail-failed`; tampilkan di UI agar operator tahu notif macet |
| Report gagal | Retry 1x; tandai failed; kirim ulang manual dari UI |
| Beszel Hub unreachable | Backoff 5 mnt; tandai gap metrik; jangan flap-kan status host |
| Flapping target | Cooldown alert (mis. 1 alert/15 mnt/target) + digest; auto-resolve + catat MTTR |
| SNMP credential salah | Tandai UNKNOWN (bukan DOWN), beri pesan jelas, notif sekali ke admin |
| Jam skewed | NTP/Chrony wajib di VPS; TimescaleDB pakai `timestamptz` server-side (`now()`) |

Prinsip: bedakan DOWN vs UNKNOWN; tiap retry ada batas + backoff + DLQ; semua kegagalan kirim tercatat dan terlihat di dasbor.
