# Product Requirements Document (PRD)
## Sistem Monitoring Infrastruktur & Pelaporan Terpusat (Internal IT Monitoring System)

| Informasi Dokumen | Detail |
| :--- | :--- |
| **Nama Produk** | Internal Infrastructure & Service Monitoring System (IISMS) |
| **Versi Dokumen** | 1.0 |
| **Tanggal** | 15 Oktober 2026 |
| **Status** | Ready for Development |
| **Target Pengguna** | Tim IT Internal (SysAdmin, Network Engineer, DevOps, Developer, IT Manager) |

---

## 1. Latar Belakang & Tujuan Produk

### 1.1 Latar Belakang
Infrastruktur IT kantor terdiri dari berbagai komponen kritis yang tersebar di jaringan lokal maupun *cloud*, mulai dari perangkat jaringan (Router, Switch, Access Point, Server), aplikasi web di tiga *environment* (`Production`, `Staging`, `Development`), database, domain & SSL, mail server, penyimpanan objek (S3), hingga metrik *resource* server yang dipantau melalui agen **Beszel**. Pemantauan manual atau terpisah-pisah menyulitkan deteksi dini insiden serta menyita waktu dalam penyusunan laporan SLA (*Service Level Agreement*) berkala.

### 1.2 Tujuan Produk
1. **Single Pane of Glass:** Menyediakan satu pusat kendali visual (3 Dashboard Spesifik + 1 Executive NOC View) untuk memantau seluruh aset IT kantor secara *real-time*.
2. **Validasi Kesehatan Akurat (Deep Health Check):** Mengeliminasi *false-positive* dengan memadukan protokol dasar (ICMP, cURL, TCP Socket) dan protokol tingkat aplikasi (SNMP, Native DB Ping `SELECT 1`, WHOIS/SSL Check, S3 `HeadBucket`, SMTP/DNSBL, dan REST API Beszel).
3. **Otomasi Pelaporan & Penjadwalan (Schedule Run & Reporting):** Menghasilkan laporan kinerja dan SLA otomatis secara **Harian**, **Mingguan**, dan **Bulanan** dalam format HTML, PDF, dan Excel/CSV, serta mengirimkannya via email sesuai jadwal (*Schedule Run*).

---

## 2. Profil Pengguna & Hak Akses (User Roles)

| Peran (Role) | Deskripsi & Hak Akses |
| :--- | :--- |
| **Super Admin / Infra Lead** | Akses penuh ke seluruh dashboard, manajemen CRUD semua target monitoring, manajemen kredensial terenkripsi, konfigurasi *Schedule Run*, pembuatan laporan, dan pengaturan pengguna. |
| **Network & SysAdmin** | Akses penuh pada **Dashboard Network** dan **Dashboard Mail, S3, & Beszel**, termasuk CRUD target jaringan/server dan eksekusi *Test Run*. |
| **Developer & DBA** | Akses pada **Dashboard Aplikasi, Domain, & Database**, melihat log *response time*, mengelola target *Staging/Development*, serta mengaktifkan *Maintenance Mode* saat *deployment*. |
| **IT Manager / Executive** | Akses *Read-Only* ke seluruh dashboard, layar **NOC Overview**, serta menerima laporan terjadwal (Harian/Mingguan/Bulanan) via email dan mengunduh arsip PDF SLA. |

---

## 3. Ruang Lingkup Target Monitoring & Spesifikasi Protokol

Sistem memonitor **7 kategori target utama** yang dikelompokkan ke dalam **3 Dashboard operasional**:

| No | Kategori Target | Grup Dashboard | Protokol & Metode Pengecekan | Metrik Utama yang Diambil | Interval Ideal |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **1** | **Network Internal** *(Router, Switch, AP, Server)* | Dashboard 1: Network | • **ICMP (Ping)**<br>• **TCP Socket**<br>• **SNMP v2c/v3 / API** | Status Up/Down, Latency ($\text{ms}$), Packet Loss ($\%$), CPU Load ($\%$), RAM ($\%$), Interface Traffic ($\text{Mbps}$). | $30\text{--}60\text{ detik}$ |
| **2** | **Aplikasi Website** *(Prod, Staging, Dev)* | Dashboard 2: App, Domain, DB | • **HTTP/HTTPS (cURL)**<br>• **JSON `/health` Check** | HTTP Status Code ($200\text{ OK}$), Response Time ($\text{ms}$), Validasi *payload* JSON, Status Sub-sistem. | $60\text{ detik}$ (Prod)<br>$300\text{ detik}$ (Dev) |
| **3** | **Domain Kantor** | Dashboard 2: App, Domain, DB | • **WHOIS / RDAP**<br>• **TLS/SSL Socket (`443`)**<br>• **DNS Resolver** | Tanggal Kedaluwarsa Domain, Sisa Hari Domain, Tanggal & Sisa Hari Sertifikat SSL, Penerbit SSL, Kesesuaian IP DNS (`A`, `MX`). | $12\text{--}24\text{ jam}$ |
| **4** | **Database Kantor** *(Prod, Staging, Dev)* | Dashboard 2: App, Domain, DB | • **TCP Socket Check**<br>• **Native Driver Ping (`SELECT 1`)** | Status Koneksi Port, Waktu Eksekusi Query ($\text{ms}$), Jumlah Koneksi Aktif (*Active Connections*). | $60\text{ detik}$ (Prod)<br>$300\text{ detik}$ (Dev) |
| **5** | **Mail Server** | Dashboard 3: Mail, S3, Beszel | • **SMTP/IMAP Socket Banner**<br>• **DNSBL Lookup**<br>• **DNS (`MX`, `SPF`, `DMARC`)** | Status Port (`25`, `465`, `587`, `993`), Waktu Respons Banner ($\text{ms}$), Status Blacklist IP Publik (Aman/Terblokir). | $300\text{ detik}$ (Port)<br>$6\text{ jam}$ (DNSBL) |
| **6** | **Object Storage (S3)** | Dashboard 3: Mail, S3, Beszel | • **HTTP/HTTPS (cURL)**<br>• **S3 SDK `HeadBucket` (AWS SigV4)** | Status Aksesibilitas Bucket, Waktu Respons ($\text{ms}$), Validasi Kredensial *Read-Only*. | $300\text{ detik}$ |
| **7** | **Server dari Beszel** | Dashboard 3: Mail, S3, Beszel | • **Beszel REST API** *(PocketBase API)* | Status Agen Beszel, CPU ($\%$), RAM Terpakai ($\text{GB}/\%$), Disk Terpakai ($\text{GB}/\%$), Network I/O, Status Kontainer Docker. | $60\text{ detik}$ |

---

## 4. Kebutuhan Fungsional (Functional Requirements)

### 4.1 Modul Dashboard & Visualisasi (`FR-DASH`)
* **FR-DASH-01 (Executive NOC Overview):** Halaman ringkasan utama yang menampilkan kartu total target berdasarkan status (`UP`, `DEGRADED`, `DOWN`, `MAINTENANCE`), insiden aktif saat ini, dan grafik *uptime* gabungan. Mendukung mode layar penuh (*TV Dark Mode*) dengan *auto-refresh* setiap $15\text{ detik}$.
* **FR-DASH-02 (Dashboard 1 - Network Internal):** Menampilkan daftar dan topologi status perangkat Router, Switch, Access Point, dan Server Internal lengkap dengan grafik *latency* ($\text{ms}$), *packet loss* ($\%$), dan utilisasi *resource* SNMP.
* **FR-DASH-03 (Dashboard 2 - Aplikasi, Domain & Database):**
  * Memiliki filter tab berdasarkan *Environment*: `All`, `Production`, `Staging`, dan `Development`.
  * Menampilkan grafik waktu respons HTTP dan query Database.
  * Menampilkan tabel *Countdown* masa aktif Domain dan SSL dengan indikator warna: Hijau ($> 30\text{ hari}$), Kuning ($15\text{--}30\text{ hari}$), dan Merah ($\le 14\text{ hari}$).
* **FR-DASH-04 (Dashboard 3 - Mail Server, S3 & Beszel Server):**
  * Menampilkan kesehatan port Mail Server dan hasil pengecekan *blacklist* (DNSBL).
  * Menampilkan konektivitas *bucket* S3.
  * Menampilkan kartu metrik *real-time* untuk setiap server yang terhubung dari **Beszel** (bar indikator CPU, RAM, Disk, Suhu, serta daftar kontainer Docker yang berjalan).

### 4.2 Modul CRUD Target Monitoring (`FR-CRUD`)
* **FR-CRUD-01 (Manajemen Target Terpadu):** Pengguna dapat melakukan *Create, Read, Update, Delete* untuk ke-7 jenis target monitoring melalui formulir dinamis yang menyesuaikan *input field* berdasarkan jenis target yang dipilih.
* **FR-CRUD-02 (Konfigurasi Parameter Pengecekan):** Setiap target wajib memiliki pengaturan:
  * `check_interval_seconds`: Frekuensi pengecekan (contoh: $30\text{s}$, $60\text{s}$, $300\text{s}$, $86400\text{s}$).
  * `timeout_seconds`: Batas waktu tunggu maksimal (default: $5\text{ detik}$).
  * `retry_threshold`: Jumlah gagal berturut-turut sebelum dinyatakan `DOWN` (default: $3\text{ kali}$).
  * `degraded_threshold_ms`: Batas waktu respons (ms) untuk menandai layanan sebagai `DEGRADED` / Lambat.
* **FR-CRUD-03 (Enkripsi Kredensial):** Seluruh data sensitif (Password Database, SNMP Community/AuthKey, S3 Secret Key, Beszel API Token) wajib dienkripsi dengan algoritma **AES-256-CBC / AES-256-GCM** sebelum disimpan ke database dan tidak pernah ditampilkan kembali dalam bentuk *plain text* di UI.
* **FR-CRUD-04 (Maintenance / Pause Toggle):** Tombol sakelar (*toggle*) untuk menjeda monitoring pada target tertentu atau satu *environment* (misal menjeda seluruh `Staging` saat pemeliharaan) agar tidak memicu *alert* palsu maupun mengurangi nilai SLA.
* **FR-CRUD-05 (Test Connection Button):** Tombol pada form CRUD untuk menguji koneksi protokol secara langsung sebelum data target disimpan.

### 4.3 Modul Schedule Run (`FR-SCHED`)
* **FR-SCHED-01 (Health Check Scheduler):** Mesin penjadwal latar belakang yang membaca konfigurasi `check_interval_seconds` tiap target dan mendistribusikan tugas pengecekan ke antrean pekerja (*Queue Worker*) secara presisi.
* **FR-SCHED-02 (Reporting Schedule CRUD):** Pengguna dapat membuat, mengubah, menonaktifkan, dan menghapus jadwal pembuatan laporan otomatis dengan parameter:
  * **Periode Jadwal:** `Harian` (pilih jam), `Mingguan` (pilih hari & jam), `Bulanan` (pilih tanggal & jam), atau `Custom Cron Expression`.
  * **Cakupan Data (Scope):** Semua Dashboard, Dashboard tertentu, atau filter khusus (misal hanya *Environment* `Production`).
  * **Kondisi Pengiriman:** *Always Send* (selalu kirim sesuai jadwal) atau *Only If Issues Found* (hanya kirim jika terdapat insiden atau peringatan kapasitas).
* **FR-SCHED-03 (Manual Trigger / "Run Now"):** Tombol aksi pada setiap daftar *Schedule Run* untuk langsung men-generate laporan dan mengirim email saat itu juga guna keperluan pengujian atau kebutuhan mendadak.

### 4.4 Modul Generate Report: Harian, Mingguan, Bulanan (`FR-REP`)
* **FR-REP-01 (Laporan Harian / Daily Operational Report):**
  * Merangkum data $24\text{ jam}$ terakhir.
  * Berisi: Ringkasan status pagi ini, daftar insiden/downtime kemarin beserta durasinya, target dengan *latency* tertinggi, peringatan Domain/SSL ($\le 14\text{ hari}$), dan server Beszel dengan penggunaan Disk/RAM $> 80\%$.
* **FR-REP-02 (Laporan Mingguan / Weekly Trend Report):**
  * Merangkum data $7\text{ hari}$ terakhir dari tabel agregasi harian.
  * Berisi: Perbandingan *uptime* dan *response time* minggu ini vs minggu lalu, daftar *Top 5 Slowest Services*, layanan yang paling sering mengalami *flapping* (putus-nyambung), dan tren beban rata-rata server Beszel & Network.
* **FR-REP-03 (Laporan Bulanan / Monthly SLA & Executive Report):**
  * Merangkum data $1\text{ bulan}$ kalender penuh.
  * Berisi: Pencapaian SLA Uptime ($\%$) per kategori dan *environment*, total durasi *downtime* (menit), nilai MTTR (*Mean Time To Recovery*), jumlah insiden bulanan, dan ringkasan kapasitas penyimpanan (Database, S3, Disk Beszel).
* **FR-REP-04 (Multi-Format Export):** Setiap laporan yang di-generate menghasilkan:
  1. **Ringkasan HTML Inline** (tampil langsung di badan email).
  2. **Dokumen PDF** yang dilengkapi grafik dan tabel berformat rapi untuk manajemen.
  3. **Dokumen Excel (`.xlsx`) / CSV** untuk lampiran data teknis.
  4. Arsip file laporan tersimpan di server dan dapat diunduh ulang kapan saja dari menu **Riwayat Laporan**.

### 4.5 Modul Pengiriman Email & Alerting (`FR-MAIL`)
* **FR-MAIL-01 (Scheduled Report Email):** Mengirimkan email laporan otomatis sesuai jadwal pada *Schedule Run* ke daftar penerima (`To`, `CC`, `BCC`) beserta lampiran PDF/Excel.
* **FR-MAIL-02 (Real-Time State-Change Alert):** Mengirimkan email peringatan instan ketika target berubah status dari `UP` menjadi `DOWN` (setelah melewati batas `retry_threshold`) dan ketika kembali pulih (`RECOVERED`).
* **FR-MAIL-03 (Retry & Delivery Audit Log):** Jika pengiriman email gagal, sistem melakukan percobaan ulang otomatis (*exponential backoff*) hingga $3\text{ kali}$ dan mencatat seluruh riwayat pengiriman di menu **Email Delivery Logs** (`Sent`, `Retrying`, `Failed` beserta pesan kesalahannya).
* **FR-MAIL-04 (Multi-Channel Failover):** Mendukung konfigurasi SMTP Relay Eksternal dan *webhook* cadangan (Telegram Bot / Slack) agar notifikasi tetap sampai meskipun Mail Server internal kantor sedang mati.

---

## 5. Standar Perhitungan Metrik & Rumus SLA

Sistem menggunakan formula matematis standar industri berikut untuk menghitung metrik pada laporan Harian, Mingguan, dan Bulanan:

1. **Persentase Uptime (SLA Uptime $\%$):**
   $$\text{Uptime (\%)} = \left( \frac{T_{\text{total}} - T_{\text{maintenance}} - T_{\text{downtime}}}{T_{\text{total}} - T_{\text{maintenance}}} \right) \times 100\%$$
   *Di mana:*
   * $T_{\text{total}}$ = Total waktu dalam periode laporan (dalam detik/menit).
   * $T_{\text{maintenance}}$ = Total waktu target berada dalam mode *Maintenance* terjadwal.
   * $T_{\text{downtime}}$ = Total durasi insiden ketika target berstatus `DOWN`.

2. **Mean Time To Recovery ($\text{MTTR}$):**
   $$\text{MTTR} = \frac{\sum_{i=1}^{n} (t_{\text{recovered}, i} - t_{\text{down}, i})}{n}$$
   *Di mana $n$ adalah total insiden yang terselesaikan pada periode tersebut, $t_{\text{down}}$ adalah waktu insiden dimulai, dan $t_{\text{recovered}}$ adalah waktu layanan kembali normal.*

3. **Mean Time Between Failures ($\text{MTBF}$):**
   $$\text{MTBF} = \frac{T_{\text{total}} - T_{\text{maintenance}} - T_{\text{downtime}}}{n}$$

4. **Packet Loss ($\%$) pada Network ICMP:**
   $$\text{Packet Loss (\%)} = \left( 1 - \frac{P_{\text{received}}}{P_{\text{transmitted}}} \right) \times 100\%$$

---

## 6. Kebutuhan Non-Fungsional (Non-Functional Requirements)

1. **Performa & Konkurensi:**
   * Waktu muat halaman Dashboard maksimal $\le 1.5\text{ detik}$ (mengambil status terkini dari *in-memory cache* Redis).
   * Pekerja latar belakang (*Queue Worker*) harus mampu mengeksekusi minimal $100\text{ target pengecekan}$ per menit secara paralel tanpa saling memblokir (*non-blocking I/O*).
2. **Retensi Data & Optimasi Penyimpanan:**
   * Log pengecekan mentah (`monitoring_logs`) disimpan selama **$90\text{ hari}$**, kemudian dihapus otomatis (*auto-pruning* via *monthly table partition drop*).
   * Data rekapitulasi harian (`monitoring_daily_summaries`) dan arsip laporan PDF disimpan selama **$5\text{ tahun}$** untuk kebutuhan audit historis.
3. **Keamanan:**
   * Seluruh komunikasi web menggunakan HTTPS (TLS 1.3).
   * Kredensial target dienkripsi menggunakan AES-256.
   * Koneksi pengecekan ke database target wajib menggunakan akun khusus berhak akses *Read-Only* minimal.
4. **Ketersediaan Tinggi (Resiliensi Monitoring):**
   * Seluruh layanan aplikasi dibungkus dalam kontainer Docker dengan kebijakan `restart: unless-stopped`.

---

## 7. Kriteria Penerimaan (Acceptance Criteria)

1. Pengguna berhasil menambahkan ke-7 jenis target melalui form CRUD, menjalankan tombol **Test Connection**, dan melihat hasilnya tampil di masing-masing dari 3 Dashboard sesuai kategorinya.
2. Ketika sebuah database atau aplikasi `Production` dimatikan, sistem mencoba mengecek sebanyak $3\text{ kali}$ (sesuai *retry threshold*) sebelum mengubah status menjadi `DOWN` dan mengirim email *alert* maksimal dalam waktu $< 2\text{ menit}$.
3. Metrik CPU, RAM, Disk, dan kontainer Docker dari **Beszel** tampil secara akurat di Dashboard 3 tanpa perlu memasang agen baru di server target.
4. Admin berhasil membuat jadwal **Schedule Run** (Harian, Mingguan, Bulanan), menekan tombol **Run Now**, dan menerima email berisi tabel ringkasan HTML beserta lampiran PDF dan Excel yang valid di kotak masuk.