package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIVersion(t *testing.T) {
	root := newRoot()
	root.SetArgs([]string{"version"})
	var buf bytes.Buffer
	root.SetOut(&buf)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected version output")
	}
}

func TestCLIScanFixtureJSON(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "cluster.json")
	root := newRoot()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{
		"scan",
		"--fixture", fixture,
		"--output", "json",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var rep map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rep); err != nil {
		t.Fatalf("stdout json: %v\n%s", err, buf.String())
	}
	if rep["has_drift"] != true {
		t.Fatalf("expected has_drift true, got %v", rep["has_drift"])
	}
	for _, key := range []string{"standard", "overall_score_percentage", "findings"} {
		if _, ok := rep[key]; !ok {
			t.Fatalf("missing %q in json output", key)
		}
	}
}

func TestCLIScanCleanFixtureExitZero(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "cluster-clean.json")
	root := newRoot()
	root.SetArgs([]string{
		"scan",
		"--fixture", fixture,
		"--output", "json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("clean fixture should exit 0: %v", err)
	}
}

func TestCLIScanCleanFixtureFailOnDriftPass(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "cluster-clean.json")
	root := newRoot()
	root.SetArgs([]string{
		"scan",
		"--fixture", fixture,
		"--fail-on-drift",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("clean fixture with fail-on-drift should exit 0: %v", err)
	}
}

func TestCLIScanStandardBSIVsDORA(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "cluster-restore-window.json")
	runJSON := func(std string) map[string]any {
		root := newRoot()
		var buf bytes.Buffer
		root.SetOut(&buf)
		root.SetArgs([]string{"scan", "--fixture", fixture, "--standard", std, "--output", "json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("standard %s: %v", std, err)
		}
		var rep map[string]any
		if err := json.Unmarshal(buf.Bytes(), &rep); err != nil {
			t.Fatalf("json: %v", err)
		}
		return rep
	}
	dora := runJSON("dora")
	bsi := runJSON("bsi")
	if dora["standard"] != "dora" || bsi["standard"] != "bsi" {
		t.Fatalf("standards dora=%v bsi=%v", dora["standard"], bsi["standard"])
	}
	doraGaps, _ := dora["restore_evidence_gap_count"].(float64)
	bsiGaps, _ := bsi["restore_evidence_gap_count"].(float64)
	if doraGaps < 1 {
		t.Fatal("dora should report restore evidence gap")
	}
	if bsiGaps != 0 {
		t.Fatalf("bsi should accept restore window, gaps=%v", bsiGaps)
	}
}

func TestCLIScanOutputMarkdownAndHTML(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "cluster.json")
	for _, format := range []string{"markdown", "html"} {
		root := newRoot()
		var buf bytes.Buffer
		root.SetOut(&buf)
		root.SetArgs([]string{"scan", "--fixture", fixture, "--output", format})
		if err := root.Execute(); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		out := buf.String()
		if !strings.Contains(strings.ToLower(out), "dora") {
			t.Fatalf("%s missing standard", format)
		}
		if !strings.Contains(out, "PVC_NO_BACKUP_SCHEDULE") {
			t.Fatalf("%s missing finding code", format)
		}
	}
}

func TestCLIScanFailOnDrift(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "cluster.json")
	root := newRoot()
	root.SetArgs([]string{
		"scan",
		"--fixture", fixture,
		"--fail-on-drift",
	})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected drift exit")
	}
	if exitCode(err) != 2 {
		t.Fatalf("expected exit 2, got %d", exitCode(err))
	}
}

func TestCLIScanMissingSource(t *testing.T) {
	root := newRoot()
	root.SetArgs([]string{"scan"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if exitCode(err) != 1 {
		t.Fatalf("expected exit 1, got %d", exitCode(err))
	}
}

func TestCLIExplainFixture(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "cluster.json")
	root := newRoot()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{
		"explain", "postgres-data-postgres-0",
		"--namespace", "app-prod",
		"--fixture", fixture,
		"--output", "json",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var detail map[string]any
	if err := json.Unmarshal(buf.Bytes(), &detail); err != nil {
		t.Fatalf("explain json: %v\n%s", err, buf.String())
	}
	if detail["namespace"] != "app-prod" || detail["name"] != "postgres-data-postgres-0" {
		t.Fatalf("unexpected pvc: %v", detail)
	}
	if detail["has_schedule"] != true {
		t.Fatalf("expected schedule coverage: %v", detail)
	}
}

func TestCLIExplainFixtureTerminalSmoke(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "cluster.json")
	root := newRoot()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{
		"explain", "postgres-data-postgres-0",
		"--namespace", "app-prod",
		"--fixture", fixture,
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "app-prod/postgres-data-postgres-0") {
		t.Fatalf("terminal explain smoke: %q", out)
	}
}

func TestRootAliasScan(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "cluster.json")
	root := newRoot()
	root.SetArgs([]string{"--fixture", fixture})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}
