// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newStackLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all known stacks",
		Args:  cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Get a list of all known backends, as we will query them all.
			bes, hasClouds := allBackends()

			// Get the current stack so we can print a '*' next to it.
			var current tokens.QName
			if s, _ := state.CurrentStack(bes); s != nil {
				// If we couldn't figure out the current stack, just don't print the '*' later on instead of failing.
				current = s.Name
			}

			// Now produce a list of summaries, and enumerate them sorted by name.
			var result error
			var stackNames []string
			var success bool
			stacks := make(map[string]*backend.Stack)
			for _, b := range bes {
				bs, err := b.ListStacks()
				if err != nil {
					// Remember the error, but keep going, so that if the cloud is unavailable we still show
					// something useful for the local development case.
					result = multierror.Append(result,
						errors.Wrapf(err, "could not list %s stacks", b.Name()))
					continue
				}
				for _, stack := range bs {
					name := string(stack.Name)
					stacks[name] = stack
					stackNames = append(stackNames, name)
				}
				success = true
			}
			sort.Strings(stackNames)

			// Finally, print them all.
			if success {
				fmt.Printf("%-20s %-48s %-18s %-25s\n", "NAME", "LAST UPDATE", "RESOURCE COUNT", "CLOUD URL")
				for _, name := range stackNames {
					// Mark the name as current '*' if we've selected it.
					stack := stacks[name]
					if name == string(current) {
						name += "*"
					}

					// Get last deployment info, provided that it exists.
					none := "n/a"
					lastUpdate := none
					resourceCount := none
					if stack.Snapshot != nil {
						if t := stack.Snapshot.Manifest.Time; !t.IsZero() {
							lastUpdate = t.String()
						}
						resourceCount = strconv.Itoa(len(stack.Snapshot.Resources))
					}

					// Print out the cloud URL.
					cloudURL := stack.CloudURL
					if cloudURL == "" {
						cloudURL = none
					}

					fmt.Printf("%-20s %-48s %-18s %-25s\n", name, lastUpdate, resourceCount, cloudURL)
				}

				// If we aren't logged into any clouds, print a warning, since it could be a mistake.
				if !hasClouds {
					fmt.Fprintf(os.Stderr, "\n")
					fmt.Fprintf(os.Stderr, "Only local stacks being shown; to see Pulumi Cloud stacks, `pulumi login`\n")
				}
			}

			return result
		}),
	}
}
