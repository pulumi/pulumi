// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/marapongo/mu/pkg/compiler"
	"github.com/marapongo/mu/pkg/compiler/backends"
	"github.com/marapongo/mu/pkg/compiler/backends/clouds"
	"github.com/marapongo/mu/pkg/compiler/backends/schedulers"
)

// defaultIn is where the Mu compiler looks for inputs by default.
const defaultInp = "."

// defaultOutput is where the Mu compiler places build artifacts by default.
const defaultOutp = ".mu"

func newBuildCmd() *cobra.Command {
	var outp string
	var cluster string
	var targetArch string
	var skipCodegen bool
	var cmd = &cobra.Command{
		Use:   "build [source] -- [args]",
		Short: "Compile a Mu Stack",
		Run: func(cmd *cobra.Command, args []string) {
			flags := cmd.Flags()
			ddash := flags.ArgsLenAtDash()

			// If there's a --, we need to separate out the command args from the stack args.
			var sargs []string
			if ddash != -1 {
				sargs = args[ddash:]
				args = args[0:ddash]
			}

			// Fetch the input source directory.
			inp := defaultInp
			if len(args) > 0 {
				inp = args[0]
			}
			abs, err := filepath.Abs(inp)
			if err != nil {
				glog.Fatal(err)
			}

			opts := compiler.DefaultOpts(abs)

			if skipCodegen {
				opts.SkipCodegen = true
			}

			// Set the cluster and architecture if specified.
			opts.Cluster = cluster
			setCloudArchOptions(targetArch, opts)

			// See if there are any arguments and, if so, accumulate them.
			if len(sargs) > 0 {
				opts.Args = make(map[string]string)
				// TODO[marapongo/mu#7]: This is a very rudimentary parser.  We can and should do better.
				for i := 0; i < len(sargs); i++ {
					sarg := sargs[i]

					// Eat - or -- at the start.
					if sarg[0] == '-' {
						sarg = sarg[1:]
						if sarg[0] == '-' {
							sarg = sarg[1:]
						}
					}
					// Now find an k=v, and split the k/v part.
					if eq := strings.IndexByte(sarg, '='); eq != -1 {
						opts.Args[sarg[:eq]] = sarg[eq+1:]
					} else {
						// No =; if the next arg doesn't start with '-', use it.  Else it  must be a boolean "true".
						if i+1 < len(sargs) && sargs[i+1][0] != '-' {
							opts.Args[sarg] = sargs[i+1]
							i++
						} else {
							// TODO(joe): support --no-key style "false"s.
							opts.Args[sarg] = "true"
						}
					}
				}

			}

			// Now new up a compiler and actually perform the build.
			mup := compiler.NewCompiler(opts)
			mup.Build(abs, outp)
		},
	}

	cmd.PersistentFlags().StringVar(
		&outp, "out", defaultOutp,
		"The directory in which to place build artifacts")
	cmd.PersistentFlags().StringVarP(
		&cluster, "cluster", "c", "",
		"Generate output for an existing, named cluster")
	cmd.PersistentFlags().StringVarP(
		&targetArch, "target", "t", "",
		"Generate output for the target cloud architecture (format: \"cloud[:scheduler]\")")
	cmd.PersistentFlags().BoolVar(
		&skipCodegen, "skip-codegen", false,
		"Skip code-generation phases of the compiler")

	return cmd
}

func setCloudArchOptions(arch string, opts *compiler.Options) {
	// If an architecture was specified, parse the pieces and set the options.  This isn't required because stacks
	// and workspaces can have defaults.  This simply overrides or provides one where none exists.
	if arch != "" {
		// The format is "cloud[:scheduler]"; parse out the pieces.
		var cloud string
		var scheduler string
		if delim := strings.IndexRune(arch, ':'); delim != -1 {
			cloud = arch[:delim]
			scheduler = arch[delim+1:]
		} else {
			cloud = arch
		}

		cloudArch, ok := clouds.Values[cloud]
		if !ok {
			fmt.Fprintf(os.Stderr, "Unrecognized cloud arch '%v'\n", cloud)
			os.Exit(-1)
		}

		var schedulerArch schedulers.Arch
		if scheduler != "" {
			schedulerArch, ok = schedulers.Values[scheduler]
			if !ok {
				fmt.Fprintf(os.Stderr, "Unrecognized cloud scheduler arch '%v'\n", scheduler)
				os.Exit(-1)
			}
		}

		opts.Arch = backends.Arch{
			Cloud:     cloudArch,
			Scheduler: schedulerArch,
		}
	}
}
