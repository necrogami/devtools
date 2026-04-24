package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/necrogami/devtools/internal/compose"
	"github.com/necrogami/devtools/internal/hostenv"
)

func newUpCmd() *cobra.Command {
	var detach bool
	cmd := &cobra.Command{
		Use:   "up <project>",
		Short: "Start a project's compose stack",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, dir, err := resolveProject(args[0])
			if err != nil {
				return err
			}
			if _, err := ensureHostPaths(cmd.ErrOrStderr()); err != nil {
				return fmt.Errorf("host pre-flight: %w", err)
			}
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve home: %w", err)
			}
			creds := hostenv.Discover(home)
			if err := compose.WriteOverride(dir, creds); err != nil {
				return fmt.Errorf("generate compose override: %w", err)
			}
			composeArgs := []string{"up"}
			if detach {
				composeArgs = append(composeArgs, "-d")
			}
			return runCompose(dir, composeArgs...)
		},
	}
	cmd.Flags().BoolVarP(&detach, "detach", "d", true,
		"run in the background (default)")
	return cmd
}

func newDownCmd() *cobra.Command {
	var withVolumes bool
	cmd := &cobra.Command{
		Use:   "down <project>",
		Short: "Stop a project's compose stack",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, dir, err := resolveProject(args[0])
			if err != nil {
				return err
			}
			downArgs := []string{"down"}
			if withVolumes {
				downArgs = append(downArgs, "--volumes")
			}
			return runCompose(dir, downArgs...)
		},
	}
	cmd.Flags().BoolVar(&withVolumes, "volumes", false,
		"also delete per-project volumes (code, home, service data)")
	return cmd
}

func newShellCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell <project>",
		Short: "Open a login shell inside the project's tools container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			_, _, err := resolveProject(name)
			if err != nil {
				return err
			}
			container := name + "-tools"
			// Replace current process so Ctrl-D / exit returns directly.
			docker, err := exec.LookPath("docker")
			if err != nil {
				return fmt.Errorf("docker not in PATH: %w", err)
			}
			execArgs := []string{"docker", "exec", "-it", container, "bash", "-l"}
			return syscallExec(docker, execArgs, os.Environ())
		},
	}
}

func newExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec <project> -- <command> [args...]",
		Short: "Run a one-shot command inside a project's tools container",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			rest := args[1:]
			_, _, err := resolveProject(name)
			if err != nil {
				return err
			}
			container := name + "-tools"
			full := append([]string{"exec", "-it", container}, rest...)
			return runDocker(full...)
		},
	}
	cmd.Flags().SetInterspersed(false)
	return cmd
}

func newLogsCmd() *cobra.Command {
	var follow bool
	cmd := &cobra.Command{
		Use:   "logs <project> [service]",
		Short: "Tail compose logs for a project (optionally one service)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, dir, err := resolveProject(args[0])
			if err != nil {
				return err
			}
			logsArgs := []string{"logs", "--tail=200"}
			if follow {
				logsArgs = append(logsArgs, "--follow")
			}
			if len(args) == 2 {
				logsArgs = append(logsArgs, args[1])
			}
			return runCompose(dir, logsArgs...)
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "stream logs")
	return cmd
}

func newPsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ps [project]",
		Short: "Show running containers (all projects, or one)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				_, dir, err := resolveProject(args[0])
				if err != nil {
					return err
				}
				return runCompose(dir, "ps")
			}
			return runDocker("ps",
				"--filter", "label=com.docker.compose.project",
				"--format", "table {{.Names}}\t{{.Status}}\t{{.Ports}}")
		},
	}
}
