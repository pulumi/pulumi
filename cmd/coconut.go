// Copyright 2017 Pulumi, Inc. All rights reserved.

package cmd

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/pulumi/coconut/pkg/compiler/core"
	"github.com/pulumi/coconut/pkg/diag"
)

func NewCoconutCmd() *cobra.Command {
	var logToStderr bool
	var verbose int
	cmd := &cobra.Command{
		Use:   "coconut",
		Short: "Coconut is a framework and toolset for reusable stacks of services",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Ensure the glog library has been initialized, including calling flag.Parse beforehand.  Unfortunately,
			// this is the only way to control the way glog runs.  That includes poking around at flags below.
			flag.Parse()
			if logToStderr {
				flag.Lookup("logtostderr").Value.Set("true")
			}
			if verbose > 0 {
				flag.Lookup("v").Value.Set(strconv.Itoa(verbose))
			}
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			glog.Flush()
		},
	}

	cmd.PersistentFlags().BoolVar(&logToStderr, "logtostderr", false, "Log to stderr instead of to files")
	cmd.PersistentFlags().IntVarP(
		&verbose, "verbose", "v", 0, "Enable verbose logging (e.g., v=3); anything >3 is very verbose")

	cmd.AddCommand(newDeployCmd())
	cmd.AddCommand(newEnvCmd())
	cmd.AddCommand(newPackCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}

var snk diag.Sink

// sink lazily allocates a sink to be used if we can't create a compiler.
func sink() diag.Sink {
	if snk == nil {
		snk = core.DefaultSink("")
	}
	return snk
}

// runFunc wraps an error-returning run func with standard Coconut error handling.  All Coconut commands should wrap
// themselves in this to ensure consistent and appropriate error behavior.  In particular, we want to avoid any calls to
// os.Exit in the middle of a callstack which might prohibit reaping of child processes, resources, etc.  And we wish to
// avoid the default Cobra unhandled error behavior, because it is formatted incorrectly and needlessly prints usage.
func runFunc(run func(cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		if err := run(cmd, args); err != nil {
			exitError(err.Error())
		}
	}
}

// exitErrorPrefix is auto-appended to any abrupt command exit.
const exitErrorPrefix = "fatal: "

// exitError issues an error and exits with a standard error exit code.
func exitError(msg string, args ...interface{}) {
	exitErrorCode(-1, msg, args...)
}

// exitErrorCode issues an error and exists with the given error exit code.
func exitErrorCode(code int, msg string, args ...interface{}) {
	sink().Errorf(diag.Message(exitErrorPrefix + fmt.Sprintf(msg, args...)))
	os.Exit(code)
}
