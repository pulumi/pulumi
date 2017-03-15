// Copyright 2017 Pulumi, Inc. All rights reserved.

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/pulumi/coconut/pkg/encoding"
	"github.com/pulumi/coconut/pkg/resource"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
	"github.com/pulumi/coconut/pkg/workspace"
)

func newEnvLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List all known environments",
		Run: runFunc(func(cmd *cobra.Command, args []string) error {
			// Read the environment directory.
			path := workspace.EnvPath("")
			files, err := ioutil.ReadDir(path)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("could not read environments: %v", err)
			}

			// Create a new context to share amongst all of the loads.
			ctx := resource.NewContext(sink())
			defer ctx.Close()

			fmt.Printf("%-20s %-48s %-12s\n", "NAME", "LAST DEPLOYMENT", "RESOURCE COUNT")
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
				fmt.Printf("%-20s %-48s %-12s\n", env.Name, lastDeploy, resourceCount)
			}

			return nil
		}),
	}
}
