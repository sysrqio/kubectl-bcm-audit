package main

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/sysrqio/kubectl-bcm-audit/internal/audit"
	"github.com/sysrqio/kubectl-bcm-audit/internal/cluster"
	"github.com/sysrqio/kubectl-bcm-audit/internal/output"
	"github.com/sysrqio/kubectl-bcm-audit/internal/standards"
)

func newExplainCmd() *cobra.Command {
	var (
		namespace         string
		excludeNamespaces string
		veleroNamespace   string
		standard          string
		outputFormat      string
		kubeconfig        string
		fixture           string
	)
	cmd := &cobra.Command{
		Use:   "explain <pvc-name>",
		Short: "Explain backup coverage for a single PVC",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var snap *cluster.Snapshot
			var err error
			ctx := context.Background()
			if fixture != "" {
				snap, err = cluster.LoadSnapshotFromFile(fixture)
			} else if kubeconfig != "" || envKubeconfig() != "" {
				snap, err = cluster.LoadFromCluster(ctx, kubeconfig, veleroNamespace)
			} else {
				return codedError{code: 1, msg: "either --fixture or --kubeconfig is required"}
			}
			if err != nil {
				return codedError{code: 1, msg: err.Error()}
			}
			opt := audit.Options{
				Filter: cluster.FilterOptions{
					Namespace:         namespace,
					ExcludeNamespaces: cluster.ParseExcludeList(excludeNamespaces),
				},
				VeleroNS: veleroNamespace,
				Standard: standards.Parse(standard),
				Now:      time.Now().UTC(),
			}
			detail, err := audit.Explain(snap, opt, args[0])
			if err != nil {
				return codedError{code: 1, msg: err.Error()}
			}
			return output.WritePVCDetail(os.Stdout, output.ParseFormat(outputFormat), detail)
		},
	}
	cmd.Flags().StringVar(&namespace, "namespace", "", "Namespace for PVC name lookup")
	cmd.Flags().StringVar(&excludeNamespaces, "exclude-namespaces", "kube-system,kube-public,kube-node-lease", "Excluded namespaces")
	cmd.Flags().StringVar(&veleroNamespace, "velero-namespace", "velero", "Velero namespace")
	cmd.Flags().StringVar(&standard, "standard", "dora", "dora|bsi")
	cmd.Flags().StringVar(&outputFormat, "output", "terminal", "terminal|json")
	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "kubeconfig path")
	cmd.Flags().StringVar(&fixture, "fixture", "", "offline fixture JSON")
	return cmd
}
