// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/encoding"

	"github.com/pulumi/pulumi/pkg/tokens"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newStackLsCmd() *cobra.Command {
	if usePulumiCloudCommands() {
		return newCloudStackLsCmd()
	}
	return newFAFStackLsCmd()
}

func newFAFStackLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all known stacks",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			var backend pulumiBackend = &localPulumiBackend{}

			currentStack, err := getCurrentStack()
			if err != nil {
				// If we couldn't figure out the current stack, just don't print the '*' later
				// on instead of failing.
				currentStack = tokens.QName("")
			}

			summaries, err := backend.GetStacks()
			if err != nil {
				return err
			}

			displayStacks(summaries, currentStack)
			return nil
		}),
	}
}

func getStacks() ([]tokens.QName, error) {
	var stacks []tokens.QName

	w, err := newWorkspace()
	if err != nil {
		return nil, err
	}

	// Read the stack directory.
	path := w.StackPath("")

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

func newCloudStackLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all known stacks",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			stacks, err := getCloudStacks()
			if err != nil {
				return nil
			}

			// Map to a summary slice.
			var summaries []stackSummary
			for _, stack := range stacks {
				summary := stackSummary{
					Name:          stack.StackName,
					LastDeploy:    "n/a", // TODO(pulumi-service/issues#249): Make this info available.
					ResourceCount: strconv.Itoa(len(stack.Resources)),
				}
				// If the stack hasn't been pushed to, it's resource count doesn't matter.
				if stack.ActiveUpdate == "" {
					summary.ResourceCount = "n/a"
				}
				summaries = append(summaries, summary)
			}

			// Ignore the error, since current stack is just cosmetic for the display.
			currentStack, err := getCurrentStack()
			if err != nil {
				currentStack = tokens.QName("")
			}

			displayStacks(summaries, currentStack)
			return nil
		}),
	}
}

// getCloudStacks returns all stacks for the current repository x workspace on the Pulumi Cloud.
func getCloudStacks() ([]apitype.Stack, error) {
	// Look up the owner, repository, and project from the workspace and nearest package.
	w, err := newWorkspace()
	if err != nil {
		return nil, err
	}
	projID, err := getCloudProjectIdentifier(w)
	if err != nil {
		return nil, err
	}

	// Query all stacks for the project on Pulumi.
	var stacks []apitype.Stack
	path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks", projID.Owner, projID.Repository, projID.Project)
	if err := pulumiRESTCall("GET", path, nil, &stacks); err != nil {
		return nil, err
	}
	return stacks, nil
}

// displayStacks prints the list of stacks to STDOUT. An optional current stack name,
// if present, will have a star by it.
func displayStacks(stacks []stackSummary, current tokens.QName) {
	fmt.Printf("%-20s %-48s %-12s\n", "NAME", "LAST UPDATE", "RESOURCE COUNT")
	for _, stack := range stacks {
		if stack.Name == current {
			stack.Name += "*"
		}
		fmt.Printf("%-20s %-48s %-12s\n", stack.Name, stack.LastDeploy, stack.ResourceCount)
	}
}
