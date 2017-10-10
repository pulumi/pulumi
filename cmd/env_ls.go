// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/workspace"

	"github.com/pulumi/pulumi/pkg/tokens"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newEnvLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all known environments",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			currentEnv, err := getCurrentEnv()
			if err != nil {
				// If we couldn't figure out the current environment, just don't print the '*' later
				// on instead of failing.
				currentEnv = tokens.QName("")
			}

			envs, err := getEnvironments()
			if err != nil {
				return err
			}

			fmt.Printf("%-20s %-48s %-12s\n", "NAME", "LAST UPDATE", "RESOURCE COUNT")
			for _, env := range envs {
				_, snapshot, _, err := lumiEngine.Environment.GetEnvironment(env)
				if err != nil {
					continue
				}

				// Now print out the name, last deployment time (if any), and resources (if any).
				lastDeploy := "n/a"
				resourceCount := "n/a"
				if snapshot != nil {
					lastDeploy = snapshot.Time.String()
					resourceCount = strconv.Itoa(len(snapshot.Resources))
				}
				display := env.String()
				if env == currentEnv {
					display += "*" // fancify the current environment.
				}
				fmt.Printf("%-20s %-48s %-12s\n", display, lastDeploy, resourceCount)
			}

			return nil
		}),
	}
}

func getEnvironments() ([]tokens.QName, error) {
	var envs []tokens.QName

	// Read the environment directory.
	path := workspace.EnvPath("")
	files, err := ioutil.ReadDir(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Errorf("could not read environments: %v", err)
	}

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
		_, _, _, err := lumiEngine.Environment.GetEnvironment(name)
		if err != nil {
			continue // failure reading the environment information.
		}

		envs = append(envs, name)
	}

	return envs, nil
}
