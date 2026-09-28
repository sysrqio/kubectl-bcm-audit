package cluster_test

import (
	"testing"

	"github.com/sysrqio/kubectl-bcm-audit/internal/cluster"
)

func TestFilterAllows(t *testing.T) {
	f := cluster.FilterOptions{
		Namespace:         "app",
		ExcludeNamespaces: []string{"kube-system"},
	}
	if !f.Allows("app") {
		t.Fatal("should allow app")
	}
	if f.Allows("kube-system") {
		t.Fatal("should exclude kube-system")
	}
	if f.Allows("other") {
		t.Fatal("should restrict to namespace")
	}
}

func TestParseExcludeList(t *testing.T) {
	got := cluster.ParseExcludeList(" a , b ")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected: %v", got)
	}
}
