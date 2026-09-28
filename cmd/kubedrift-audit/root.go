package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := newRoot().Execute(); err != nil {
		os.Exit(exitCode(err))
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if ce, ok := err.(codedError); ok {
		return ce.code
	}
	return 1
}

type codedError struct {
	code int
	msg  string
}

func (e codedError) Error() string { return e.msg }

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:     "kubedrift-audit",
		Aliases: []string{"kubectl-bcm-audit"},
		Short:   "Local Kubernetes BCM/DORA backup coverage audit vs Velero",
		Long:    "Audit PersistentVolumeClaim backup coverage and restore evidence against Velero schedules without sending data off-cluster.",
		SilenceUsage: true,
	}
	scan := newScanCmd()
	root.AddCommand(scan)
	root.AddCommand(newExplainCmd())
	root.AddCommand(newVersionCmd())
	// Root alias: same as `scan`
	root.RunE = scan.RunE
	root.Flags().AddFlagSet(scan.Flags())
	return root
}
