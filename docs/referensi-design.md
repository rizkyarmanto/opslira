# Dokumen Referensi Desain UI/UX & Standar Komponen Frontend
**Nama Proyek:** Sistem Monitoring Infrastruktur & Layanan Internal (Single Pane of Glass)  
**Dokumen:** `referensi-design.md`  
**Versi:** 1.0  
**Target Pembaca:** Frontend Engineer, UI/UX Designer, Full-Stack Developer  

---

## 1. Filosofi Desain & Identitas Visual

Aplikasi monitoring ini dirancang menggunakan pendekatan **"High-Contrast Dark NOC (Network Operations Center)"**. Tujuan utamanya adalah memastikan keterbacaan metrik yang tinggi dari jarak jauh (layar TV/Monitor ruang IT 24/7) sekaligus memberikan pengalaman interaksi yang responsif saat digunakan untuk manajemen konfigurasi (CRUD & Reporting).

1. **Calm When Healthy, Loud When Down:** Kartu layanan yang berstatus sehat (`UP`) harus tampil tenang, bersih, dan minim distraksi animasi. Sebaliknya, layanan yang mengalami gangguan (`DEGRADED` atau `DOWN`) harus langsung menarik perhatian visual melalui efek *border glow/trail* dan kontras warna yang tegas.
2. **Zero-Reload Experience:** Perpindahan antar tab *environment* (`Production`, `Staging`, `Development`), pembukaan panel form CRUD, serta pembaruan angka metrik *real-time* berlangsung tanpa *page reload* menggunakan kombinasi WebSocket, Smart Polling, dan *View Transitions*.
3. **Print & Email Safe:** Komponen interaktif berbasis JavaScript/Framer Motion hanya digunakan pada antarmuka web, sedangkan halaman cetakan **PDF Report** dan **HTML Email** menggunakan tata letak statis murni agar 100% kompatibel dengan *Gotenberg (Headless Chromium)* dan *Email Client*.

---

## 2. Standar Tech Stack Frontend

Agar kelima referensi UI Library dapat diintegrasikan secara langsung (*copy-paste* komponen tanpa kendala kompatibilitas), standar teknologi frontend yang digunakan adalah:

| Kategori | Teknologi / Library | Fungsi Utama |
| :--- | :--- | :--- |
| **Core Framework** | **React 18+ & TypeScript** (via **Inertia.js** atau **Vite/Next.js**) | Fondasi utama pembuatan komponen, *state management*, dan *type-safety* untuk ke-7 skema protokol. |
| **Styling Engine** | **Tailwind CSS v3.4 / v4** | Utilitas styling utama dengan mode gelap (`dark` class) sebagai *default*. |
| **Animation Engine** | **Motion (`framer-motion`) + Native View Transitions API** | Mesin penggerak untuk komponen dari `motion-primitives.com`, `ui-layouts.com`, dan `transitions.dev`. |
| **Charting Library** | **Apache ECharts** (atau **ApexCharts**) | Merender grafik *time-series* (Response Time, SNMP Traffic, CPU/RAM/Disk Beszel) berbasis *Canvas*. |
| **Iconography** | **Lucide React** (`lucide-react`) | Ikon vektor konsisten untuk Router, Switch, Server, Database, Domain, SSL, Mail, dan S3. |
| **Form & Validation** | **React Hook Form + Zod** | Validasi skema form CRUD yang berubah secara dinamis sesuai pilihan protokol target. |

---

## 3. Sistem Warna & Tipografi (Design Tokens)

### 3.1 Palet Warna Tema Gelap (NOC Dark Theme)
Gunakan variabel warna Tailwind berikut secara konsisten di seluruh halaman dashboard dan modal CRUD:

| Elemen UI | Kelas Tailwind | Kode Hex | Keterangan Penggunaan |
| :--- | :--- | :--- | :--- |
| **Main Background** | `bg-zinc-950` / `bg-slate-950` | `#09090b` | Latar belakang utama aplikasi (hemat daya & ramah mata di ruang gelap). |
| **Card Surface** | `bg-zinc-900/70 backdrop-blur-md` | `#18181b` (70%) | Permukaan kartu target monitoring dengan efek *glassmorphism* tipis. |
| **Card Border (Default)** | `border-zinc-800/80` | `#27272a` | Garis tepi kartu saat layanan dalam kondisi normal. |
| **Text Primary** | `text-zinc-100` | `#f4f4f5` | Judul layanan, angka metrik utama, dan nama server. |
| **Text Secondary / Muted**| `text-zinc-400` | `#a1a1aa` | Label alamat IP, URL endpoint, nama protokol, dan *timestamp*. |

### 3.2 Warna Semantik Status Monitoring
| Status | Warna Utama | Kelas Badge & Indikator | Aturan Visual |
| :--- | :--- | :--- | :--- |
| **UP / HEALTHY** | **Emerald (Hijau)** | `text-emerald-400 bg-emerald-500/10 border-emerald-500/20` | Titik hijau berdenyut halus (*pulse dot*), kartu tanpa efek *glow* berlebih. |
| **DEGRADED / WARNING** | **Amber (Kuning/Oranye)**| `text-amber-400 bg-amber-500/10 border-amber-500/30` | Dipicu jika *latency* tinggi, Disk/RAM > 80%, atau masa aktif Domain/SSL < 14 hari. |
| **DOWN / CRITICAL** | **Rose / Red (Merah)** | `text-rose-400 bg-rose-500/15 border-rose-500/50` | Dipicu setelah gagal 3x berturut-turut. Mengaktifkan **Red Border Trail** pada kartu. |
| **MAINTENANCE / PAUSED**| **Sky / Slate (Biru Abu)** | `text-sky-400 bg-sky-500/10 border-sky-500/20` | Saat target dijeda manual lewat *Toggle Switch*. Pengecekan & alert dinonaktifkan. |

### 3.3 Standar Tipografi
* **Font UI & Heading:** `Inter` atau `Plus Jakarta Sans` — digunakan untuk navigasi, judul kartu, tabel, dan label form.
* **Font Angka, IP, & Log:** `JetBrains Mono` atau `Geist Mono` (`font-mono tabular-nums`) — **Wajib** diterapkan pada seluruh alamat IP, Port, persentase SLA (`99.98%`), *response time* (`124 ms`), sisa hari SSL, dan log terminal agar lebar digit angka selalu sejajar.

---

## 4. Katalog & Pemetaan 5 Sumber UI Library

Berikut adalah panduan teknis komponen mana yang diambil dari masing-masing website referensi dan di mana komponen tersebut dipasang:

### 4.1 `ui-layouts.com` — Kerangka Tata Letak & Navigasi
Digunakan untuk membangun struktur makro halaman, *grid* kartu, dan panel samping:
1. **Bento Grid Layout:**
   * Dipakai pada **Executive NOC Overview** dan **Dashboard 3 (Mail, S3, Beszel)** untuk menyusun kartu dengan ukuran bervariasi (misal: kartu lebar `col-span-2` untuk grafik beban server Beszel berdampingan dengan kartu `col-span-1` untuk status port Mail Server dan kapasitas *bucket* S3).
2. **Animated Pill Tabs:**
   * Dipakai pada bagian atas **Dashboard 2 (Aplikasi, Domain, Database)** untuk memfilter tampilan berdasarkan environment: `All`, `Production`, `Staging`, dan `Development`.
3. **Vaul Drawer / Slide-Over Sheet:**
   * Panel geser dari sisi kanan layar (`w-full max-w-xl`) untuk menampung form **CRUD Tambah/Edit Target** dan **CRUD Schedule Run** sehingga admin tidak kehilangan konteks dashboard saat mengubah konfigurasi.

### 4.2 `motion-primitives.com` — Mikro-Interaksi Metrik & Efek Insiden
Digunakan untuk menghidupkan perubahan data *real-time* dari WebSocket/Polling:
1. **`SlidingNumber` / `AnimatedNumber`:**
   * Dipasang pada angka **Response Time (`ms`)**, **SLA Uptime (`%`)**, **CPU/RAM Usage (`%`)**, dan **Total Layanan UP/DOWN**. Saat data baru masuk setiap 30 detik, angka akan bergulir secara vertikal dengan mulus.
2. **`BorderTrail` & `GlowEffect`:**
   * **Khusus Status `DOWN` / `CRITICAL`:** Dipasang mengelilingi kartu layanan yang sedang mati (dengan warna `rose-500`). Efek cahaya berjalan di garis tepi kartu ini membuat tim IT langsung menyadari adanya insiden dari seberang ruangan.
3. **`TransitionPanel` / `AnimatedGroup`:**
   * Dipasang di dalam **Form CRUD Target**. Ketika pengguna mengganti jenis target di *dropdown* (misal dari `Network ICMP` ke `Database` atau `S3 Storage`), *field* input spesifik protokolnya akan berganti dengan efek *smooth height & fade transition*.
4. **`MorphingDialog` / `Accordion`:**
   * Digunakan untuk membuka pop-up detail **Incident Timeline** dan riwayat pesan error (`SELECT 1 timeout`, `SSL Handshake Failed`, atau `DNSBL Listed`).

### 4.3 `beautifului.dev` — Kartu Monitoring, Badge, & Tabel Data
Digunakan untuk estetika permukaan komponen (*surface styling*) dan penyajian informasi padat:
1. **Glassmorphic Monitoring Cards:**
   * Desain kartu target dengan *header* ikon protokol, indikator tren *latency*, deretan **60 bar Heartbeat**, dan *footer* waktu pengecekan terakhir (`Last check: 12s ago`).
2. **Countdown Progress Cards (Khusus Domain & SSL):**
   * Kartu dengan bilah progres horizontal yang berubah warna berdasarkan sisa hari masa aktif sertifikat SSL dan Domain (`> 30 hari` Hijau, `14–30 hari` Kuning, `< 14 hari` Merah).
3. **Protocol & Delivery Badges:**
   * *Pill badge* dengan aksen titik warna dan border tipis untuk menandai protokol (`ICMP`, `SNMP v3`, `cURL /health`, `WHOIS+SSL`, `Native DB`, `SMTP+DNSBL`, `S3 SDK`, `Beszel API`) serta status di tabel `email_delivery_logs` (`Sent`, `Queued`, `Retrying`, `Failed`).
4. **Command Palette (`Ctrl + K` / `Cmd + K`):**
   * Modal pencarian cepat bergaya *spotlight* untuk mencari nama server, IP address, domain, atau langsung melompat ke menu pembuatan report.

### 4.4 `uiverse.io` — Elemen Kontrol Mikro, Toggle, & Loader (Filter: Tailwind CSS)
Digunakan untuk elemen interaktif fungsional berukuran kecil:
1. **Heartbeat Pulsing Dot:**
   * Indikator lingkaran hijau dengan efek `animate-ping` berlapis di sudut kanan atas setiap kartu layanan yang berstatus `UP`.
2. **Tactile Toggle Switches:**
   * Dipasang pada:
     * Tombol **Maintenance Mode (Pause/Resume)** di setiap kartu target.
     * Tombol **Enable/Disable Schedule** pada tabel daftar *Schedule Run*.
     * Tombol **"Send Only If Issues Found"** pada pengaturan jadwal email.
3. **Radar / Dual-Ring Spinner:**
   * Animasi *loading* khusus yang muncul di dalam tombol saat pengguna mengklik:
     * Tombol **"Test Connection"** (saat menguji koneksi DB, S3, atau API Beszel di form CRUD).
     * Tombol **"Test Run & Send Email Now"** (saat memicu pembuatan PDF & pengiriman email secara manual).

### 4.5 `transitions.dev` — Transisi Perpindahan Halaman & Filter State
Digunakan untuk menjaga kontinuitas visual menggunakan *View Transitions API*:
1. **Card-to-Detail Shared Transition (`view-transition-name`):**
   * Saat pengguna mengklik kartu **Server Beszel** atau **Router Utama**, kartu tersebut bertransisi secara mulus (*morphing*) menjadi halaman penuh yang menampilkan grafik historis CPU, RAM, Disk, Docker Container, dan log insiden.
2. **Layout Grid Reordering:**
   * Saat pengguna menekan tombol filter **"Tampilkan Hanya yang Bermasalah (Down/Degraded)"**, kartu-kartu di dalam *grid* bergeser mengisi posisi baru secara halus tanpa efek berkedip (*flicker*).

---

## 5. Spesifikasi Tata Letak per Halaman (Screen Blueprints)

```
+-----------------------------------------------------------------------------------+
| [Logo] INTERNAL MONITOR   [Search Ctrl+K]          [NOC TV Mode] [+ Tambah Target]|
+-----------------------------------------------------------------------------------+
| EXECUTIVE SUMMARY BAR (Bento Top Row - ui-layouts + motion-primitives)            |
| +-----------------+ +-----------------+ +-----------------+ +-------------------+ |
| | TOTAL TARGET: 48| | UP / NORMAL: 45 | | DEGRADED: 2     | | DOWN / CRIT: 1    | |
| | Global SLA 99.9%| | [Pulse Green]   | | SSL <14d / Slow | | [Red BorderTrail] | |
| +-----------------+ +-----------------+ +-----------------+ +-------------------+ |
+-----------------------------------------------------------------------------------+
| NAVIGATION TABS (ui-layouts Animated Tabs):                                       |
| (1) Network Internal | (2) App, Domain & DB [Prod|Stg|Dev] | (3) Mail, S3 & Beszel|
| (4) CRUD & Targets   | (5) Schedule Run & Reports                                 |
+-----------------------------------------------------------------------------------+
```

### 5.1 Dashboard 1: Network Internal (Router, Switch, AP, Server Internal)
* **Baris Atas (Topology & Traffic Summary):** Grafik *Area Chart* (ECharts) yang menampilkan agregat trafik *Inbound/Outbound (Mbps)* dari SNMP Router utama.
* **Grid Kartu Perangkat (`grid-cols-1 md:grid-cols-2 xl:grid-cols-3`):**
  * **Header Kartu:** Ikon perangkat (`Router` / `Network` / `Wifi` / `Server`), Nama Perangkat, IP Address (`font-mono`), dan *Heartbeat Dot* (`uiverse.io`).
  * **Body Kartu:**
    * Angka **Latency (`ms`)** & **Packet Loss (`%`)** menggunakan `SlidingNumber` (`motion-primitives.com`).
    * Bar progres kecil untuk **CPU Load (%)** dan **Memory Usage (%)** yang diambil dari SNMP.
    * **60-Bar Uptime Heartbeat:** Deretan 60 balok vertikal yang merepresentasikan status 60 menit terakhir.
  * **Footer Kartu:** Status port krusial & tombol cepat *Maintenance Toggle*.

### 5.2 Dashboard 2: Aplikasi Website, Domain Kantor, & Database
* **Sub-Filter Environment (`ui-layouts.com`):** Tab pemilih **`All`**, **`Production`**, **`Staging`**, dan **`Development`**.
  * *Aturan Visual:* Jika layanan `Development` mati, tampilkan *badge* merah biasa **tanpa** menyalakan sirine atau `BorderTrail` darurat agar tidak mengganggu fokus dari layanan `Production`.
* **Seksi A — Kartu Aplikasi Website (cURL & `/health`):**
  * Menampilkan URL endpoint, badge HTTP Status (`200 OK` / `503`), grafik mini (*sparkline*) *Response Time (ms)*, dan indikator status JSON `/health`.
* **Seksi B — Kartu Domain & SSL (`beautifului.dev`):**
  * Menampilkan Nama Domain, *Registrar*, tanggal kedaluwarsa WHOIS, serta *Issuer* SSL.
  * Dilengkapi **Dual Countdown Bar**: satu bar untuk **Domain Expiry (`142 Hari`)** dan satu bar untuk **SSL Certificate Expiry (`18 Hari`)**.
* **Seksi C — Kartu Database (`PostgreSQL`, `MySQL`, `SQL Server`):**
  * Menampilkan Host:Port, badge environment, waktu eksekusi kueri `SELECT 1` (`4.2 ms`), dan jumlah koneksi aktif.

### 5.3 Dashboard 3: Mail Server, Object Storage (S3), & Server Beszel
* **Seksi A — Panel Mail Server:**
  * Kartu status port (`SMTP 25`, `SMTPS 465`, `Submission 587`, `IMAPS 993`) beserta waktu respons *banner*.
  * **DNSBL Reputation Card:** Indikator apakah IP publik Mail Server bersih (`Listed in 0/50 Blacklists` - Hijau) atau terkena *blacklist* (`Spamhaus Listed` - Merah).
* **Seksi B — Panel Object Storage (S3):**
  * Kartu status koneksi `HeadBucket` (Normal/Forbidden/Timeout), *Response Time*, nama *Endpoint/Region*, serta *Radial Gauge* pemakaian kapasitas *bucket*.
* **Seksi C — Bento Grid Server Beszel (Integrasi PocketBase API):**
  * Setiap server dari Beszel ditampilkan sebagai kartu lebar yang memuat **3 Radial Gauge / Progress Bar** (*CPU Usage %*, *RAM Usage %*, *Disk Usage %*), *Network I/O*, serta badge jumlah **Docker Containers Running** di dalam server tersebut.

### 5.4 Halaman / Drawer CRUD Dinamis (7 Protokol)
Saat tombol **"+ Tambah Target"** diklik, *Slide-over Drawer* (`ui-layouts.com`) terbuka dengan susunan form:
1. **Informasi Umum:** Nama Target, Grup Dashboard, Environment (`Prod`/`Staging`/`Dev`), Interval Pengecekan (`30s`, `1m`, `5m`, `12h`, `24h`), dan *Failure Threshold* (`3x Retry`).
2. **Panel Dinamis Protokol (`motion-primitives` `TransitionPanel`):**
   * Pilih `Network` $\rightarrow$ Muncul input IP Address, SNMP Version (`v2c`/`v3`), dan SNMP Community/Auth (dienkripsi).
   * Pilih `Website` $\rightarrow$ Muncul input URL, Method (`GET`/`POST`), Expected HTTP Code (`200`), dan JSON Key Check.
   * Pilih `Domain` $\rightarrow$ Muncul input Nama Domain, Port TLS (`443`), dan batas hari peringatan (`H-30`, `H-14`, `H-7`).
   * Pilih `Database` $\rightarrow$ Muncul pilihan Driver (`pgsql`/`mysql`), Host, Port, Username, Password (dienkripsi AES-256), dan Database Name.
   * Pilih `Mail Server` $\rightarrow$ Muncul input Hostname, Daftar Port (`25,465,587,993`), dan opsi Cek Blacklist DNSBL.
   * Pilih `S3 Storage` $\rightarrow$ Muncul input Endpoint URL, Region, Bucket Name, Access Key, dan Secret Key (dienkripsi AES-256).
   * Pilih `Beszel Server` $\rightarrow$ Muncul input Beszel Hub URL, System ID, dan API Token (dienkripsi AES-256).
3. **Action Bar Bawah:** Tombol **"Test Connection"** (dengan *Radar Spinner* dari `uiverse.io` yang menampilkan hasil tes langsung di dalam drawer) dan tombol **"Simpan Target"**.

### 5.5 Halaman Schedule Run, Generate Report & Log Email
* **Daftar Jadwal Aktif (Schedule Cards):**
  * Menampilkan kartu jadwal **Harian (08:00 WIB)**, **Mingguan (Senin 08:00 WIB)**, dan **Bulanan (Tanggal 1, 08:00 WIB)**.
  * Setiap kartu dilengkapi *Toggle Switch* (`uiverse.io`) untuk mengaktifkan/menonaktifkan jadwal, daftar *chip* email penerima, badge format (`PDF` + `XLSX` + `HTML`), serta tombol **"Run & Send Now"** untuk pengujian instan.
* **Tabel Riwayat Laporan & Pengiriman Email (`beautifului.dev`):**
  * Menampilkan waktu eksekusi, periode laporan, status pengiriman (`Sent`, `Retrying 1/3`, `Failed`), tombol **Download PDF/Excel**, dan tombol **Resend Email**.

---

## 6. Standar Desain Template Laporan (PDF & Email)

Untuk menjamin keandalan fitur **Generate Report**, tampilan laporan memiliki aturan desain khusus yang terpisah dari UI Web utama:

### 6.1 Desain Dokumen PDF (Dicetak via Gotenberg)
* **Tema Warna:** **Light Mode Profesional (Kertas Putih `#FFFFFF`, Teks `#0F172A`)** agar hemat tinta jika dicetak fisik dan nyaman dibaca sebagai dokumen lampiran manajemen.
* **Ukuran & Margin:** Kertas `A4 Portrait`, margin `15mm`, menggunakan penomoran halaman otomatis di bagian *footer*.
* **Struktur Halaman PDF:**
  1. **Header Dokumen:** Logo Perusahaan, Judul Laporan (*Laporan Harian Operasional* / *Laporan Mingguan Performa* / *Laporan Bulanan SLA Eksekutif*), Rentang Tanggal, dan Waktu Cetak.
  2. **Executive SLA Scorecard:** 4 Kotak ringkasan besar berisi **Persentase SLA Uptime Global (`99.96%`)**, **Total Insiden**, **Total Durasi Downtime (Menit)**, dan **MTTR (Rata-rata Waktu Pemulihan)**.
  3. **Grafik Tren Statis:** Grafik garis *Response Time* dan diagram batang *Uptime per Kategori* yang dirender menjadi **SVG statis** (tanpa animasi JS).
  4. **Tabel Detail per Kategori:** Tabel pencapaian SLA untuk Network, Aplikasi (Prod/Stg/Dev), Database, Mail, S3, dan Beszel, ditambah tabel **Peringatan Kedaluwarsa Domain & SSL**.

### 6.2 Desain Badan Email (HTML Email Summary)
* Dibangun menggunakan struktur **Tabel HTML (`<table>`) dengan Inline CSS** (melalui *Maizzle* atau *MJML*) dengan lebar maksimal `600px`.
* Memuat **Banner Status Warna** di bagian paling atas email:
  * **Hijau:** *"Semua Sistem Berjalan Normal (SLA 100%)"*
  * **Kuning/Merah:** *"Terdapat 2 Insiden Downtime & 1 Domain Akan Kedaluwarsa"*
* Menyertakan tabel ringkas 5 metrik utama langsung di badan email sehingga manajemen bisa membaca intisarinya dari layar HP tanpa wajib membuka file lampiran PDF/Excel.

---

## 7. Struktur Folder Komponen Frontend (Rekomendasi)

```text
resources/js/ (atau src/)
├── components/
│   ├── ui-layouts/                # Komponen adaptasi dari ui-layouts.com
│   │   ├── BentoGrid.tsx          # Grid kartu dashboard & NOC
│   │   ├── AnimatedTabs.tsx       # Filter tab Prod / Staging / Dev
│   │   └── SlideOverDrawer.tsx    # Drawer untuk CRUD Target & Schedule Run
│   ├── motion-primitives/         # Komponen adaptasi dari motion-primitives.com
│   │   ├── SlidingNumber.tsx      # Animasi angka Latency, SLA %, CPU/RAM
│   │   ├── BorderTrail.tsx        # Efek border merah berjalan saat status DOWN
│   │   └── TransitionPanel.tsx    # Transisi form dinamis 7 protokol
│   ├── beautiful-ui/              # Komponen adaptasi dari beautifului.dev
│   │   ├── StatusCard.tsx         # Kartu utama target + 60-bar heartbeat
│   │   ├── ExpiryCountdownCard.tsx# Kartu khusus WHOIS Domain & SSL Expiry
│   │   ├── ProtocolBadge.tsx      # Badge protokol & status email log
│   │   └── CommandPalette.tsx     # Pencarian cepat Ctrl+K
│   └── uiverse/                   # Komponen adaptasi dari uiverse.io (Tailwind)
│       ├── PulseDot.tsx           # Titik indikator live status UP/DOWN
│       ├── MaintenanceSwitch.tsx  # Toggle switch Pause/Resume & Schedule
│       └── RadarButtonLoader.tsx  # Tombol dengan animasi saat Test Connection
├── pages/
│   ├── Dashboard/
│   │   ├── NocOverview.tsx        # Layar gabungan untuk TV Monitoring
│   │   ├── NetworkDashboard.tsx   # Dashboard 1: Router, Switch, AP, Server
│   │   ├── AppDbDomainDashboard.tsx # Dashboard 2: Web, Domain/SSL, Database
│   │   └── MailS3BeszelDashboard.tsx # Dashboard 3: Mail, S3, Beszel API
│   ├── Targets/
│   │   ├── TargetIndex.tsx        # Tabel manajemen semua target
│   │   └── TargetFormDrawer.tsx   # Form CRUD dinamis 7 protokol + Test Connection
│   └── Reports/
│       ├── ScheduleManager.tsx    # CRUD Schedule Run (Harian, Mingguan, Bulanan)
│       ├── EmailDeliveryLogs.tsx  # Log pengiriman & tombol Run/Resend Now
│       └── PdfPrintTemplate.tsx   # Template statis khusus untuk Gotenberg PDF
```

---

## 8. Guardrails Performa & Aksesibilitas (Wajib Dipatuhi)

1. **Aturan CPU/GPU Layar NOC 24/7:**
   * **DILARANG** menyalakan efek `BorderTrail` (`motion-primitives.com`) atau animasi bayangan CSS terus-menerus pada kartu yang berstatus `UP`. Efek `BorderTrail` hanya boleh di-mount (`conditional rendering`) ketika `target.status === 'DOWN'`.
2. **Page Visibility Throttling:**
   * Saat tab browser diminimalkan atau tidak terlihat di layar (`document.hidden === true`), hentikan sementara semua animasi `SlidingNumber` dan perlambat interval *polling* grafik agar tidak menguras memori browser pengguna.
3. **Standar Kontras & Dukungan Buta Warna:**
   * Jangan pernah membedakan status `UP`, `DEGRADED`, dan `DOWN` **hanya** menggunakan warna hijau, kuning, dan merah. Selalu sertakan **ikon pendamping** (`CheckCircle2` untuk UP, `AlertTriangle` untuk Degraded, `XOctagon` untuk Down) serta label teks eksplisit pada setiap *badge*.
4. **Isolasi CSS Cetakan PDF:**
   * Halaman `/reports/pdf-preview` yang diakses oleh kontainer **Gotenberg** harus memuat *stylesheet* khusus tanpa *library* animasi (`framer-motion` dinonaktifkan) dan menyertakan atribut `data-pdf-ready="true"` setelah grafik SVG selesai dirender.