package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/necrogami/devtools/internal/compose"
)

func newBumpCmd() *cobra.Command {
	var tag string
	cmd := &cobra.Command{
		Use:   "bump <project>",
		Short: "Update DEVTOOLS_TAG in a project's .env",
		Long: "Rewrites DEVTOOLS_TAG in projects/<name>/.env. " +
			"Default tag is today's YYYY-MM-DD. Run `dev up <name>` after.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, dir, err := resolveProject(args[0])
			if err != nil {
				return err
			}
			resolved := tag
			if resolved == "" {
				resolved = time.Now().UTC().Format("2006-01-02")
			}
			prev, err := compose.BumpTag(dir, resolved)
			if err != nil {
				return err
			}
			if prev == resolved {
				fmt.Fprintf(cmd.OutOrStdout(),
					"%s already pinned to %s\n", args[0], resolved)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"%s: %s → %s\n  next: dev up --pull %s\n",
				args[0], displayTag(prev), resolved, args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "",
		"image tag to pin (default: today's UTC date)")
	return cmd
}

func displayTag(s string) string {
	if s == "" {
		return "(unset)"
	}
	return s
}
