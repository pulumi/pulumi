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

func newStackLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all known stacks",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			currentStack, err := getCurrentStack()
			if err != nil {
				// If we couldn't figure out the current stack, just don't print the '*' later
				// on instead of failing.
				currentStack = tokens.QName("")
			}

			stacks, err := getStacks()
			if err != nil {
				return err
			}

			fmt.Printf("%-20s %-48s %-12s\n", "NAME", "LAST UPDATE", "RESOURCE COUNT")
			for _, stack := range stacks {
				_, _, snapshot, err := getStack(stack)
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
				display := stack.String()
				if stack == currentStack {
					display += "*" // fancify the current stack.
				}
				fmt.Printf("%-20s %-48s %-12s\n", display, lastDeploy, resourceCount)
			}

			return nil
		}),
	}
}

func getStacks() ([]tokens.QName, error) {
	var stacks []tokens.QName

	// Read the stack directory.
	path := workspace.StackPath("")
	files, err := ioutil.ReadDir(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Errorf("could not read stacks: %v", err)
	}

	for _, file := range files {
		// Ignore directories.
		if file.IsDir() {
			continue
		}

		// Skip files without valid extensions (e.g., *.bak files).
		stackfn := file.Name()
		ext := filepath.Ext(stackfn)
		if _, has := encoding.Marshalers[ext]; !has {
			continue
		}

		// Read in this stack's information.
		name := tokens.QName(stackfn[:len(stackfn)-len(ext)])
		_, _, _, err := getStack(name)
		if err != nil {
			continue // failure reading the stack information.
		}

		stacks = append(stacks, name)
	}

	return stacks, nil
}
