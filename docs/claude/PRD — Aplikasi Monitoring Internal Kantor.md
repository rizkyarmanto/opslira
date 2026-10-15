# PRD — Aplikasi Monitoring Internal Kantor

*Versi 0.1 (draft) · 15 Oktober 2026 · Dokumen terkait: Dokumen Arsitektur Aplikasi Monitoring Internal*

## 1. Ringkasan

Aplikasi web internal untuk memantau kesehatan infrastruktur dan layanan kantor dalam satu tempat: jaringan, aplikasi (production, staging, development), domain, database, mail server, S3, dan server yang dikelola Beszel. Aplikasi menyediakan dashboard, pengelolaan target lewat CRUD, alert, serta report harian, mingguan, dan bulanan yang dikirim otomatis lewat email sesuai jadwal.

Pendekatan teknis yang diusulkan adalah hybrid: aplikasi custom (control plane) untuk CRUD, dashboard, incident, dan report, dengan Prometheus sebagai mesin pengecekan dan penyimpan time-series. Detailnya ada di Dokumen Arsitektur.

## 2. Latar belakang dan masalah

*Asumsi awal, perlu divalidasi dengan tim.*

- Belum ada satu tampilan terpadu untuk kondisi jaringan, aplikasi, domain, database, mail, S3, dan server.
- Gangguan berisiko diketahui terlambat bila tidak ada alert otomatis dengan tingkat keparahan yang membedakan production dari staging dan development.
- Pengecekan sebatas ping, curl, dan telnet hanya menjawab hidup atau mati. Masalah seperti sertifikat atau domain yang akan expired, port flapping, dan DB yang terbuka tetapi tidak sehat tidak terlihat.
- Belum ada laporan berkala (uptime, insiden, tren) untuk evaluasi internal.
- Menambah target baru masih bergantung pada edit konfigurasi manual.

## 3. Tujuan dan non-tujuan

### Tujuan

1. Satu sumber kebenaran untuk status seluruh infrastruktur dan layanan kantor.
2. Deteksi dini dan notifikasi cepat, dengan severity yang mengikuti environment.
3. Report harian, mingguan, dan bulanan yang dihasilkan dan dikirim otomatis.
4. Target, check, jadwal, dan penerima dapat dikelola lewat UI tanpa deploy ulang.
5. Pengecekan yang bermakna (expiry, login, API), bukan hanya reachability.

### Non-tujuan (versi 1)

- Bukan APM, distributed tracing, atau log management.
- Bukan pengganti Grafana untuk analisis ad-hoc yang mendalam.
- Tidak ada auto-remediation.
- Tidak multi-tenant dan tidak ada status page publik.
- Tidak ada aplikasi mobile native (cukup web responsif).

## 4. Pengguna dan peran

| Peran | Deskripsi | Hak utama |
| --- | --- | --- |
| Admin | Penanggung jawab aplikasi | Semua fitur, termasuk user, channel notifikasi, dan audit log |
| Editor | Operator/engineer | CRUD target dan check, acknowledge incident, maintenance window, jadwal report dan penerima |
| Viewer | Pengguna umum tim | Melihat dashboard, incident, dan report |
| Penerima report | Tidak punya akun | Menerima report lewat email |

## 5. Cakupan monitoring

| Kategori | Contoh target | Pengecekan utama | Environment |
| --- | --- | --- | --- |
| Network internal | Router, switch, AP, server internal | ICMP, SNMP (status interface, traffic, CPU/memori, uptime), TCP port | Internal |
| Aplikasi kantor | Website | HTTP(S): status code, keyword, response time, expiry SSL | Production, staging, development |
| Domain | Domain kantor | Expiry domain (RDAP/WHOIS), DNS record, expiry SSL | Semua |
| Database | Database kantor | TCP port, login + `SELECT 1`, opsional jumlah koneksi dan replication lag | Production, staging, development |
| Mail server | Server email | Handshake SMTP/IMAP + STARTTLS, expiry sertifikat, MX/SPF/DKIM/DMARC, blacklist | Production |
| S3 | Endpoint dan bucket S3 | S3 API (HeadBucket, put-get-delete objek kecil), latency | Production |
| Server Beszel | Server yang dipantau Beszel | Status serta CPU/memori/disk lewat API hub Beszel | Semua |

Catatan: Telnet diimplementasikan sebagai TCP connect check, karena protokol Telnet sendiri tidak diperlukan.

## 6. Kebutuhan fungsional

Prioritas: P0 = wajib untuk rilis awal, P1 = penting, P2 = nanti.

### 6.1 Manajemen target (CRUD)

| ID | Kebutuhan | Prioritas |
| --- | --- | --- |
| FR-1.1 | CRUD target untuk seluruh kategori pada bagian 5, dengan field wajib sesuai kategori | P0 |
| FR-1.2 | Atribut target: environment, tag, owner, interval, timeout, jumlah retry, dan parent | P0 |
| FR-1.3 | Satu target dapat memiliki banyak check (mis. website: HTTP + TLS; domain: RDAP + DNS + TLS) | P0 |
| FR-1.4 | Pause/enable target tanpa menghapus datanya | P0 |
| FR-1.5 | Kredensial (DB, S3, SNMP, SMTP) disimpan di Vault; UI hanya menyimpan referensi | P0 |
| FR-1.6 | Perubahan target diterapkan ke mesin pengecekan otomatis dalam 1 menit, tanpa restart | P0 |
| FR-1.7 | Tombol Test check sebelum menyimpan | P1 |
| FR-1.8 | Import dan export massal (CSV/YAML) | P2 |

### 6.2 Pengecekan

| ID | Kebutuhan | Prioritas |
| --- | --- | --- |
| FR-2.1 | ICMP ping dan TCP connect | P0 |
| FR-2.2 | HTTP(S): status code yang diharapkan, keyword, response time, redirect, expiry SSL | P0 |
| FR-2.3 | DNS resolve dan cek record | P1 |
| FR-2.4 | SNMP untuk router, switch, dan AP: status interface, traffic, error, CPU/memori, uptime | P0 |
| FR-2.5 | Expiry domain lewat RDAP dengan fallback WHOIS | P0 |
| FR-2.6 | SMTP/IMAP handshake dan STARTTLS, expiry sertifikat | P0 |
| FR-2.7 | Cek MX/SPF/DKIM/DMARC dan blacklist | P2 |
| FR-2.8 | S3 API: HeadBucket dan put-get-delete objek kecil, ukur latency | P0 |
| FR-2.9 | Database: login dan `SELECT 1` (PostgreSQL, MySQL/MariaDB); replication lag | P0 / P2 |
| FR-2.10 | Integrasi API hub Beszel: status server, CPU, memori, disk | P0 |
| FR-2.11 | Retry N kali sebelum status DOWN dan deteksi flapping | P0 |

### 6.3 Dashboard

| ID | Kebutuhan | Prioritas |
| --- | --- | --- |
| FR-3.1 | Overview: ringkasan status global, incident aktif, dan item yang segera expired | P0 |
| FR-3.2 | Dashboard Network: status perangkat, traffic dan error interface, uptime | P0 |
| FR-3.3 | Dashboard Aplikasi, Domain, dan Database: matriks per environment, response time, sisa hari expiry SSL dan domain, status DB | P0 |
| FR-3.4 | Dashboard Mail Server, S3, dan Server: status, latency, resource server dari Beszel | P0 |
| FR-3.5 | Halaman detail target dengan grafik 24 jam, 7 hari, dan 30 hari | P1 |
| FR-3.6 | Filter berdasarkan kategori, environment, dan tag; pencarian; auto-refresh | P0 |

### 6.4 Alert dan incident

| ID | Kebutuhan | Prioritas |
| --- | --- | --- |
| FR-4.1 | Alert berbasis rule untuk down, latency tinggi, mendekati expiry, dan resource kritis | P0 |
| FR-4.2 | Notifikasi lewat Telegram dan email; notifikasi resolve | P0 |
| FR-4.3 | Severity mengikuti environment (production critical, staging warning, development info) | P0 |
| FR-4.4 | Lifecycle incident: open, acknowledged, resolved, dengan timeline | P0 |
| FR-4.5 | Deduplikasi dan pengelompokan alert | P0 |
| FR-4.6 | Suppress parent-child (mis. switch mati tidak memicu alert semua AP di bawahnya) | P1 |
| FR-4.7 | Maintenance window dan silence, dikecualikan dari perhitungan SLA | P1 |
| FR-4.8 | Channel tambahan (Slack, webhook umum) | P2 |

### 6.5 Report dan schedule

| ID | Kebutuhan | Prioritas |
| --- | --- | --- |
| FR-5.1 | Report harian (kemarin 00:00-24:00): incident 24 jam, target bermasalah, uptime per kategori, sertifikat/domain yang segera expired | P0 |
| FR-5.2 | Report mingguan (Senin-Minggu lalu): tren uptime dan latency, perbandingan minggu sebelumnya, top 5 target bermasalah, MTTR | P1 |
| FR-5.3 | Report bulanan (bulan kalender lalu): uptime/SLA per target dan environment, total downtime, tren kapasitas server, ringkasan incident | P1 |
| FR-5.4 | CRUD jadwal: tipe, ekspresi cron, timezone, scope (kategori/environment/tag/target), format, grup penerima, enable/disable | P0 |
| FR-5.5 | Pengiriman email: ringkasan di body HTML, lampiran PDF dan CSV, link ke dashboard, retry dengan backoff | P0 |
| FR-5.6 | Idempotency: tidak ada email ganda untuk periode yang sama; catch-up bila aplikasi sempat mati | P0 |
| FR-5.7 | Riwayat run (status, error, file hasil) dan regenerate untuk periode lampau | P1 |
| FR-5.8 | Run now dan kirim email uji | P1 |

### 6.6 Administrasi dan keamanan

| ID | Kebutuhan | Prioritas |
| --- | --- | --- |
| FR-6.1 | Login lokal dan RBAC tiga peran (admin, editor, viewer) | P0 |
| FR-6.2 | Audit log untuk semua perubahan CRUD | P0 |
| FR-6.3 | Manajemen user, channel notifikasi, dan grup penerima | P0 |
| FR-6.4 | Endpoint health dan metrik internal aplikasi | P0 |
| FR-6.5 | SSO (OIDC/LDAP) | P2 |

## 7. Kebutuhan non-fungsional (usulan)

| Aspek | Target |
| --- | --- |
| Skala awal | Hingga 500 target dan 2.000 check; interval minimum 30 detik, default 60 detik |
| Latensi deteksi | Gangguan production terdeteksi dan notifikasi terkirim dalam 3 menit |
| Performa dashboard | p95 pemuatan halaman di bawah 2 detik |
| Ketersediaan | 99,5% untuk aplikasi; ada heartbeat eksternal untuk memantau aplikasi itu sendiri |
| Retensi | Time-series 15-30 hari; statistik harian, report, incident, dan audit log minimal 13 bulan |
| Keamanan | TLS, password di-hash, secret di Vault, akun read-only untuk check database, audit log |
| Kompatibilitas | Browser modern; tampilan responsif untuk melihat status dari ponsel |
| Pemulihan | Backup Postgres harian dan uji restore berkala |

## 8. Metrik keberhasilan

| Metrik | Target |
| --- | --- |
| Cakupan target terdaftar | 100% dari daftar pada bagian 5 |
| MTTD untuk production | Di bawah 3 menit |
| False positive alert | Di bawah 5% |
| Report terkirim tepat waktu | Minimal 99% |
| Sertifikat/domain expired tanpa peringatan | 0 |

## 9. Rilis bertahap

| Fase | Cakupan |
| --- | --- |
| 1 | Model target, CRUD, check ICMP/HTTP/TCP, alert Telegram, dashboard dasar, incident log |
| 2 | Expiry SSL dan domain, check database, S3, dan mail, rollup harian |
| 3 | SNMP untuk network dan integrasi Beszel |
| 4 | Report generator, scheduler, dan pengiriman email (harian dulu, lalu mingguan dan bulanan) |
| 5 | Suppress parent-child, maintenance window, RBAC lanjutan, dan penyempurnaan |

## 10. Asumsi dan dependensi

- Seluruh komponen berjalan di jaringan kantor agar dapat menjangkau LAN.
- Vault tersedia untuk menyimpan kredensial.
- API hub Beszel dapat diakses dari aplikasi.
- Tersedia SMTP relay untuk pengiriman report, dan bot Telegram untuk alert.
- Akun read-only dapat disediakan di setiap database yang dipantau.

## 11. Risiko

| Risiko | Dampak | Mitigasi |
| --- | --- | --- |
| Mail server yang dipantau juga dipakai untuk mengirim report dan alert | Report dan alert tidak terkirim saat mail server down | Alert kritikal lewat Telegram; gunakan SMTP relay terpisah; retry dan catat kegagalan |
| Retensi Prometheus pendek | Report bulanan tidak akurat | Rollup harian ke Postgres sebagai sumber report |
| Alert berlebihan | Alert fatigue | Retry sebelum DOWN, severity per environment, suppress parent-child, maintenance window |
| Kredensial check tersebar | Kebocoran | Semua secret di Vault, akun read-only, audit log |
| ICMP butuh privilege khusus di container | Check ICMP gagal | `CAP_NET_RAW` atau `ping_group_range`; uji sejak fase 1 |
| Aplikasi monitoring itu sendiri mati tanpa diketahui | Buta saat gangguan | Alert Watchdog ke heartbeat eksternal |

## 12. Pertanyaan terbuka

1. Keputusan akhir: hybrid dengan Prometheus atau engine pengecekan sepenuhnya custom?
2. Perkiraan jumlah target dan interval yang dibutuhkan per kategori?
3. Channel notifikasi selain Telegram dan email?
4. SMTP relay apa yang dipakai, dan berapa batas ukuran lampiran?
5. Autentikasi: cukup login lokal atau perlu SSO?
6. Bagaimana autentikasi ke API hub Beszel?
7. Siapa penerima tiap report, dan apakah perlu isi berbeda untuk manajemen dan teknis?
8. Target SLA per layanan dan kebijakan retensi final?
9. Apakah UI perlu diakses dari luar kantor (lewat VPN atau diekspos)?
