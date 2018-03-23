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

	"github.com/pulumi/pulumi/pkg/backend/cloud"
	"github.com/pulumi/pulumi/pkg/backend/local"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// NewPulumiCmd creates a new Pulumi Cmd instance.
func NewPulumiCmd() *cobra.Command {
	var logFlow bool
	var logToStderr bool
	var tracing string
	var verbose int
	var cwd string
	cmd := &cobra.Command{
		Use: "pulumi",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if cwd != "" {
				if err := os.Chdir(cwd); err != nil {
					cmdutil.ExitError(err.Error())
				}
			}

			if err := upgradeConfigurationFiles(); err != nil {
				cmdutil.ExitError(err.Error())
			}

			cmdutil.InitLogging(logToStderr, verbose, logFlow)
			cmdutil.InitTracing("pulumi-cli", tracing)
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			glog.Flush()
			cmdutil.CloseTracing()
		},
	}

	// Add additional help that includes which clouds we are logged into.
	defaultHelp := cmd.HelpFunc()
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		defaultHelp(cmd, args)

		loggedInto, current, logErr := cloud.CurrentBackends(cmdutil.Diag())
		contract.IgnoreError(logErr) // we want to make progress anyway.
		if len(loggedInto) > 0 {
			fmt.Printf("\n")
			fmt.Printf("Currently logged into the Pulumi Cloud%s\n", cmdutil.EmojiOr(" ☁️", ""))
			for _, be := range loggedInto {
				var marker string
				if be.Name() == current {
					marker = "*"
				}
				fmt.Printf("    %s%s\n", be.Name(), marker)
			}
		}
	})

	cmd.PersistentFlags().StringVarP(&cwd, "cwd", "C", "", "Run pulumi as if it had been started in another directory")
	cmd.PersistentFlags().BoolVarP(&cmdutil.Emoji, "emoji", "e",
		runtime.GOOS == "darwin", "Enable emojis in the output")
	cmd.PersistentFlags().BoolVar(&local.DisableIntegrityChecking, "disable-integrity-checking", false,
		"Disable integrity checking of checkpoint files")
	cmd.PersistentFlags().BoolVar(&logFlow, "logflow", false, "Flow log settings to child processes (like plugins)")
	cmd.PersistentFlags().BoolVar(&logToStderr, "logtostderr", false, "Log to stderr instead of to files")
	cmd.PersistentFlags().StringVar(&tracing, "tracing", "", "Emit tracing to a Zipkin-compatible tracing endpoint")
	cmd.PersistentFlags().IntVarP(
		&verbose, "verbose", "v", 0, "Enable verbose logging (e.g., v=3); anything >3 is very verbose")

	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newDestroyCmd())
	cmd.AddCommand(newHistoryCmd())
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newLogsCmd())
	cmd.AddCommand(newNewCmd())
	cmd.AddCommand(newPluginCmd())
	cmd.AddCommand(newPreviewCmd())
	cmd.AddCommand(newStackCmd())
	cmd.AddCommand(newUpdateCmd())
	cmd.AddCommand(newUpdate2Cmd())
	cmd.AddCommand(newVersionCmd())

	// Commands specific to the Pulumi Cloud Management Console.
	cmd.AddCommand(newLoginCmd())
	cmd.AddCommand(newLogoutCmd())

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
