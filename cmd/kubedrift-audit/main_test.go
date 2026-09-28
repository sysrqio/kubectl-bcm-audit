package main

import (
	"bytes"
	"os"
	"path/filepath"
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
	root.SetArgs([]string{
		"scan",
		"--fixture", fixture,
		"--output", "json",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
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
	root.SetOut(os.Stdout)
	root.SetArgs([]string{
		"explain", "postgres-data-postgres-0",
		"--namespace", "app-prod",
		"--fixture", fixture,
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
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
