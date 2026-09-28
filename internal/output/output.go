package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/sysrqio/kubectl-bcm-audit/internal/audit"
)

type Format string

const (
	Terminal Format = "terminal"
	JSON     Format = "json"
	Markdown Format = "markdown"
	HTML     Format = "html"
)

func ParseFormat(s string) Format {
	switch strings.ToLower(s) {
	case "json":
		return JSON
	case "markdown", "md":
		return Markdown
	case "html":
		return HTML
	default:
		return Terminal
	}
}

func Write(w io.Writer, format Format, rep *audit.Report) error {
	switch format {
	case JSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	case Markdown:
		_, err := fmt.Fprintf(w, "# BCM/DORA Backup Coverage Audit\n\n")
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "- **Standard:** %s\n", rep.Standard)
		fmt.Fprintf(w, "- **Score:** %.1f%%\n", rep.OverallScorePercentage)
		fmt.Fprintf(w, "- **PVCs:** %d covered / %d total\n", rep.CoveredPVCs, rep.TotalPVCs)
		fmt.Fprintf(w, "- **Drift:** %d\n", rep.DriftCount)
		fmt.Fprintf(w, "- **Restore evidence gaps:** %d\n\n", rep.RestoreEvidenceGapCount)
		fmt.Fprintf(w, "## Findings\n\n")
		for _, f := range rep.Findings {
			fmt.Fprintf(w, "- [%s] **%s** %s/%s — %s\n", f.Severity, f.Code, f.Namespace, f.Resource, f.Message)
		}
		return nil
	case HTML:
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta charset="utf-8"><title>BCM Audit</title></head><body>`)
		fmt.Fprintf(w, `<h1>BCM/DORA Backup Coverage Audit</h1>`)
		fmt.Fprintf(w, `<p>Standard: %s | Score: <strong>%.1f%%</strong></p>`, rep.Standard, rep.OverallScorePercentage)
		fmt.Fprintf(w, `<ul>`)
		for _, f := range rep.Findings {
			fmt.Fprintf(w, `<li>[%s] %s: %s</li>`, f.Severity, f.Code, f.Message)
		}
		fmt.Fprintf(w, `</ul></body></html>`)
		return nil
	default:
		fmt.Fprintf(w, "BCM/DORA backup coverage audit (%s)\n", rep.Standard)
		fmt.Fprintf(w, "Overall score: %.1f%% (%d/%d PVCs with schedule coverage)\n", rep.OverallScorePercentage, rep.CoveredPVCs, rep.TotalPVCs)
		fmt.Fprintf(w, "Drift: %d | Restore evidence gaps: %d\n\n", rep.DriftCount, rep.RestoreEvidenceGapCount)
		if len(rep.Findings) == 0 {
			fmt.Fprintf(w, "No findings.\n")
			return nil
		}
		fmt.Fprintf(w, "Findings:\n")
		for _, f := range rep.Findings {
			fmt.Fprintf(w, "  [%s] %s — %s\n", f.Severity, f.Code, f.Message)
		}
		return nil
	}
}

func WritePVCDetail(w io.Writer, format Format, d *audit.PVCDetail) error {
	switch format {
	case JSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	default:
		fmt.Fprintf(w, "PVC %s/%s\n", d.Namespace, d.Name)
		fmt.Fprintf(w, "  Schedule coverage: %v\n", d.HasSchedule)
		if len(d.MatchedSchedules) > 0 {
			fmt.Fprintf(w, "  Schedules: %s\n", strings.Join(d.MatchedSchedules, ", "))
		}
		if d.RecentBackup != "" {
			fmt.Fprintf(w, "  Recent backup: %s\n", d.RecentBackup)
		}
		fmt.Fprintf(w, "  Restore evidence: %v\n", d.RestoreEvidence)
		for _, f := range d.Findings {
			fmt.Fprintf(w, "  [%s] %s — %s\n", f.Severity, f.Code, f.Message)
		}
		return nil
	}
}
