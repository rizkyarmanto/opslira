<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <title>{{ $period_type }} Report | Opslira</title>
    <style>
        * { box-sizing: border-box; }
        body.pdf-body {
            margin: 0;
            padding: 0;
            background: #ffffff;
            color: #0f172a;
            font-family: Inter, Helvetica, Arial, sans-serif;
            font-size: 11px;
            line-height: 1.45;
        }
        .header { display: flex; align-items: center; justify-content: space-between; border-bottom: 2px solid #0f172a; padding-bottom: 12px; margin-bottom: 18px; }
        .header h1 { font-size: 20px; margin: 0; font-weight: 800; }
        .header .meta { text-align: right; color: #475569; font-size: 10px; }
        .brand { display: flex; align-items: center; gap: 8px; }
        .brand img { width: 32px; height: 32px; border-radius: 6px; object-fit: cover; }

        .scorecard { display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; margin-bottom: 20px; }
        .score { border: 1px solid #e2e8f0; border-radius: 8px; padding: 12px; }
        .score .label { font-size: 9px; text-transform: uppercase; letter-spacing: 0.06em; color: #64748b; }
        .score .value { font-size: 20px; font-weight: 800; margin-top: 4px; }
        .score .value.good { color: #047857; }
        .score .value.warn { color: #b45309; }
        .score .value.bad { color: #be123c; }

        h2 { font-size: 13px; margin: 18px 0 8px; border-left: 3px solid #0f172a; padding-left: 8px; }
        table { width: 100%; border-collapse: collapse; font-size: 10px; }
        th, td { border: 1px solid #e2e8f0; padding: 6px 8px; text-align: left; }
        th { background: #f1f5f9; font-weight: 700; }
        td.num { text-align: right; font-variant-numeric: tabular-nums; }
        .footer { margin-top: 24px; border-top: 1px solid #e2e8f0; padding-top: 8px; font-size: 9px; color: #64748b; text-align: center; }
    </style>
</head>
<body class="pdf-body" data-pdf-ready="true">
    <div class="header">
        <div class="brand">
            <svg width="32" height="32" viewBox="0 0 32 32" xmlns="http://www.w3.org/2000/svg">
                <rect width="32" height="32" rx="6" fill="#0f172a"/>
                <path d="M9 22V10h3.2v9.2H21V22H9z" fill="#9fe870"/>
                <circle cx="22" cy="11.5" r="2.5" fill="#9fe870"/>
            </svg>
            <div>
                <h1>{{ ucfirst(strtolower($period_type)) }} Operations Report</h1>
                <div style="font-size:10px;color:#64748b;">Internal Infrastructure Monitoring System</div>
            </div>
        </div>
        <div class="meta">
            <div><strong>Period:</strong> {{ $period_label }}</div>
            <div><strong>Generated:</strong> {{ $generated_at }}</div>
        </div>
    </div>

    {{-- Executive SLA Scorecard --}}
    <div class="scorecard">
        <div class="score">
            <div class="label">Global SLA Uptime</div>
            <div class="value {{ $summary['global_sla'] >= 99 ? 'good' : ($summary['global_sla'] >= 95 ? 'warn' : 'bad') }}">
                {{ number_format($summary['global_sla'], 2) }}%
            </div>
        </div>
        <div class="score">
            <div class="label">Total Incidents</div>
            <div class="value">{{ $summary['total_incidents'] }}</div>
        </div>
        <div class="score">
            <div class="label">Total Downtime</div>
            <div class="value">{{ number_format($summary['total_downtime_seconds'] / 60, 1) }} min</div>
        </div>
        <div class="score">
            <div class="label">MTTR</div>
            <div class="value">{{ number_format($summary['mttr_seconds'] / 60, 1) }} min</div>
        </div>
    </div>

    {{-- Static SVG uptime bar chart per category --}}
    <h2>Uptime by Category</h2>
    @php
        $byGroup = collect($rows)->groupBy('dashboard_group');
        $maxBars = max(1, $byGroup->count());
        $barW = (int) (600 / $maxBars) - 20;
    @endphp
    <svg width="600" height="140" xmlns="http://www.w3.org/2000/svg">
        <line x1="0" y1="120" x2="600" y2="120" stroke="#cbd5e1" stroke-width="1"/>
        @foreach ($byGroup as $group => $groupRows)
            @php
                $avg = round(collect($groupRows)->avg('uptime_percentage'), 2);
                $h = (int) (($avg / 100) * 100);
                $x = 20 + ($loop->index * ($barW + 20));
            @endphp
            <rect x="{{ $x }}" y="{{ 120 - $h }}" width="{{ $barW }}" height="{{ $h }}" fill="#047857" rx="3"/>
            <text x="{{ $x + $barW / 2 }}" y="136" text-anchor="middle" font-size="9" fill="#475569">{{ $group }}</text>
            <text x="{{ $x + $barW / 2 }}" y="{{ 114 - $h }}" text-anchor="middle" font-size="9" fill="#0f172a">{{ $avg }}%</text>
        @endforeach
    </svg>

    {{-- Detail table per target --}}
    <h2>SLA Achievement Table per Target</h2>
    <table>
        <thead>
            <tr>
                <th>Target</th>
                <th>Group</th>
                <th>Environment</th>
                <th>SLA (%)</th>
                <th>Incidents</th>
                <th>Downtime (s)</th>
                <th>MTTR (s)</th>
            </tr>
        </thead>
        <tbody>
            @foreach ($rows as $row)
                <tr>
                    <td>{{ $row['name'] }}</td>
                    <td>{{ $row['dashboard_group'] }}</td>
                    <td>{{ $row['environment'] }}</td>
                    <td class="num">{{ number_format($row['uptime_percentage'], 2) }}</td>
                    <td class="num">{{ $row['incident_count'] }}</td>
                    <td class="num">{{ $row['total_downtime_seconds'] }}</td>
                    <td class="num">{{ number_format($row['mttr_seconds'], 1) }}</td>
                </tr>
            @endforeach
        </tbody>
    </table>

    <div class="footer">
        Opslira Monitoring. This report is generated automatically. This document is internal.
    </div>
</body>
</html>
