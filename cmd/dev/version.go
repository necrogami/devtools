package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print dev CLI build info",
		Run: func(cmd *cobra.Command, _ []string) {
			// Tagged release build via GoReleaser sets all three via
			// -ldflags -X. A source-tree `go build` leaves them at
			// their defaults, which used to render as the confusing
			// "dev dev / commit: none / built: unknown". Fall back to
			// a single clear "development build" line in that case.
			if version == "dev" && commit == "none" {
				fmt.Fprintln(cmd.OutOrStdout(), "dev (development build — unreleased source tree)")
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"dev %s\ncommit: %s\nbuilt:  %s\n", version, commit, date)
		},
	}
}
