package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/necrogami/devtools/internal/paths"
	"github.com/necrogami/devtools/internal/tmpl"
)

// projectGitignore is written into every scaffolded project. It prevents
// the runtime-generated compose override and the .env (which contains the
// project's pinned image tag and any service secrets) from being committed.
const projectGitignore = `# devtools: written by 'dev new'
# Host-specific bind mounts regenerated every 'dev up'.
docker-compose.override.yml
# Per-project environment; may contain secrets.
.env
`

func newNewCmd() *cobra.Command {
	var devtoolsTag string
	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Scaffold a new project stack under projects/<name>",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := paths.ValidateProjectName(name); err != nil {
				return err
			}

			r, err := resolvePaths()
			if err != nil {
				return err
			}

			dstDir, err := r.ProjectDir(name)
			if err != nil {
				return err
			}

			err = tmpl.Render(r.Template, dstDir, tmpl.Vars{
				Project:     name,
				DevtoolsTag: devtoolsTag,
			})
			if err != nil {
				return fmt.Errorf("scaffold: %w", err)
			}

			// Write .gitignore so the runtime-generated override and the
			// project's .env never land in commits.
			gi := filepath.Join(dstDir, ".gitignore")
			if err := os.WriteFile(gi, []byte(projectGitignore), 0o644); err != nil {
				return fmt.Errorf("write .gitignore: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(),
				"created %s\n\n  next: dev up %s\n", dstDir, name)
			return nil
		},
	}
	cmd.Flags().StringVar(&devtoolsTag, "tag", "latest",
		"initial DEVTOOLS_TAG for the project's .env")
	return cmd
}
