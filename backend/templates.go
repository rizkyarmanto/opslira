package opslira

import "embed"

// ReportTemplates holds the HTML report templates (daily/weekly/monthly).
//
//go:embed templates/reports/*.html
var ReportTemplates embed.FS
