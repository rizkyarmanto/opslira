package reportrender

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// sampleData mirrors one buildReportData() payload; row 2 carries markup to
// prove user strings are escaped in HTML (but preserved verbatim in XLSX).
func sampleData(periodType string) Data {
	return Data{
		PeriodType:   periodType,
		PeriodLabel:  "01 Oct 2026 to 02 Oct 2026",
		GeneratedAt:  "02 Oct 2026 08:00:00 WIB",
		ScheduleName: "Morning Ops",
		Rows: []Row{
			{
				Name:                 "api-prod-01",
				DashboardGroup:       "API",
				Environment:          "PRODUCTION",
				TargetType:           "HTTP",
				UptimePercentage:     99.5,
				MTTRSeconds:          618.5,
				MTBFSeconds:          84000,
				TotalDowntimeSeconds: 1237,
				IncidentCount:        2,
				TotalChecks:          10000,
			},
			{
				Name:                 `mail-<script>alert("x")</script>`,
				DashboardGroup:       "Mail",
				Environment:          "STAGING",
				TargetType:           "SMTP",
				UptimePercentage:     98.5,
				MTTRSeconds:          300,
				MTBFSeconds:          42000,
				TotalDowntimeSeconds: 600,
				IncidentCount:        2,
				TotalChecks:          5000,
			},
		},
		Summary: Summary{
			GlobalSLA:            99.23,
			TotalIncidents:       4,
			TotalDowntimeSeconds: 1837,
			MTTRSeconds:          459.25,
			TotalTargets:         2,
		},
	}
}

func TestRenderHTMLAllPeriods(t *testing.T) {
	tests := []struct {
		period string
		h1     string
	}{
		{"DAILY", "Daily Operations Report"},
		{"WEEKLY", "Weekly Performance Report"},
		{"MONTHLY", "Monthly Executive SLA Report"},
	}
	for _, tc := range tests {
		t.Run(tc.period, func(t *testing.T) {
			out, err := RenderHTML(sampleData(tc.period))
			if err != nil {
				t.Fatalf("RenderHTML: %v", err)
			}
			for _, want := range []string{
				"<title>" + tc.period + " Report | Opslira</title>",
				"<h1>" + tc.h1 + "</h1>",
				"<strong>Period:</strong> 01 Oct 2026 to 02 Oct 2026",
				"<strong>Generated:</strong> 02 Oct 2026 08:00:00 WIB",
				// Scorecard: labels + formatted values.
				"Global SLA Uptime",
				"99.23%",
				`class="value good"`,
				"Total Incidents",
				"30.6 min",
				"7.7 min",
				// Static SVG chart and table sections.
				"Uptime by Category",
				`<svg width="600" height="140"`,
				">API</text>",
				">Mail</text>",
				"99.50%", // API group average
				"98.50%", // Mail group average
				"SLA Achievement Table per Target",
				"<th>Target</th>",
				"<th>Downtime (s)</th>",
				"<th>MTTR (s)</th>",
				// Row values.
				"<td>api-prod-01</td>",
				"<td>PRODUCTION</td>",
				"<td class=\"num\">99.50</td>",
				"<td class=\"num\">1237</td>",
				"<td class=\"num\">618.5</td>",
				"<td class=\"num\">98.50</td>",
				"<td class=\"num\">300.0</td>",
				// Escaped user string.
				"&lt;script&gt;",
				"Opslira Monitoring.",
			} {
				if !strings.Contains(out, want) {
					t.Errorf("output missing %q", want)
				}
			}
			// The markup in the row name must never reach the document raw;
			// the templates contain no <script>/<img> of their own.
			if strings.Contains(out, "<script") || strings.Contains(out, "<img") {
				t.Errorf("user string was not escaped")
			}
		})
	}
}

func TestRenderHTMLBadPeriod(t *testing.T) {
	for _, period := range []string{"HOURLY", "", "QUARTERLY", "daily report"} {
		out, err := RenderHTML(Data{PeriodType: period})
		if err == nil {
			t.Errorf("period %q: expected an error", period)
		}
		if out != "" {
			t.Errorf("period %q: expected empty output, got %d bytes", period, len(out))
		}
	}
}

func TestBuildPDF(t *testing.T) {
	const htmlBody = `<html><body>report body</body></html>`
	got := struct {
		path        string
		contentType string
		fileName    string
		fileBody    string
		fields      map[string]string
	}{fields: map[string]string{}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path = r.URL.Path
		got.contentType = r.Header.Get("Content-Type")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		for _, k := range []string{"marginTop", "marginBottom", "marginLeft", "marginRight", "printBackground"} {
			got.fields[k] = r.FormValue(k)
		}
		f, hdr, err := r.FormFile("files")
		if err != nil {
			t.Errorf(`missing multipart "files" part: %v`, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer f.Close()
		got.fileName = hdr.Filename
		b, err := io.ReadAll(f)
		if err != nil {
			t.Errorf("read files part: %v", err)
		}
		got.fileBody = string(b)
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4 fake"))
	}))
	defer srv.Close()

	// Trailing slash exercises the URL trimming.
	out, err := BuildPDF(context.Background(), srv.URL+"/", htmlBody)
	if err != nil {
		t.Fatalf("BuildPDF: %v", err)
	}
	if string(out) != "%PDF-1.4 fake" {
		t.Errorf("pdf bytes = %q", out)
	}
	if got.path != "/forms/chromium/convert/html" {
		t.Errorf("request path = %q", got.path)
	}
	if !strings.HasPrefix(got.contentType, "multipart/form-data;") {
		t.Errorf("content-type = %q", got.contentType)
	}
	if got.fileName != "index.html" {
		t.Errorf(`files filename = %q, want "index.html"`, got.fileName)
	}
	if got.fileBody != htmlBody {
		t.Errorf("files body = %q", got.fileBody)
	}
	for k, want := range map[string]string{
		"marginTop":       "0.59",
		"marginBottom":    "0.59",
		"marginLeft":      "0.59",
		"marginRight":     "0.59",
		"printBackground": "true",
	} {
		if got.fields[k] != want {
			t.Errorf("%s = %q, want %q", k, got.fields[k], want)
		}
	}
}

func TestBuildPDFErrors(t *testing.T) {
	// Unconfigured endpoint: unavailable, and the phrase callers match on.
	_, err := BuildPDF(context.Background(), "", "<html></html>")
	if !errors.Is(err, ErrGotenbergUnavailable) {
		t.Errorf("empty URL: errors.Is(ErrGotenbergUnavailable) = false, err = %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "gotenberg unavailable") {
		t.Errorf("empty URL: error %v missing %q", err, "gotenberg unavailable")
	}

	// Reachable host answering with a non-2xx status is NOT unavailable.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer srv.Close()
	if _, err := BuildPDF(context.Background(), srv.URL, "<html></html>"); err == nil {
		t.Error("HTTP 502: expected an error")
	} else if errors.Is(err, ErrGotenbergUnavailable) {
		t.Errorf("HTTP 502 must not be ErrGotenbergUnavailable: %v", err)
	}

	// Connection-level failure: dial error carries the unavailable sentinel.
	// Port 1 on loopback refuses immediately; no external network involved.
	if _, err := BuildPDF(context.Background(), "http://127.0.0.1:1", "<html></html>"); !errors.Is(err, ErrGotenbergUnavailable) {
		t.Errorf("dial failure: errors.Is(ErrGotenbergUnavailable) = false, err = %v", err)
	}
}

func TestBuildXLSX(t *testing.T) {
	blob, err := BuildXLSX(sampleData("DAILY"))
	if err != nil {
		t.Fatalf("BuildXLSX: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(blob))
	if err != nil {
		t.Fatalf("open xlsx: %v", err)
	}
	defer f.Close()
	sheet := f.GetSheetName(0)

	for cell, want := range map[string]string{
		// Summary block.
		"A1": "Summary",
		"A2": "Period", "B2": "01 Oct 2026 to 02 Oct 2026",
		"A3": "Global SLA (%)", "B3": "99.23",
		"A4": "Total Incidents", "B4": "4",
		"A5": "Total Downtime (minutes)", "B5": "30.6",
		"A6": "MTTR (minutes)", "B6": "7.7",
		// Blank spacer row, then headers.
		"A7": "",
		"A8": "Target", "B8": "Group", "C8": "Environment",
		"D8": "SLA (%)", "E8": "Incidents", "F8": "Downtime (s)",
		// One row per target; the markup row stays verbatim in the workbook.
		"A9":  "api-prod-01",
		"B9":  "API",
		"C9":  "PRODUCTION",
		"D9":  "99.5",
		"E9":  "2",
		"F9":  "1237",
		"A10": `mail-<script>alert("x")</script>`,
		"B10": "Mail",
		"C10": "STAGING",
		"D10": "98.5",
		"E10": "2",
		"F10": "600",
		// No extra rows.
		"A11": "",
		"A12": "",
	} {
		got, err := f.GetCellValue(sheet, cell)
		if err != nil {
			t.Fatalf("GetCellValue %s: %v", cell, err)
		}
		if got != want {
			t.Errorf("%s = %q, want %q", cell, got, want)
		}
	}
}
