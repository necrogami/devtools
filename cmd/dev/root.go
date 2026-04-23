package main

import (
	"context"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

// Populated at build time via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// repoFlag holds the --repo override (empty = auto-discover from CWD).
var repoFlag string

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "dev",
		Short:         "Manage devtools images and per-project container stacks",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&repoFlag, "repo", "",
		"path inside the devtools repo (default: current directory)")

	root.AddCommand(
		newNewCmd(),
		newUpCmd(),
		newDownCmd(),
		newShellCmd(),
		newExecCmd(),
		newLogsCmd(),
		newPsCmd(),
		newListCmd(),
		newBumpCmd(),
		newInitSharedCmd(),
		newBuildCmd(),
		newPushCmd(),
		newDoctorCmd(),
		newVersionCmd(),
	)

	return root
}

func execute(ctx context.Context) error {
	return fang.Execute(ctx, newRootCmd())
}
