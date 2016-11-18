// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/marapongo/mu/pkg/compiler"
	"github.com/marapongo/mu/pkg/compiler/backends"
	"github.com/marapongo/mu/pkg/compiler/backends/clouds"
	"github.com/marapongo/mu/pkg/compiler/backends/schedulers"
	"github.com/spf13/cobra"
)

// defaultIn is where the Mu compiler looks for inputs by default.
const defaultInp = "."

// defaultOutput is where the Mu compiler places build artifacts by default.
const defaultOutp = ".mu"

func newBuildCmd() *cobra.Command {
	var outp string
	var arch string
	var target string
	var cmd = &cobra.Command{
		Use:   "build [source]",
		Short: "Compile a Mu Stack",
		Run: func(cmd *cobra.Command, args []string) {
			inp := defaultInp
			if len(args) > 0 {
				inp = args[0]
			}

			abs, err := filepath.Abs(inp)
			if err != nil {
				glog.Fatal(err)
			}

			opts := compiler.DefaultOpts(abs)
			opts.Target = target
			setCloudArchOptions(arch, &opts)

			mup := compiler.NewCompiler(opts)
			mup.Build(abs, outp)
		},
	}

	cmd.PersistentFlags().StringVar(
		&outp, "out", defaultOutp,
		"The directory in which to place build artifacts")
	cmd.PersistentFlags().StringVarP(
		&arch, "arch", "a", "",
		"Generate output for a specific cloud architecture (format: \"cloud[:scheduler]\")")
	cmd.PersistentFlags().StringVarP(
		&target, "target", "t", "",
		"Generate output for an existing, named target")

	return cmd
}

func setCloudArchOptions(arch string, opts *compiler.Options) {
	// If an architecture was specified, parse the pieces and set the options.  This isn't required because stacks
	// and workspaces can have defaults.  This simply overrides or provides one where none exists.
	if arch != "" {
		// The format is "cloud[:scheduler]"; parse out the pieces.
		var cloud string
		var cloudScheduler string
		if delim := strings.IndexRune(arch, ':'); delim != -1 {
			cloud = arch[:delim]
			cloudScheduler = arch[delim+1:]
		} else {
			cloud = arch
		}

		cloudArch, ok := clouds.ArchMap[cloud]
		if !ok {
			fmt.Fprintf(os.Stderr, "Unrecognized cloud arch '%v'\n", cloud)
			os.Exit(-1)
		}

		var cloudSchedulerArch schedulers.Arch
		if cloudScheduler != "" {
			cloudSchedulerArch, ok = schedulers.ArchMap[cloudScheduler]
			if !ok {
				fmt.Fprintf(os.Stderr, "Unrecognized cloud scheduler arch '%v'\n", cloudScheduler)
				os.Exit(-1)
			}
		}

		opts.Arch = backends.Arch{
			Cloud:     cloudArch,
			Scheduler: cloudSchedulerArch,
		}
	}
}
