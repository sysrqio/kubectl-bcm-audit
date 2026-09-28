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
