# PRD — Aplikasi Monitoring Internal Kantor

**Versi:** 1.0
**Tanggal:** 15 Oktober 2026
**Status:** Draft
**Stack saran:** FE Next.js, BE NestJS/Laravel, PostgreSQL + TimescaleDB, Redis + BullMQ, Docker Compose
**Timezone standar:** Asia/Jakarta (WIB, UTC+7)

---

## 1. Tujuan & Latar Belakang

### 1.1 Latar belakang
Operasional kantor bergantung pada 7 kelompok objek: network internal, aplikasi (Prod/Staging/Dev), domain, database, mail server, S3, dan server Beszel. Saat ini tidak ada monitoring terpusat. Gangguan diketahui dari laporan manual/user. Tidak ada data SLA, MTTR, histori insiden, dan laporan rutin.

### 1.2 Tujuan
1. Monitoring terpusat 7 objek dengan cek otomatis terjadwal.
2. Deteksi dini gangguan via alerting Telegram/Discord/email.
3. Visibilitas kondisi via 4 dashboard.
4. Laporan harian/mingguan/bulanan PDF + Excel terkirim terjadwal.
5. Data SLA%, MTTR, uptime untuk evaluasi operasional.
6. Retensi data jelas: detail 90 hari, agregat 1 tahun.

### 1.3 Indikator keberhasilan awal
- 100% objek kritikal termonitor dalam 1 bulan setelah go-live.
- Alert terkirim < 2 menit setelah deteksi down.
- Laporan harian terkirim otomatis tiap hari pukul 07:00 WIB.

---

## 2. Stakeholder & User Role

| Stakeholder | Peran | Kepentingan |
|---|---|---|
| IT Manager | Owner / Approver | SLA, laporan, keputusan maintenance |
| Tim IT / NOC | Admin + Operator | Konfigurasi, respon insiden |
| Developer | Operator (terbatas) | Status App/DB S3 Staging/Dev |
| Manajemen | Viewer | Dashboard overview, laporan bulanan |
| Vendor/ISP (eksternal) | Non-user | Menerima eskalasi manual |

### 2.1 Matriks role

| Fitur | Admin | Operator | Viewer |
|---|---|---|---|
| Lihat dashboard (4 dashboard) | Ya | Ya | Ya |
| CRUD target monitor | Ya | Ya (tidak hapus objek kritikal*) | Tidak |
| CRUD user & role | Ya | Tidak | Tidak |
| Atur schedule check (30s/1m/5m) | Ya | Ya | Tidak |
| Atur schedule email laporan | Ya | Tidak | Tidak |
| Atur channel alert (Telegram/Discord/email) | Ya | Ya (uji kirim saja) | Tidak |
| Aktif/nonaktif maintenance mode | Ya | Ya | Tidak |
| Hapus data histori / ubah retensi | Ya | Tidak | Tidak |
| Lihat & ekspor laporan | Ya | Ya | Ya (hanya lihat + unduh) |
| Lihat audit log | Ya | Ya (read-only) | Tidak |

*Objek kritikal: ditandai flag `is_critical`. Hanya Admin boleh hapus. Operator hanya boleh edit/nonaktifkan sementara via maintenance mode.

### 2.2 Kebutuhan autentikasi
- Login username + password (hash bcrypt/argon2).
- Session timeout 30 menit idle.
- Opsional 2FA TOTP untuk Admin (fase 2, tidak wajib MVP).
- Semua aksi CRUD tercatat di audit log: siapa, kapan (WIB), apa, sebelum/sesudah.

---

## 3. Scope — 7 Objek Monitor

### Objek 1: Network (Router, Switch, AP, Server Internal)
- Metode: ICMP ping + SNMP (v2c/v3) + TCP port check.
- Parameter per target: IP/hostname, tipe perangkat, community SNMP / kredensial SNMPv3, OID profil (CPU, memori, interface status, uptime perangkat), port TCP (mis. 22, 443, 3389).
- Metrik: status up/down, latency ms, packet loss %, CPU %, memori %, interface up/down, uptime perangkat.
- Interval default: 30 detik (ping/TCP), 1 menit (SNMP).

### Objek 2: Aplikasi Prod/Staging/Dev (via HTTPS)
- Metode: HTTP(S) GET/HEAD ke URL + ukur latency + cek keyword + cek TLS.
- Parameter: URL, method, expected status (mis. 200), keyword wajib ada di body, threshold latency (mis. >2000 ms = degraded), peringatan expiry TLS H-30/H-14/H-7.
- Metrik: status UP/DEGRADED/DOWN, HTTP status code, latency ms, keyword match ya/tidak, sisa hari TLS.
- Interval default: 1 menit.

### Objek 3: Domain (via DNS + WHOIS)
- Metode: query DNS (A/AAAA/MX/NS) + cek WHOIS/RDAP expiry.
- Parameter: nama domain, nameserver acuan, expiry warning H-60/H-30/H-7.
- Metrik: resolve OK/gagal, waktu resolve ms, expiry date, sisa hari.
- Interval default: 5 menit (DNS), 1x sehari (WHOIS expiry).

### Objek 4: Database Prod/Staging/Dev (via TCP + test query)
- Metode: TCP connect ke host:port + test query ringan (`SELECT 1`).
- Parameter: tipe (PostgreSQL/MySQL/dll), host, port, kredensial read-only khusus monitoring, query test, threshold waktu query.
- Metrik: reachable ya/tidak, waktu konek ms, waktu query ms, error message.
- Interval default: 1 menit.
- Keamanan: akun DB khusus monitoring, hak minimal, password tersimpan terenkripsi.

### Objek 5: Mail Server (SMTP/IMAP/queue/blacklist/TLS)
- Metode: cek port SMTP (25/587)/IMAP (143/993) + TLS cert + ukuran queue + cek blacklist (DNSBL).
- Parameter: hostname, port, STARTTLS/SSL, kredensial test IMAP opsional, threshold queue, daftar DNSBL (mis. Spamhaus, Barracuda).
- Metrik: status tiap protokol, latency, sisa hari TLS, jumlah queue, status listed/clean per DNSBL.
- Interval default: 1 menit (port/TLS), 5 menit (queue/blacklist).

### Objek 6: S3 (via API HeadBucket/List)
- Metode: AWS S3 API `HeadBucket` + `ListObjectsV2` (max 1 key).
- Parameter: endpoint (AWS/minio), bucket, region, access key/secret, threshold latency.
- Metrik: HeadBucket OK/gagal, List OK/gagal, latency ms, error code (NoSuchBucket/AccessDenied/dll).
- Interval default: 5 menit.
- Keamanan: IAM user khusus monitoring, hanya `s3:ListBucket`.

### Objek 7: Server Beszel (via API)
- Metode: panggil API Beszel hub/agent, baca status sistem.
- Parameter: URL hub, API token, daftar agent/sistem yang dipantau.
- Metrik: status agent online/offline, CPU %, RAM %, disk %, load, uptime.
- Interval default: 1 menit.

### 3.1 Aturan status global per target
- `UP`: semua cek sukses.
- `DEGRADED`: sukses tapi lewati threshold (latency tinggi, TLS mau expired, packet loss > ambang).
- `DOWN`: cek konektivitas/status gagal N kali berturut (default N=3 untuk hindari flapping).
- `MAINTENANCE`: target dalam jendela maintenance, cek tetap jalan tapi tidak picu alert.
- `UNKNOWN`: belum pernah dicek / error konfigurasi.

---

## 4. Functional Requirements per Modul

### FR-01 Dashboard (4 dashboard)
- **FR-01.1 Dashboard Overview:** ringkasan semua objek: jumlah UP/DEGRADED/DOWN/MAINTENANCE, daftar insiden aktif, top 5 latency tertinggi, SLA 30 hari per kategori, grafik uptime 24 jam.
- **FR-01.2 Dashboard Network:** tabel perangkat + status ping/SNMP/TCP, grafik latency & packet loss, indikator CPU/memori, filter per lokasi/tipe.
- **FR-01.3 Dashboard App/Domain/DB:** status aplikasi + latency + TLS, status domain + expiry, status DB + waktu query. Badge environment Prod/Staging/Dev dengan warna beda.
- **FR-01.4 Dashboard Mail/S3/Server:** status SMTP/IMAP/queue/blacklist, status bucket S3, status agent Beszel + CPU/RAM/disk.
- **FR-01.5** Auto-refresh tiap 30 detik (bisa di-toggle). Filter: kategori, status, environment, tag. Klik baris → halaman detail target + grafik histori + daftar insiden.
- **FR-01.6** Detail target: grafik latency/uptime (24j/7h/30h), tabel hasil cek terakhir 50 baris, tombol: Test Now, Mute alert, Masuk maintenance.

### FR-02 CRUD Semua Objek
- **FR-02.1** CRUD untuk tiap tipe objek dengan form validasi (IP/hostname/URL wajib valid, port 1–65535).
- **FR-02.2** Fitur: tambah, edit, duplikat, aktif/nonaktif, hapus (soft delete, butuh konfirmasi + alasan, masuk audit log).
- **FR-02.3** Field umum: nama, kategori (7 objek), environment (Prod/Staging/Dev/Internal), tag, interval check (30s/1m/5m), threshold alert, flag kritikal, pemilik/penanggung jawab.
- **FR-02.4** Tombol "Test Connection" sebelum simpan, tampilkan hasil langsung (sukses/gagal + latency + pesan error).
- **FR-02.5** Import/export massal via CSV/Excel (template disediakan). Import menampilkan preview + baris error sebelum commit.
- **FR-02.6** Bulk action: ubah interval, aktif/nonaktif, masuk maintenance sekaligus.

### FR-03 Check Engine + Scheduler
- **FR-03.1** Scheduler Redis + BullMQ, interval pilihan: 30 detik, 1 menit, 5 menit. WHOIS/blacklist/expiry cukup 1x sehari (jadwal malam 01:00 WIB).
- **FR-03.2** Worker per tipe cek (icmp, snmp, http, dns, tcp-db, mail, s3, beszel). Retry 2x dengan backoff sebelum nyatakan DOWN.
- **FR-03.3** Anti-flapping: status berubah ke DOWN hanya jika gagal 3x berturut. Recovery ke UP butuh 2x sukses berturut.
- **FR-03.4** Timeout per cek default: 10 detik (bisa diubah per target 5–30 detik).
- **FR-03.5** Hasil cek simpan ke TimescaleDB hypertable: `check_results` (detail) + agregat per jam/hari (`check_hourly`, `check_daily`).
- **FR-03.6** Tombol "Run Now" menjalankan cek di luar jadwal, masuk antrean prioritas.
- **FR-03.7** Jika worker mati/antrean menumpuk > 5 menit, tampilkan banner warning di dashboard + alert ke Admin.

### FR-04 Alerting (Telegram/Discord/Email)
- **FR-04.1** Trigger: DOWN, recovery UP, DEGRADED, TLS/domain expiry warning, blacklist listed, queue overload.
- **FR-04.2** Channel: Telegram bot, Discord webhook, email SMTP. Per target bisa pilih 1+ channel + level (critical/warning/info).
- **FR-04.3** Isi alert wajib: nama target, kategori, environment, status lama→baru, waktu WIB, latency/error, link detail dashboard.
- **FR-04.4** Cooldown: alert DOWN yang sama tidak dikirim ulang dalam 15 menit (bisa diubah). Recovery selalu dikirim.
- **FR-04.5** Eskalasi: jika DOWN > 30 menit belum recovery, kirim ulang + tandai "ESKALASI".
- **FR-04.6** Maintenance mode menekan alert (tidak kirim), tapi insiden tetap dicatat.
- **FR-04.7** Log pengiriman alert: channel, waktu, sukses/gagal, response. Tombol "Test Alert" per channel.
- **FR-04.8** Template pesan bisa diedit Admin (variabel: {{name}}, {{status}}, {{time_wib}}, {{error}}, {{url}}).

### FR-05 Report Harian/Mingguan/Bulanan (PDF + Excel)
- **FR-05.1** Jenis: harian (07:00 WIB, data H-1), mingguan (Senin 07:00 WIB), bulanan (tgl 1, 07:00 WIB).
- **FR-05.2** Isi: ringkasan uptime% per kategori, SLA% per target kritikal, jumlah insiden + MTTR, top 10 downtime terlama, daftar expiry TLS/domain 30 hari ke depan, grafik uptime.
- **FR-05.3** Format: PDF (tampilan rapi, kop kantor, tanda tangan otomatis "Generated by Monitoring") + Excel (sheet: summary, detail insiden, SLA per target, raw agregat).
- **FR-05.4** Generate manual kapan saja dengan rentang tanggal kustom.
- **FR-05.5** Riwayat laporan tersimpan 1 tahun, bisa unduh ulang.

### FR-06 Schedule Email (Asia/Jakarta)
- **FR-06.1** Jadwal pengiriman laporan memakai timezone Asia/Jakarta. UI menampilkan waktu dalam WIB, simpan di DB sebagai UTC + konversi otomatis.
- **FR-06.2** Daftar penerima per jadwal (To/Cc), subjek kustom, lampiran PDF+Excel.
- **FR-06.3** Jika pengiriman gagal, retry 3x (interval 10 menit), lalu tandai gagal + alert ke Admin.
- **FR-06.4** Log email: waktu kirim WIB, penerima, status, message-id.

### FR-07 Maintenance Mode
- **FR-07.1** Atur per target atau bulk: mulai–selesai (WIB), alasan wajib, pembuat otomatis dari user login.
- **FR-07.2** Selama maintenance: status = MAINTENANCE, cek tetap jalan, alert ditekan, dashboard tampilkan badge kuning.
- **FR-07.3** Jadwal berulang didukung (mis. tiap Minggu 00:00–02:00 WIB).
- **FR-07.4** Maintenance yang kedaluwarsa otomatis selesai. Perpanjangan butuh alasan baru.
- **FR-07.5** Daftar maintenance aktif/riwayat terlihat di dashboard Overview.

### FR-08 Audit Log
- **FR-08.1** Catat: login/logout, CRUD target, ubah schedule/alert, maintenance, generate/acknowledge insiden, kirim laporan manual, ubah user/role.
- **FR-08.2** Field: waktu (WIB + UTC), user, IP, aksi, objek, nilai lama→baru (JSON).
- **FR-08.3** Read-only, tidak bisa dihapus operator. Retensi 1 tahun. Filter + ekspor CSV.

### FR-09 Insiden & SLA (pendukung)
- **FR-09.1** Insiden otomatis dibuat saat target DOWN, ditutup saat recovery. Field: mulai/selesai (WIB), durasi, penyebab (error terakhir), ack oleh siapa.
- **FR-09.2** SLA% = (total waktu UP / total waktu termonitor) × 100, dihitung per target/bulan, tidak termasuk periode MAINTENANCE.
- **FR-09.3** MTTR = rata-rata durasi insiden DOWN→recovery per periode.
- **FR-09.4** Tombol Acknowledge insiden oleh Operator/Admin.

---

## 5. Non-Functional Requirements

### 5.1 Performa
| Item | Target |
|---|---|
| Cek terjadwal 30s | Selesai < 25 detik untuk 200 target |
| Dashboard load | < 3 detik (100 target, data 24 jam) |
| API list target | < 500 ms (p95) |
| Generate laporan harian | < 2 menit |
| Concurrent user | 50 user bersamaan |
| Worker | Horizontal scale via tambah replica BullMQ worker |

### 5.2 Keamanan & RBAC
- RBAC 3 role (Admin/Operator/Viewer) enforced di BE + FE.
- Password hash argon2/bcrypt, min 8 karakter.
- Kredensial termonitor (SNMP community, DB password, S3 secret, Beszel token, SMTP password) disimpan terenkripsi (AES-256-GCM, kunci via env/secret manager), tidak pernah tampil plain di API/FE (masking `••••`).
- Rate limit login 5x/5 menit/IP. Audit log login gagal.
- HTTPS wajib di production. Header keamanan standar.
- Secret Telegram bot token / Discord webhook hanya bisa dilihat Admin, edit menimpa (tidak read-back plain).

### 5.3 Retensi data
- `check_results` (detail per cek): 90 hari, lalu hapus otomatis (job harian 02:00 WIB).
- Agregat `check_hourly` / `check_daily`: 1 tahun.
- Insiden, laporan, audit log: 1 tahun.
- Job retensi tercatat di log + tampilkan tanggal purge terakhir di halaman Settings.

### 5.4 Timezone Asia/Jakarta
- Semua input/output waktu di UI dalam WIB (Asia/Jakarta).
- DB simpan `timestamptz` (UTC). Konversi di BE.
- Scheduler (BullMQ repeat + cron laporan) memakai `Asia/Jakarta`.
- Format tampil: `DD MMM YYYY HH:mm WIB` (mis. 15 Okt 2026 07:00 WIB).
- DST tidak berlaku, tidak perlu penanganan khusus.

### 5.5 Ketersediaan & operasional
- Deploy Docker Compose: `web` (Next.js), `api` (NestJS/Laravel), `postgres+timescale`, `redis`, `worker`, `scheduler`.
- Health endpoint `/healthz` (api) dan `/api/health` (worker queue depth).
- Backup DB harian otomatis (pg_dump, simpan 7 hari). Restore terdokumentasi.
- Log aplikasi terstruktur (JSON), level info/warn/error.

### 5.6 Kompatibilitas
- Browser: Chrome/Edge/Firefox 2 versi terakhir.
- Mobile: dashboard readable (responsive), bukan prioritas penuh.

---

## 6. User Stories + Acceptance Criteria

### US-01 — Admin tambah target network
**Sebagai** Admin, **saya ingin** menambah router via IP + SNMP **agar** termonitor otomatis.
- AC1: Form validasi IP/hostname, community wajib jika tipe SNMP.
- AC2: Klik "Test Connection" tampilkan hasil < 15 detik.
- AC3: Setelah simpan, target muncul di Dashboard Network ≤ 1 menit dengan status awal (bukan UNKNOWN > 2 menit).
- AC4: Aksi tercatat di audit log.

### US-02 — Operator atur schedule check
**Sebagai** Operator, **saya ingin** ubah interval target 1m → 30s **agar** pantauan lebih rapat saat gangguan.
- AC1: Pilihan hanya 30s/1m/5m (dropdown, tidak ada input bebas).
- AC2: Perubahan berlaku pada siklus berikutnya, tampilkan "interval aktif" di detail.
- AC3: Bulk edit ≥ 20 target selesai < 10 detik.

### US-03 — Viewer lihat dashboard overview
**Sebagai** Viewer, **saya ingin** lihat status semua objek pagi hari **agar** tahu kondisi tanpa bertanya ke IT.
- AC1: Dashboard load < 3 detik.
- AC2: Tidak ada tombol CRUD/hapus terlihat.
- AC3: Waktu tampil dalam WIB.

### US-04 — Alert Telegram saat app down
**Sebagai** Operator, **saya ingin** terima pesan Telegram saat aplikasi Prod DOWN **agar** segera respon.
- AC1: Pesan masuk < 2 menit setelah 3x gagal berturut.
- AC2: Isi memuat nama, status lama→baru, waktu WIB, error, link detail.
- AC3: Tidak ada pesan duplikat dalam 15 menit cooldown.

### US-05 — Laporan harian otomatis via email
**Sebagai** IT Manager, **saya ingin** terima PDF+Excel tiap 07:00 WIB **agar** bisa evaluasi harian.
- AC1: Email tiba 07:00–07:15 WIB dengan 2 lampiran.
- AC2: Angka SLA di PDF = angka di dashboard untuk periode sama (toleransi rounding 0,01%).
- AC3: Jika gagal kirim, retry 3x lalu alert ke Admin.

### US-06 — Maintenance mode
**Sebagai** Operator, **saya ingin** set maintenance server 2 jam saat patching **agar** tidak kena spam alert.
- AC1: Alasan wajib diisi, tanpa alasan tombol simpan mati.
- AC2: Selama maintenance tidak ada alert keluar untuk target tsb (verifikasi di log alert).
- AC3: Setelah waktu selesai, status kembali otomatis ke hasil cek aktual.

### US-07 — Cek TLS expiry aplikasi
**Sebagai** Admin, **saya ingin** dapat peringatan H-30/H-14/H-7 sebelum sertifikat expired **agar** sempat renewal.
- AC1: Peringatan muncul sebagai status DEGRADED + alert warning.
- AC2: Daftar expiry 30 hari ke depan tampil di laporan.

### US-08 — Cek blacklist mail server
**Sebagai** Admin, **saya ingin** tahu jika IP mail masuk DNSBL **agar** segera delist.
- AC1: Status listed memicu alert critical + tampil nama DNSBL.
- AC2: Cek ulang manual via "Run Now" memperbarui status < 1 menit.

### US-09 — MTTR & SLA terlihat
**Sebagai** IT Manager, **saya ingin** lihat SLA% dan MTTR bulanan **agar** bisa lapor ke manajemen.
- AC1: SLA tidak hitung periode maintenance.
- AC2: MTTR = rata-rata durasi insiden tertutup pada periode tsb.
- AC3: Nilai bisa diklik untuk drill-down daftar insiden pembentuknya.

### US-10 — Import massal target
**Sebagai** Operator, **saya ingin** import 50 target via Excel **agar** tidak input satu-satu.
- AC1: Template tersedia untuk diunduh.
- AC2: Baris error ditampilkan sebelum commit, baris valid tetap bisa diproses (partial import dengan konfirmasi).
- AC3: Hasil import masuk audit log (jumlah sukses/gagal + file name).

---

## 7. Metrik Sukses

| Metrik | Definisi | Target | Sumber |
|---|---|---|---|
| Uptime aplikasi monitoring | % waktu app bisa diakses/bulan | ≥ 99,5% | Log uptime app sendiri |
| Uptime objek kritikal | Rata-rata SLA% target `is_critical`/bulan | ≥ 99% | `check_daily` |
| MTTR | Rata-rata durasi DOWN→recovery/bulan | < 60 menit (kritikal < 30 menit) | Tabel insiden |
| Waktu deteksi → alert | Selisih waktu DOWN confirmed → alert terkirim | < 2 menit (p95) | Log alert |
| Laporan terkirim | % jadwal laporan sukses terkirim tepat waktu | 100% (toleransi keterlambatan 15 mnt) | Log email |
| Cakupan monitor | % aset kritikal terdaftar & aktif | 100% aset kritikal | Inventaris vs DB target |
| False positive | % alert DOWN yang recovery < 2 menit (flapping) | < 5% | Log insiden |
| Adopsi dashboard | Viewer aktif mingguan | ≥ 80% akun Viewer login 1x/minggu | Audit log login |

Review metrik tiap bulan di laporan bulanan + rapat IT.

---

## 8. Out of Scope & Risiko

### 8.1 Out of scope (MVP ini TIDAK mencakup)
1. Auto-remediation / restart service otomatis.
2. Monitoring APM dalam kode (tracing, profiling) — hanya black-box dari luar.
3. Monitoring CCTV, printer, perangkat non-IP.
4. Mobile app native (cukup web responsive).
5. Multi-tenancy / multi-kantor. Single instance.
6. 2FA wajib, SSO/LDAP (fase 2).
7. SMS/voice call alerting (hanya Telegram/Discord/email).
8. Prediksi AI/anomaly detection (fase lanjutan).
9. Monitoring biaya cloud / billing S3.

### 8.2 Risiko & mitigasi
| Risiko | Dampak | Mitigasi |
|---|---|---|
| App monitoring itu sendiri down | Buta total | Health check eksternal sederhana (mis. UptimeRobot ke `/healthz`) + alert ke Admin via jalur luar |
| Redis/BullMQ macet, cek menumpuk | Data bolong, alert telat | Monitor queue depth + banner warning, worker autoscale/replica, persistensi Redis AOF |
| Kredensial monitoring bocor | Akses DB/SNMP/S3 disalahgunakan | Enkripsi at-rest, akun read-only minimal, rotasi berkala, audit log akses |
| Flapping / false positive | Alert fatigue, alert diabaikan | Aturan 3x gagal + cooldown 15 mnt + eskalasi 30 mnt |
| Waktu server melenceng (scheduler/report jam salah) | Laporan telat, jadwal WIB kacau | NTP wajib di semua container, scheduler pakai tz Asia/Jakarta, simpan UTC |
| DB membengkak (cek 30s × banyak target) | Disk penuh, query lambat | Hypertable + retensi 90 hari + agregat, monitoring disk, purge job terjadwal |
| Perubahan API Beszel / S3 | Cek gagal massal | Versi API di-pin, error mapping jelas (bedakan config error vs down), Test Connection |
| DNSBL rate-limit | Cek blacklist gagal | Cache hasil 5 mnt, batasi daftar DNSBL, backoff retry |
| Email laporan masuk spam | Laporan tidak dibaca | SPF/DKIM/DMARC domain pengirim, subjek konsisten, log + retry |

### 8.3 Asumsi
- Jaringan internal mengizinkan ICMP/SNMP dari worker ke perangkat.
- Kredensial read-only DB/SNMP/S3 disediakan pemilik sistem.
- Server SMTP relay untuk email laporan & alert tersedia.
- Bot Telegram + webhook Discord dibuat sebelum UAT.

---

## Lampiran A — Ringkasan API/Struktur Data (acuan implementasi)

- Entitas inti: `targets` (id, name, category, env, config JSONB, interval, thresholds, is_critical, is_active), `check_results` (hypertable: target_id, ts, status, latency_ms, payload JSONB), `incidents` (target_id, started_at, ended_at, ack_by), `maintenances`, `alert_logs`, `report_schedules`, `report_history`, `audit_logs`, `users`.
- Endpoint acuan: `GET /api/dashboard/overview`, `CRUD /api/targets`, `POST /api/targets/:id/test`, `POST /api/targets/:id/run-now`, `CRUD /api/maintenances`, `GET /api/incidents`, `POST /api/incidents/:id/ack`, `CRUD /api/report-schedules`, `POST /api/reports/generate`, `GET /api/audit-logs`.

*End of PRD v1.0*
