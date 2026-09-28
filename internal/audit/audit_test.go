package audit_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sysrqio/kubectl-bcm-audit/internal/audit"
	"github.com/sysrqio/kubectl-bcm-audit/internal/cluster"
	"github.com/sysrqio/kubectl-bcm-audit/internal/standards"
)

func TestRunFixtureDetectsDriftAndRestoreGap(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "cluster.json")
	snap, err := cluster.LoadSnapshotFromFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	opt := audit.Options{
		Filter: cluster.FilterOptions{
			ExcludeNamespaces: cluster.ParseExcludeList("kube-system,kube-public,kube-node-lease"),
		},
		Standard: standards.DORA,
		Now:      now,
	}
	rep := audit.Run(snap, opt)
	if rep.TotalPVCs != 3 {
		t.Fatalf("expected 3 PVCs, got %d", rep.TotalPVCs)
	}
	if rep.DriftCount != 1 {
		t.Fatalf("expected 1 drift (orphan PVC), got %d", rep.DriftCount)
	}
	if rep.CoveredPVCs != 2 {
		t.Fatalf("expected 2 covered PVCs, got %d", rep.CoveredPVCs)
	}
	if rep.RestoreEvidenceGapCount < 1 {
		t.Fatalf("expected restore evidence gap for staging backup")
	}
	if rep.OverallScorePercentage <= 0 || rep.OverallScorePercentage > 100 {
		t.Fatalf("invalid score: %f", rep.OverallScorePercentage)
	}
}

func TestRunCleanFixtureHighScoreNoDrift(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "cluster-clean.json")
	snap, err := cluster.LoadSnapshotFromFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	rep := audit.Run(snap, audit.Options{
		Filter: cluster.FilterOptions{
			ExcludeNamespaces: cluster.ParseExcludeList("kube-system,kube-public,kube-node-lease"),
		},
		Standard: standards.DORA,
		Now:      now,
	})
	if rep.DriftCount != 0 || rep.HasDrift {
		t.Fatalf("expected no drift, got drift_count=%d", rep.DriftCount)
	}
	if rep.OverallScorePercentage < 99 {
		t.Fatalf("expected high score, got %f", rep.OverallScorePercentage)
	}
	if rep.RestoreEvidenceGapCount != 0 {
		t.Fatalf("expected no restore gaps, got %d", rep.RestoreEvidenceGapCount)
	}
}

func TestStandardBSIVsDORARestoreEvidenceWindow(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "cluster-restore-window.json")
	snap, err := cluster.LoadSnapshotFromFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	base := audit.Options{
		Filter: cluster.FilterOptions{
			ExcludeNamespaces: cluster.ParseExcludeList("kube-system,kube-public,kube-node-lease"),
		},
		Now: now,
	}
	dora := base
	dora.Standard = standards.DORA
	bsi := base
	bsi.Standard = standards.BSI

	doraRep := audit.Run(snap, dora)
	bsiRep := audit.Run(snap, bsi)

	if doraRep.Standard != "dora" || bsiRep.Standard != "bsi" {
		t.Fatalf("standards: dora=%q bsi=%q", doraRep.Standard, bsiRep.Standard)
	}
	if doraRep.RestoreEvidenceGapCount < 1 {
		t.Fatal("DORA (30d restore window) should flag restore evidence gap")
	}
	if bsiRep.RestoreEvidenceGapCount != 0 {
		t.Fatalf("BSI (90d restore window) should accept restore evidence, gaps=%d", bsiRep.RestoreEvidenceGapCount)
	}
	if countCode(doraRep.Findings, "RESTORE_EVIDENCE_GAP") < 1 {
		t.Fatal("expected RESTORE_EVIDENCE_GAP under DORA")
	}
	if countCode(bsiRep.Findings, "RESTORE_EVIDENCE_GAP") != 0 {
		t.Fatal("expected no RESTORE_EVIDENCE_GAP under BSI")
	}
}

func countCode(findings []audit.Finding, code string) int {
	n := 0
	for _, f := range findings {
		if f.Code == code {
			n++
		}
	}
	return n
}

func TestExplainPVC(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "cluster.json")
	snap, err := cluster.LoadSnapshotFromFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	opt := audit.Options{
		Filter: cluster.FilterOptions{
			ExcludeNamespaces: cluster.ParseExcludeList("kube-system"),
		},
		Standard: standards.DORA,
		Now:      time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC),
	}
	d, err := audit.Explain(snap, opt, "app-legacy/orphan-volume-claim")
	if err != nil {
		t.Fatal(err)
	}
	if d.HasSchedule {
		t.Fatal("orphan PVC should not have schedule")
	}
	if len(d.Findings) == 0 {
		t.Fatal("expected findings")
	}
}
