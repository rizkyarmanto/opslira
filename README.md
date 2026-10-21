# Opslira

Opslira adalah aplikasi monitoring infrastruktur dan layanan internal (single pane of glass).
Satu dasbor untuk memantau kesehatan, respons, dan SLA dari tujuh kategori target, lengkap
dengan notifikasi insiden dan laporan berkala dalam format PDF dan Excel.

- **Backend:** Laravel 11 (PHP 8.3), monolit modular di `app/Modules/`
- **Frontend:** React 18 + TypeScript lewat Inertia.js, Tailwind CSS, framer-motion, ECharts
- **Database:** PostgreSQL 16 (partisi bulanan pada `monitoring_logs`)
- **Cache dan antrean:** Redis 7
- **PDF:** Gotenberg 8 (headless Chromium)
- **Runner periodik:** biner Go kecil di `go/`

## Tujuh kategori target dan adapter probe

Konfigurasi target bersifat polimorfik: satu tabel `monitoring_targets` dengan kolom
`config_payload` (JSONB) yang divalidasi per `target_type`. Logika probe berada di balik
`ProbeAdapterInterface`; menambah kategori berarti menambah adapter, bukan mengubah scheduler.

| Kategori (`target_type`) | Protokol (`protocol_type`) | Adapter |
|---|---|---|
| NETWORK_DEVICE | ICMP_SNMP | `NetworkSnmpProbeAdapter` |
| WEB_APP | HTTP_HEALTH | `HttpHealthProbeAdapter` |
| DOMAIN | WHOIS_SSL | `DomainSslProbeAdapter` |
| DATABASE | DB_NATIVE_PING | `DatabasePingProbeAdapter` |
| MAIL_SERVER | SMTP_DNSBL | `MailServerProbeAdapter` |
| S3_STORAGE | S3_HEAD | `S3BucketProbeAdapter` |
| BESZEL_SERVER | BESZEL_API | `BeszelApiProbeAdapter` |

## Modul aplikasi (`app/Modules/`)

| Modul | Peran |
|---|---|
| Dashboard | NOC Overview, halaman Alerts, dan dasbor per domain (network, apps-domains-db, mail-s3-beszel) |
| Targets | CRUD target, validasi `config_payload` per tipe, enkripsi kredensial (AES-256-GCM) |
| Probes | Adapter probe, eksekusi probe, pelacakan state target, dan endpoint internal untuk runner |
| Incidents | `AlertNotificationService` untuk notifikasi perubahan status (UP/DEGRADED/DOWN/MAINTENANCE) |
| Reporting | Jadwal laporan, builder PDF (Gotenberg) dan Excel, log pengiriman email, endpoint dispatch internal |
| Users | Manajemen akun dan peran (SUPER_ADMIN, SYSADMIN, DEVELOPER, MANAGER) |

## Fitur utama

- **NOC Overview** (`/dashboard/noc`): ringkasan seluruh target dalam satu layar.
- **Alerts** (`/dashboard/alerts`): daftar insiden dan perubahan status terkini.
- **Dasbor per domain**: `/dashboard/network`, `/dashboard/apps-domains-db`, `/dashboard/mail-s3-beszel`.
- **Laporan** (`/reports/schedules`): jadwal DAILY/WEEKLY/MONTHLY, ekspor PDF (via Gotenberg) dan
  Excel (XLSX), unduhan file, serta log pengiriman email di `/reports/email-logs`.
- **Notifikasi insiden**: Email (penerima di `ALERT_RECIPIENTS`), Telegram
  (`TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`), dan Slack (`SLACK_WEBHOOK_URL`).

## Arsitektur scheduler dan antrean

`routes/console.php` sengaja tidak mendaftarkan schedule apa pun. Seluruh pemicu periodik
dimiliki biner Go runner di `go/`:

- Tiap `RUNNER_TICK_SECONDS` (default 15 detik): runner klaim target yang jatuh tempo lalu
  mengeksekusi probe lewat endpoint internal PHP (container web, FPM).
- Tiap `REPORT_TICK_SECONDS` (default 60 detik): runner memicu pembuatan laporan yang jatuh tempo.
- Harian pada jam `ROLLUP_AT` (00:05) dan `PRUNE_AT` (02:00) waktu `RUNNER_TIMEZONE`
  (default Asia/Jakarta): rollup ringkasan harian dan pembuatan/pemangkasan partisi.
- Concurency probe dikontrol `PROBE_CONCURRENCY` (default 8).

Runner berbicara ke Laravel melalui endpoint internal ber-prefix `/internal`
(`/internal/probes/claim`, `/internal/probes/{target}`, `/internal/reports/dispatch`,
`/internal/maintenance/rollup`, `/internal/maintenance/prune`) dan mengautentikasi diri dengan
header `X-Opslira-Runner` berisi nilai `OPSLIRA_RUNNER_TOKEN`.

Antrean Laravel memakai tiga lane: `alerts`, `probes`, dan `reports`. Pada konfigurasi default
(`QUEUE_CONNECTION=sync` di `.env.example`), job berjalan inline dari endpoint runner sehingga
tidak perlu worker queue terpisah. Jika beralih ke `QUEUE_CONNECTION=redis`, jalankan
`php artisan queue:work` untuk lane-lane tersebut.

```
Go runner (tick) -> POST /internal/probes/claim -> ProbeAdapter (PHP) -> PostgreSQL + Redis state
                                                   |
                                     state change -> AlertNotificationService -> Email/Telegram/Slack
Go runner (report tick) -> POST /internal/reports/dispatch -> Gotenberg PDF + Excel -> Email
Runner harian -> /internal/maintenance/rollup dan /internal/maintenance/prune
```

## Deploy baremetal

### Prasyarat

- PHP 8.3 (lihat `composer.json`, `"php": "^8.3"`) dengan ekstensi: `pdo_pgsql`, `pgsql`,
  `bcmath`, `gd`, `intl`, `zip`, `opcache`, `pcntl`, `sockets`. Redis memakai `predis/predis`
  sehingga ekstensi redis native tidak wajib.
- Composer 2
- Node.js 20+ dan npm
- PostgreSQL 16
- Redis 7
- Go 1.22+ (untuk membangun runner periodik; opsional bila memakai cron manual)
- Gotenberg 8 (opsional; untuk ekspor PDF. Tanpa itu, fitur PDF nonaktif)

### Langkah instalasi

```bash
git clone <url-repo> opslira
cd opslira

composer install --no-dev --optimize-autoloader
npm install
npm run build

cp .env.example .env
php artisan key:generate

php artisan migrate --force
php artisan db:seed --force
php artisan storage:link   # bila perlu; laporan disimpan di storage/app/reports
```

Isi nilai berikut di `.env`:

- `APP_KEY` (terisi `key:generate`), `APP_ENCRYPTION_KEY` (opsional; jika kosong, kredensial
  memakai `APP_KEY`), `APP_URL`, `APP_TIMEZONE`
- `DB_HOST`, `DB_PORT`, `DB_DATABASE`, `DB_USERNAME`, `DB_PASSWORD`
- `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`
- `GOTENBERG_URL` (mis. `http://127.0.0.1:3000`)
- `OPSLIRA_RUNNER_TOKEN` (secret bersama untuk endpoint internal)
- `ALERT_RECIPIENTS`, `MAIL_*`, `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`, `SLACK_WEBHOOK_URL`

### Web server (Nginx + PHP-FPM)

Arahkan root ke folder `public/`. Contoh blok server:

```nginx
server {
    listen 80;
    server_name opslira.internal;

    root /path/ke/opslira/public;
    index index.php index.html;

    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }

    location ~ \.php$ {
        fastcgi_pass unix:/run/php/php8.3-fpm.sock;   # atau 127.0.0.1:9000
        fastcgi_index index.php;
        fastcgi_param SCRIPT_FILENAME $realpath_root$fastcgi_script_name;
        include fastcgi_params;
        # Pembuatan laporan bisa berjalan beberapa menit.
        fastcgi_read_timeout 300s;
        fastcgi_send_timeout 300s;
    }

    location ~ /\.(?!well-known).* {
        deny all;
    }
}
```

### Cron dan worker

```bash
* * * * * cd /path/ke/opslira && php artisan schedule:run >> /dev/null 2>&1
```

Catatan akurat: `routes/console.php` tidak mendaftarkan schedule apa pun, sehingga
`schedule:run` aman dijalankan tiap menit tetapi tidak mengeksekusi probe atau laporan.
Pemicu periodik (klaim target, probe, dispatch laporan, rollup, prune) dimiliki Go runner di
`go/`. Untuk kerja periodik di baremetal, bangun dan jalankan runner tersebut:

```bash
cd go
go build -o opslira-runner .
OPSLIRA_BASE_URL=http://127.0.0.1 \
OPSLIRA_RUNNER_TOKEN=<token-anda> \
RUNNER_TIMEZONE=Asia/Jakarta \
./opslira-runner
```

Atau panggil perintah artisan secara manual sesuai kebutuhan:
`php artisan monitoring:dispatch-probes`, `monitoring:rollup`,
`monitoring:dispatch-reports`, `monitoring:prune-partitions`,
`monitoring:create-partition YYYY-MM`.

Queue worker hanya diperlukan bila memakai antrean nyata. Dengan `QUEUE_CONNECTION=sync`
(default) tidak perlu worker. Bila `QUEUE_CONNECTION=redis`:

```bash
php artisan queue:work redis --queue=alerts,probes,reports --tries=1 --timeout=15
```

### Verifikasi

Buka `http://<host>/login` lalu masuk dengan kredensial seed bawaan:
`admin@opslira.local` / `ChangeMe!2026`.

## Deploy Docker

`docker-compose.yml` menjalankan lima service:

- `monitoring-web` (port `:8080`): Nginx + PHP-FPM untuk aplikasi Laravel dan eksekusi probe
- `monitoring-runner`: biner Go yang memiliki seluruh kerja periodik
- `monitoring-gotenberg`: Gotenberg 8 untuk PDF
- `monitoring-postgres`: PostgreSQL 16
- `monitoring-redis`: Redis 7

Langkah:

```bash
cp .env.example .env
# Isi APP_KEY, APP_ENCRYPTION_KEY, DB_PASSWORD, dan OPSLIRA_RUNNER_TOKEN di .env.
# .env.example sudah mengarah ke nama service compose:
#   DB_HOST=monitoring-postgres, REDIS_HOST=monitoring-redis,
#   GOTENBERG_URL=http://monitoring-gotenberg:3000

docker compose up -d --build
docker compose exec monitoring-web php artisan migrate --force
docker compose exec monitoring-web php artisan db:seed --force
```

`OPSLIRA_RUNNER_TOKEN` pada `.env` harus sama dengan nilai yang dipakai service
`monitoring-runner` agar endpoint internal menerima panggilan runner.

Buka <http://localhost:8080> dan masuk dengan `admin@opslira.local` / `ChangeMe!2026`.

## Varian ringan: docker-compose.lite.yml

Tersedia varian hemat di `docker-compose.lite.yml` yang berjalan tanpa service PostgreSQL
dan Redis bawaan. Database diarahkan ke Postgres eksternal (default `host.docker.internal`;
di Linux ganti IP/hostname host), sedangkan cache, sesi, dan antrean memakai `file`/`sync`.
Cocok untuk demo atau lingkungan kecil. Perhatikan bahwa eksekusi probe penuh tetap
membutuhkan Redis (state target dibaca lewat Redis), jadi varian ini cocok untuk UI dan
laporan tanpa probe aktif. Rincian konfigurasi dan batasannya ada di komentar dalam
`docker-compose.lite.yml`.

## Perintah artisan

```bash
php artisan monitoring:dispatch-probes        # kirim job probe target yang jatuh tempo
php artisan monitoring:rollup                 # agregasi ringkasan harian
php artisan monitoring:dispatch-reports       # kirim laporan terjadwal
php artisan monitoring:prune-partitions       # buat dan pangkas partisi monitoring_logs
php artisan monitoring:create-partition YYYY-MM   # buat satu partisi manual
```

## Dokumentasi

- `AGENTS.md`: panduan operasional untuk agen kode
- `docs/gemini/`: arsitektur, ERD dan DDL, PRD
- `docs/referensi-design.md`: spesifikasi UI dan UX
- `docs/Design.md`: design tokens
