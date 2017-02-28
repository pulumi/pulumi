// Copyright 2016 Pulumi, Inc. All rights reserved.

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

func newHuskLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List all known husks",
		Run: func(cmd *cobra.Command, args []string) {
			path := workspace.HuskPath("")
			files, err := ioutil.ReadDir(path)
			if err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "fatal: could not read husks: %v\n", err)
				os.Exit(-1)
			}

			fmt.Printf("%-20s %-48s %-12s\n", "NAME", "LAST DEPLOYMENT", "RESOURCE COUNT")
			for _, file := range files {
				// Ignore directories.
				if file.IsDir() {
					continue
				}

				// Skip files without valid extensions (e.g., *.bak files).
				huskfn := file.Name()
				ext := filepath.Ext(huskfn)
				if _, has := encoding.Marshalers[ext]; !has {
					continue
				}

				// Create a new context and read in the husk information.
				name := tokens.QName(huskfn[:len(huskfn)-len(ext)])
				ctx := resource.NewContext(sink())
				huskfile, husk, old := readHusk(ctx, name)
				if husk == nil {
					contract.Assert(!ctx.Diag.Success())
					continue // failure reading the husk information.
				}

				// Now print out the name, last deployment time (if any), and resources (if any).
				lastDeploy := "n/a"
				resourceCount := "n/a"
				if huskfile.Latest != nil {
					lastDeploy = huskfile.Latest.Time.String()
				}
				if old != nil {
					resourceCount = strconv.Itoa(len(old.Resources()))
				}
				fmt.Printf("%-20s %-48s %-12s\n", husk.Name, lastDeploy, resourceCount)
			}
		},
	}
}
