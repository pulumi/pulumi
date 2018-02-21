// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"io/ioutil"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/util/archive"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// newArchiveCommand creates a command which just builds the archive we would ship to Pulumi.com to
// do a deployment.
func newArchiveCommand() *cobra.Command {
	var forceNoDefaultIgnores bool
	var forceDefaultIgnores bool

	cmd := &cobra.Command{
		Use:   "archive <path-to-archive>",
		Short: "create an archive suitable for deployment",
		Args:  cmdutil.SpecificArgs([]string{"path-to-archive"}),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			if forceDefaultIgnores && forceNoDefaultIgnores {
				return errors.New("can't specify --no-default-ignores and --default-ignores at the same time")
			}

			proj, path, err := workspace.DetectProjectAndPath()
			if err != nil {
				return err
			}

			useDeafultIgnores := proj.UseDefaultIgnores()

			if forceDefaultIgnores {
				useDeafultIgnores = true
			} else if forceNoDefaultIgnores {
				useDeafultIgnores = false
			}

			// path is the path to the Pulumi.yaml file.  Need its parent directory.
			dir := filepath.Dir(path)
			archiveContents, err := archive.Process(dir, useDeafultIgnores)
			if err != nil {
				return errors.Wrap(err, "creating archive")
			}

			return ioutil.WriteFile(args[0], archiveContents.Bytes(), 0644)
		}),
	}
	cmd.PersistentFlags().BoolVar(
		&forceNoDefaultIgnores, "--no-default-ignores", false,
		"Do not use default ignores, regardless of Pulumi.yaml")
	cmd.PersistentFlags().BoolVar(
		&forceDefaultIgnores, "--default-ignores", false,
		"Use default ignores, regardless of Pulumi.yaml")

	return cmd
}
