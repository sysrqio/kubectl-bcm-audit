package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/sysrqio/kubectl-bcm-audit/internal/audit"
	"github.com/sysrqio/kubectl-bcm-audit/internal/cluster"
	"github.com/sysrqio/kubectl-bcm-audit/internal/output"
	"github.com/sysrqio/kubectl-bcm-audit/internal/standards"
)

type scanFlags struct {
	namespace         string
	excludeNamespaces string
	veleroNamespace   string
	standard          string
	outputFormat      string
	failOnDrift       bool
	kubeconfig        string
	fixture           string
}

func newScanCmd() *cobra.Command {
	f := &scanFlags{
		excludeNamespaces: "kube-system,kube-public,kube-node-lease",
		veleroNamespace:   "velero",
		standard:          "dora",
		outputFormat:      "terminal",
	}
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan cluster or fixture for backup coverage drift",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScan(f)
		},
	}
	cmd.Flags().StringVar(&f.namespace, "namespace", "", "Limit audit to a single namespace")
	cmd.Flags().StringVar(&f.excludeNamespaces, "exclude-namespaces", f.excludeNamespaces, "Comma-separated namespaces to skip")
	cmd.Flags().StringVar(&f.veleroNamespace, "velero-namespace", f.veleroNamespace, "Namespace where Velero CRDs live")
	cmd.Flags().StringVar(&f.standard, "standard", f.standard, "Compliance heuristic: dora|bsi")
	cmd.Flags().StringVar(&f.outputFormat, "output", f.outputFormat, "Output format: terminal|json|markdown|html")
	cmd.Flags().BoolVar(&f.failOnDrift, "fail-on-drift", false, "Exit 2 when drift findings exist")
	cmd.Flags().StringVar(&f.kubeconfig, "kubeconfig", "", "Path to kubeconfig (optional live cluster)")
	cmd.Flags().StringVar(&f.fixture, "fixture", "", "Path to JSON cluster snapshot for offline audit")
	return cmd
}

func runScan(f *scanFlags) error {
	var snap *cluster.Snapshot
	var err error
	ctx := context.Background()

	if f.fixture != "" {
		snap, err = cluster.LoadSnapshotFromFile(f.fixture)
		if err != nil {
			return codedError{code: 1, msg: err.Error()}
		}
	} else if f.kubeconfig != "" || envKubeconfig() != "" {
		snap, err = cluster.LoadFromCluster(ctx, f.kubeconfig, f.veleroNamespace)
		if err != nil {
			return codedError{code: 1, msg: err.Error()}
		}
	} else {
		return codedError{code: 1, msg: "either --fixture or --kubeconfig (or KUBECONFIG) is required for scan"}
	}

	opt := audit.Options{
		Filter: cluster.FilterOptions{
			Namespace:         f.namespace,
			ExcludeNamespaces: cluster.ParseExcludeList(f.excludeNamespaces),
		},
		VeleroNS: f.veleroNamespace,
		Standard: standards.Parse(f.standard),
		Now:      time.Now().UTC(),
	}
	rep := audit.Run(snap, opt)
	if err := output.Write(os.Stdout, output.ParseFormat(f.outputFormat), rep); err != nil {
		return err
	}
	if f.failOnDrift && rep.HasDrift {
		return codedError{code: 2, msg: fmt.Sprintf("drift detected: %d uncovered PVC(s)", rep.DriftCount)}
	}
	return nil
}

func envKubeconfig() string {
	return os.Getenv("KUBECONFIG")
}
