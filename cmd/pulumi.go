// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"bufio"
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

var (
	// The lumi engine provides an API for common lumi tasks.  It's shared across the
	// `pulumi` command and the deployment engine in the pulumi-service. For `pulumi` we set
	// the engine to write output and errors to os.Stdout and os.Stderr.
	lumiEngine engine.Engine
)

func init() {
	lumiEngine = engine.Engine{Stdout: os.Stdout, Stderr: os.Stderr, Environment: localEnvProvider{}}
}

// NewPulumiCmd creates a new Pulumi Cmd instance.
func NewPulumiCmd() *cobra.Command {
	var logFlow bool
	var logToStderr bool
	var verbose int
	cmd := &cobra.Command{
		Use: "pulumi",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cmdutil.InitLogging(logToStderr, verbose, logFlow)
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			glog.Flush()
		},
	}

	cmd.PersistentFlags().BoolVar(&logFlow, "logflow", false, "Flow log settings to child processes (like plugins)")
	cmd.PersistentFlags().BoolVar(&logToStderr, "logtostderr", false, "Log to stderr instead of to files")
	cmd.PersistentFlags().IntVarP(
		&verbose, "verbose", "v", 0, "Enable verbose logging (e.g., v=3); anything >3 is very verbose")

	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newDestroyCmd())
	cmd.AddCommand(newEnvCmd())
	cmd.AddCommand(newPreviewCmd())
	cmd.AddCommand(newPushCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}

func pkgargFromArgs(args []string) string {
	if len(args) > 0 {
		return args[0]
	}

	return ""
}

func confirmPrompt(msg string, name string) bool {
	prompt := fmt.Sprintf(msg, name)
	fmt.Print(
		colors.ColorizeText(fmt.Sprintf("%v%v%v\n", colors.SpecAttention, prompt, colors.Reset)))
	fmt.Printf("Please confirm that this is what you'd like to do by typing (\"%v\"): ", name)
	reader := bufio.NewReader(os.Stdin)
	if line, _ := reader.ReadString('\n'); line != name+"\n" {
		fmt.Fprintf(os.Stderr, "Confirmation declined -- exiting without doing anything\n")
		return false
	}
	return true
}
