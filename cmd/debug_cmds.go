// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/util/archive"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/spf13/cobra"
)

// newArchiveCommand creates a command which just builds the archive we would ship to Pulumi.com to
// do a deployment.
func newArchiveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive <path-to-archive>",
		Short: "create an archive suitable for deployment",
		Args:  cobra.ExactArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return errors.Wrap(err, "getting working directory")
			}
			programPath, err := workspace.DetectPackage(cwd)
			if err != nil {
				return errors.Wrap(err, "looking for Pulumi package")
			}
			if programPath == "" {
				return errors.New("no Pulumi package found")
			}

			// programPath is the path to the Pulumi.yaml file. Need its parent folder.
			programFolder := filepath.Dir(programPath)
			archiveContents, err := archive.Process(programFolder)
			if err != nil {
				return err
			}

			return ioutil.WriteFile(args[0], archiveContents.Bytes(), 0644)
		}),
	}

	return cmd
}
