// Copyright 2017 Pulumi, Inc. All rights reserved.

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/lumi/pkg/encoding"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/cmdutil"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/workspace"
)

func newEnvLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List all known environments",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Read the environment directory.
			path := workspace.EnvPath("")
			files, err := ioutil.ReadDir(path)
			if err != nil && !os.IsNotExist(err) {
				return errors.Errorf("could not read environments: %v", err)
			}

			// Create a new context to share amongst all of the loads.
			ctx := resource.NewContext(cmdutil.Sink())
			defer ctx.Close()

			fmt.Printf("%-20s %-48s %-12s\n", "NAME", "LAST DEPLOYMENT", "RESOURCE COUNT")
			curr := getCurrentEnv()
			for _, file := range files {
				// Ignore directories.
				if file.IsDir() {
					continue
				}

				// Skip files without valid extensions (e.g., *.bak files).
				envfn := file.Name()
				ext := filepath.Ext(envfn)
				if _, has := encoding.Marshalers[ext]; !has {
					continue
				}

				// Read in this environment's information.
				name := tokens.QName(envfn[:len(envfn)-len(ext)])
				envfile, env, old := readEnv(ctx, name)
				if env == nil {
					contract.Assert(!ctx.Diag.Success())
					continue // failure reading the environment information.
				}

				// Now print out the name, last deployment time (if any), and resources (if any).
				lastDeploy := "n/a"
				resourceCount := "n/a"
				if envfile.Latest != nil {
					lastDeploy = envfile.Latest.Time.String()
				}
				if old != nil {
					resourceCount = strconv.Itoa(len(old.Resources()))
				}
				display := env.Name
				if display == curr {
					display += "*" // fancify the current environment.
				}
				fmt.Printf("%-20s %-48s %-12s\n", display, lastDeploy, resourceCount)
			}

			return nil
		}),
	}
}
