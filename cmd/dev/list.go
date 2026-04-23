package main

import (
	"fmt"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/necrogami/devtools/internal/compose"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all projects under projects/ with their pinned image tag",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolvePaths()
			if err != nil {
				return err
			}
			names, err := r.ListProjects()
			if err != nil {
				return err
			}
			if len(names) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no projects yet — run `dev new <name>`")
				return nil
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "PROJECT\tTAG\tSTATUS")
			for _, n := range names {
				dir, _ := r.ProjectDir(n)
				tag := "—"
				if env, err := compose.LoadEnv(compose.EnvFile(dir)); err == nil {
					if v, ok := env.Get(compose.DevtoolsTagKey); ok && v != "" {
						tag = v
					}
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", n, tag, dockerStatus(n))
			}
			return tw.Flush()
		},
	}
}

// dockerStatus returns a short running-state summary for a project's
// primary tools container. "stopped" when the container is absent.
func dockerStatus(project string) string {
	out, err := exec.Command("docker", "inspect",
		"--format", "{{.State.Status}}",
		project+"-tools").CombinedOutput()
	if err != nil {
		return "stopped"
	}
	return strings.TrimSpace(string(out))
}
