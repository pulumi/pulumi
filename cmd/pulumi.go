// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend/local"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

// NewPulumiCmd creates a new Pulumi Cmd instance.
func NewPulumiCmd() *cobra.Command {
	var cwd string
	var logFlow bool
	var logToStderr bool
	var tracing string
	var profiling string
	var verbose int

	cmd := &cobra.Command{
		Use: "pulumi",
		PersistentPreRun: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			if cwd != "" {
				if err := os.Chdir(cwd); err != nil {
					return err
				}
			}

			cmdutil.InitLogging(logToStderr, verbose, logFlow)
			cmdutil.InitTracing("pulumi-cli", "pulumi", tracing)

			if profiling != "" {
				if err := cmdutil.InitProfiling(profiling); err != nil {
					glog.Warningf("could not initialize profiling: %v", err)
				}
			}

			return nil
		}),
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			glog.Flush()
			cmdutil.CloseTracing()

			if profiling != "" {
				if err := cmdutil.CloseProfiling(profiling); err != nil {
					glog.Warningf("could not close profiling: %v", err)
				}
			}
		},
	}

	// Add additional help that includes a link to the docs website.
	defaultHelp := cmd.HelpFunc()
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		defaultHelp(cmd, args)
		fmt.Println("")
		fmt.Println("Additional documentation available at https://docs.pulumi.com")
	})

	cmd.PersistentFlags().StringVarP(&cwd, "cwd", "C", "",
		"Run pulumi as if it had been started in another directory")
	cmd.PersistentFlags().BoolVarP(&cmdutil.Emoji, "emoji", "e", runtime.GOOS == "darwin",
		"Enable emojis in the output")
	cmd.PersistentFlags().BoolVar(&local.DisableIntegrityChecking, "disable-integrity-checking", false,
		"Disable integrity checking of checkpoint files")
	cmd.PersistentFlags().BoolVar(&logFlow, "logflow", false,
		"Flow log settings to child processes (like plugins)")
	cmd.PersistentFlags().BoolVar(&logToStderr, "logtostderr", false,
		"Log to stderr instead of to files")
	cmd.PersistentFlags().StringVar(&tracing, "tracing", "",
		"Emit tracing to a Zipkin-compatible tracing endpoint")
	cmd.PersistentFlags().StringVar(&profiling, "profiling", "",
		"Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively")
	cmd.PersistentFlags().IntVarP(&verbose, "verbose", "v", 0,
		"Enable verbose logging (e.g., v=3); anything >3 is very verbose")

	// Common commands:
	cmd.AddCommand(newCancelCmd())
	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newDestroyCmd())
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newLoginCmd())
	cmd.AddCommand(newLogoutCmd())
	cmd.AddCommand(newLogsCmd())
	cmd.AddCommand(newNewCmd())
	cmd.AddCommand(newPluginCmd())
	cmd.AddCommand(newPreviewCmd())
	cmd.AddCommand(newRefreshCmd())
	cmd.AddCommand(newStackCmd())
	cmd.AddCommand(newUpdateCmd())
	cmd.AddCommand(newVersionCmd())

	// Less common, and thus hidden, commands:
	cmd.AddCommand(newGenBashCompletionCmd(cmd))
	cmd.AddCommand(newGenMarkdownCmd(cmd))

	// We have a set of commands that are useful for developers of pulumi that we add when PULUMI_DEBUG_COMMANDS is
	// set to true.
	if cmdutil.IsTruthy(os.Getenv("PULUMI_DEBUG_COMMANDS")) {
		cmd.AddCommand(newArchiveCommand())
	}

	// Tell flag about -C, so someone can do pulumi -C <working-directory> stack and the call to cmdutil.InitLogging
	// which calls flag.Parse under the hood doesn't yell at you.
	//
	// TODO[pulumi/pulumi#301]: when we move away from using glog, it should be safe to remove this.
	flag.StringVar(&cwd, "C", "", "Run pulumi as if it had been started in another directory")

	return cmd
}

func confirmPrompt(prompt string, name string) bool {
	if prompt != "" {
		fmt.Print(
			colors.ColorizeText(
				fmt.Sprintf("%s%s%s\n", colors.SpecAttention, prompt, colors.Reset)))
	}

	fmt.Print(
		colors.ColorizeText(
			fmt.Sprintf("%sPlease confirm that this is what you'd like to do by typing (%s\"%s\"%s):%s ",
				colors.SpecAttention, colors.BrightWhite, name, colors.SpecAttention, colors.Reset)))

	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line) == name
}
