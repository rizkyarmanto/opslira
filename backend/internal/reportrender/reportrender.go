// Package reportrender renders the daily/weekly/monthly HTML report templates,
// converts them to PDF through Gotenberg's Chromium endpoint, and builds XLSX
// workbooks. It ports Laravel's PdfReportBuilder/ExcelReportBuilder; the
// returned bytes belong to the caller, which owns storage.
package reportrender

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xuri/excelize/v2"

	opslira "opslira/backend"
)

// ErrGotenbergUnavailable wraps connection-level failures (and an unconfigured
// endpoint) so callers can map them to 503 while other failures stay 502.
var ErrGotenbergUnavailable = errors.New("gotenberg unavailable")

// Row is one target's line in the SLA achievement table; the JSON shape
// matches the rows built by GenerateAndSendReportJob::buildReportData.
type Row struct {
	Name                 string  `json:"name"`
	DashboardGroup       string  `json:"dashboard_group"`
	Environment          string  `json:"environment"`
	TargetType           string  `json:"target_type"`
	UptimePercentage     float64 `json:"uptime_percentage"`
	MTTRSeconds          float64 `json:"mttr_seconds"`
	MTBFSeconds          float64 `json:"mtbf_seconds"`
	TotalDowntimeSeconds int     `json:"total_downtime_seconds"`
	IncidentCount        int     `json:"incident_count"`
	TotalChecks          int     `json:"total_checks"`
}

// Summary is the executive scorecard block.
type Summary struct {
	GlobalSLA            float64 `json:"global_sla"`
	TotalIncidents       int     `json:"total_incidents"`
	TotalDowntimeSeconds int     `json:"total_downtime_seconds"`
	MTTRSeconds          float64 `json:"mttr_seconds"`
	TotalTargets         int     `json:"total_targets"`
}

// Data is the payload for RenderHTML and BuildXLSX, mirroring the array
// returned by GenerateAndSendReportJob::buildReportData.
type Data struct {
	PeriodType   string  `json:"period_type"` // DAILY|WEEKLY|MONTHLY
	PeriodLabel  string  `json:"period_label"`
	GeneratedAt  string  `json:"generated_at"`
	ScheduleName string  `json:"schedule_name"`
	Rows         []Row   `json:"rows"`
	Summary      Summary `json:"summary"`
}

// groupBar is one pre-computed bar of the static SVG uptime chart.
type groupBar struct {
	Group   string
	AvgText string
	X       int
	W       int
	H       int
	CX      int
	ValueY  int
}

// view is the template payload: the flat Data fields (promoted through the
// embedding) plus the per-category bars the SVG chart ranges over.
type view struct {
	Data
	ByGroup []groupBar
}

var (
	tmplOnce sync.Once
	tmpl     *template.Template
	tmplErr  error
)

// reportTemplates lazily parses reports/{daily,weekly,monthly}.html exactly
// once from the root embed FS.
func reportTemplates() (*template.Template, error) {
	tmplOnce.Do(func() {
		tmpl, tmplErr = template.New("daily.html").Funcs(template.FuncMap{
			// Templates receive int and float64 fields; html/template does not
			// convert between them, so these helpers normalise at call time.
			"num2":   func(v interface{}) string { return strconv.FormatFloat(toFloat(v), 'f', 2, 64) },
			"num1":   func(v interface{}) string { return strconv.FormatFloat(toFloat(v), 'f', 1, 64) },
			"divf":   func(a, b interface{}) float64 { return toFloat(a) / toFloat(b) },
			"sub120": func(h int) int { return 120 - h },
		}).ParseFS(opslira.ReportTemplates, "templates/reports/*.html")
	})
	return tmpl, tmplErr
}

// RenderHTML renders the report for data.PeriodType (DAILY|WEEKLY|MONTHLY →
// reports/<lower period_type>.html) into a standalone light-themed HTML
// document with static SVG charts; user-supplied strings are escaped by
// html/template.
func RenderHTML(data Data) (string, error) {
	period := strings.ToLower(data.PeriodType)
	if period != "daily" && period != "weekly" && period != "monthly" {
		return "", fmt.Errorf("reportrender: unknown period type %q", data.PeriodType)
	}
	t, err := reportTemplates()
	if err != nil {
		return "", fmt.Errorf("reportrender: parse report templates: %w", err)
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, period+".html", view{Data: data, ByGroup: bars(data.Rows)}); err != nil {
		return "", fmt.Errorf("reportrender: render %s report: %w", period, err)
	}
	return buf.String(), nil
}

// BuildPDF converts html to PDF via Gotenberg's Chromium endpoint and returns
// the PDF bytes. Connection-level failures are wrapped in
// ErrGotenbergUnavailable; the client times out after 300s so long renders
// survive reverse-proxy idle timeouts.
func BuildPDF(ctx context.Context, gotenbergURL, html string) ([]byte, error) {
	if gotenbergURL == "" {
		return nil, fmt.Errorf("%w: gotenberg URL is empty", ErrGotenbergUnavailable)
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("files", "index.html")
	if err != nil {
		return nil, fmt.Errorf("reportrender: multipart files part: %w", err)
	}
	if _, err := io.WriteString(fw, html); err != nil {
		return nil, fmt.Errorf("reportrender: write files part: %w", err)
	}
	for _, field := range []struct{ name, value string }{
		{"marginTop", "0.59"},
		{"marginBottom", "0.59"},
		{"marginLeft", "0.59"},
		{"marginRight", "0.59"},
		{"printBackground", "true"},
	} {
		if err := w.WriteField(field.name, field.value); err != nil {
			return nil, fmt.Errorf("reportrender: multipart field %s: %w", field.name, err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("reportrender: multipart close: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimSuffix(gotenbergURL, "/")+"/forms/chromium/convert/html", &body)
	if err != nil {
		return nil, fmt.Errorf("reportrender: build gotenberg request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := (&http.Client{Timeout: 300 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGotenbergUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("reportrender: gotenberg returned HTTP %d", resp.StatusCode)
	}
	pdf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reportrender: read gotenberg response: %w", err)
	}
	if len(pdf) == 0 {
		return nil, fmt.Errorf("reportrender: gotenberg returned an empty PDF")
	}
	return pdf, nil
}

// BuildXLSX builds the report workbook mirroring ExcelReportBuilder's CSV
// layout: a Summary block (Period, Global SLA (%), Total Incidents, Total
// Downtime (minutes), MTTR (minutes)), a blank row, then the per-target
// headers and rows.
func BuildXLSX(data Data) ([]byte, error) {
	f := excelize.NewFile()
	records := [][]interface{}{
		{"Summary"},
		{"Period", data.PeriodLabel},
		{"Global SLA (%)", data.Summary.GlobalSLA},
		{"Total Incidents", data.Summary.TotalIncidents},
		{"Total Downtime (minutes)", round1(float64(data.Summary.TotalDowntimeSeconds) / 60)},
		{"MTTR (minutes)", round1(data.Summary.MTTRSeconds / 60)},
		{}, // blank spacer row
		{"Target", "Group", "Environment", "SLA (%)", "Incidents", "Downtime (s)"},
	}
	for _, r := range data.Rows {
		records = append(records, []interface{}{
			r.Name, r.DashboardGroup, r.Environment,
			r.UptimePercentage, r.IncidentCount, r.TotalDowntimeSeconds,
		})
	}
	for i, record := range records {
		if len(record) == 0 {
			continue // the spacer row stays empty
		}
		cell, err := excelize.CoordinatesToCellName(1, i+1)
		if err != nil {
			return nil, fmt.Errorf("reportrender: xlsx cell %d: %w", i+1, err)
		}
		if err := f.SetSheetRow("Sheet1", cell, &record); err != nil {
			return nil, fmt.Errorf("reportrender: xlsx row %d: %w", i+1, err)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("reportrender: xlsx write: %w", err)
	}
	return buf.Bytes(), nil
}

// bars groups rows by dashboard group in first-seen order (Laravel's
// collect()->groupBy()) and pre-computes each static SVG bar's geometry.
func bars(rows []Row) []groupBar {
	order := make([]string, 0, len(rows))
	sums := make(map[string]float64, len(rows))
	counts := make(map[string]int, len(rows))
	for _, r := range rows {
		if _, ok := sums[r.DashboardGroup]; !ok {
			order = append(order, r.DashboardGroup)
		}
		sums[r.DashboardGroup] += r.UptimePercentage
		counts[r.DashboardGroup]++
	}
	barW := 600/max(1, len(order)) - 20
	out := make([]groupBar, 0, len(order))
	for i, group := range order {
		avg := math.Round(sums[group]/float64(counts[group])*100) / 100
		h := int(avg)
		x := 20 + i*(barW+20)
		out = append(out, groupBar{
			Group:   group,
			AvgText: strconv.FormatFloat(avg, 'f', 2, 64),
			X:       x,
			W:       barW,
			H:       h,
			CX:      x + barW/2,
			ValueY:  114 - h,
		})
	}
	return out
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// toFloat normalises template numeric args (int, int64, float64).
func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case float32:
		return float64(n)
	case float64:
		return n
	default:
		return 0
	}
}
