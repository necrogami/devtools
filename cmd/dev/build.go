package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// defaultImage is the canonical registry reference for the devtools image.
const defaultImage = "ghcr.io/necrogami/devtools"

func newBuildCmd() *cobra.Command {
	var (
		platforms string
		push      bool
		tag       string
	)
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build the base devtools image locally via buildx",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolvePaths()
			if err != nil {
				return err
			}
			if tag == "" {
				tag = time.Now().UTC().Format("2006-01-02")
			}
			ref := fmt.Sprintf("%s:%s", defaultImage, tag)
			latest := fmt.Sprintf("%s:latest", defaultImage)

			args := []string{
				"buildx", "build",
				"--platform", platforms,
				"--file", "base/Dockerfile",
				"--tag", ref,
				"--tag", latest,
			}
			if push {
				args = append(args, "--push")
			} else {
				args = append(args, "--load")
			}
			args = append(args, ".")

			cmd.Printf(">>> %s\n", ref)
			return runDockerIn(r.Root, args...)
		},
	}
	cmd.Flags().StringVar(&platforms, "platform", "linux/amd64,linux/arm64",
		"comma-separated list of target platforms")
	cmd.Flags().BoolVar(&push, "push", false,
		"push to registry instead of loading into local docker")
	cmd.Flags().StringVar(&tag, "tag", "",
		"image tag (default: today's YYYY-MM-DD)")
	return cmd
}

func newPushCmd() *cobra.Command {
	var tag string
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Build + push the base devtools image to the configured registry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			buildCmd := newBuildCmd()
			if err := buildCmd.Flags().Set("push", "true"); err != nil {
				return err
			}
			if tag != "" {
				if err := buildCmd.Flags().Set("tag", tag); err != nil {
					return err
				}
			}
			return buildCmd.RunE(buildCmd, nil)
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "",
		"image tag (default: today's YYYY-MM-DD)")
	return cmd
}
