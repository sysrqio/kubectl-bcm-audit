package output_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sysrqio/kubectl-bcm-audit/internal/audit"
	"github.com/sysrqio/kubectl-bcm-audit/internal/output"
)

func sampleReport() *audit.Report {
	return &audit.Report{
		Standard:                "dora",
		StandardDescription:     "DORA operational resilience (backup coverage heuristic)",
		GeneratedAt:             time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC),
		OverallScorePercentage:  86.7,
		TotalPVCs:               3,
		CoveredPVCs:             2,
		DriftCount:              1,
		RestoreEvidenceGapCount: 1,
		HasDrift:                true,
		Findings: []audit.Finding{
			{
				Code:      "PVC_NO_BACKUP_SCHEDULE",
				Severity:  audit.SeverityCritical,
				Namespace: "app-legacy",
				Message:   "PVC app-legacy/orphan-volume-claim has no matching Velero backup schedule",
			},
		},
	}
}

func TestWriteJSONReportFields(t *testing.T) {
	var buf bytes.Buffer
	if err := output.Write(&buf, output.JSON, sampleReport()); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	for _, key := range []string{
		"standard", "overall_score_percentage", "total_pvcs", "covered_pvcs",
		"drift_count", "restore_evidence_gap_count", "has_drift", "findings",
	} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("missing json field %q", key)
		}
	}
	if decoded["standard"] != "dora" {
		t.Fatalf("standard: %v", decoded["standard"])
	}
	findings, ok := decoded["findings"].([]any)
	if !ok || len(findings) != 1 {
		t.Fatalf("findings: %v", decoded["findings"])
	}
}

func TestWriteMarkdownReportFields(t *testing.T) {
	var buf bytes.Buffer
	if err := output.Write(&buf, output.Markdown, sampleReport()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"# BCM/DORA Backup Coverage Audit",
		"**Standard:** dora",
		"**Score:** 86.7%",
		"**PVCs:** 2 covered / 3 total",
		"**Drift:** 1",
		"**Restore evidence gaps:** 1",
		"## Findings",
		"PVC_NO_BACKUP_SCHEDULE",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("markdown missing %q:\n%s", want, out)
		}
	}
}

func TestWriteHTMLReportFields(t *testing.T) {
	var buf bytes.Buffer
	if err := output.Write(&buf, output.HTML, sampleReport()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"<!DOCTYPE html>",
		"BCM/DORA Backup Coverage Audit",
		"Standard: dora",
		"86.7%",
		"PVC_NO_BACKUP_SCHEDULE",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("html missing %q:\n%s", want, out)
		}
	}
}

func TestWritePVCDetailJSON(t *testing.T) {
	d := &audit.PVCDetail{
		Namespace:       "app-prod",
		Name:            "postgres-data-postgres-0",
		HasSchedule:     true,
		RestoreEvidence: true,
		MatchedSchedules: []string{"daily-app-prod"},
	}
	var buf bytes.Buffer
	if err := output.WritePVCDetail(&buf, output.JSON, d); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["namespace"] != "app-prod" || decoded["has_schedule"] != true {
		t.Fatalf("unexpected detail: %v", decoded)
	}
}
