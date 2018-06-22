// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend/local"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/logging"
)

// NewPulumiCmd creates a new Pulumi Cmd instance.
func NewPulumiCmd() *cobra.Command {
	var cwd string
	var logFlow bool
	var logToStderr bool
	var tracing string
	var tracingHeaderFlag string
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

			logging.InitLogging(logToStderr, verbose, logFlow)
			cmdutil.InitTracing("pulumi-cli", "pulumi", tracing)
			if tracingHeaderFlag != "" {
				tracingHeader = tracingHeaderFlag
			}

			if profiling != "" {
				if err := cmdutil.InitProfiling(profiling); err != nil {
					logging.Warningf("could not initialize profiling: %v", err)
				}
			}

			return nil
		}),
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			logging.Flush()
			cmdutil.CloseTracing()

			if profiling != "" {
				if err := cmdutil.CloseProfiling(profiling); err != nil {
					logging.Warningf("could not close profiling: %v", err)
				}
			}
		},
	}

	// Add additional help that includes a link to the docs website.
	defaultHelp := cmd.HelpFunc()
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		defaultHelp(cmd, args)
		fmt.Println("")
		fmt.Println("Additional documentation available at https://pulumi.io")
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
	if hasDebugCommands() {
		cmd.AddCommand(newArchiveCommand())

		cmd.PersistentFlags().StringVar(&tracingHeaderFlag, "tracing-header", "",
			"Include the tracing header with the given contents.")
	}

	reportCommandInvocations(cmd.Name(), cmd)

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

// reportCommandInvocations hooks into the command's PreRun callback (chaining if necessary) to
// send a telemetry event indiciating a CLI command was invoked. The command's name is prefixed
// with namePrefix, so command names are registered as "pulumi-stack-ls".
func reportCommandInvocations(namePrefix string, cmd *cobra.Command) {
	// Chain in the PreRun callback.
	prevPreRun := cmd.PreRun
	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		if prevPreRun != nil {
			prevPreRun(cmd, args)
		}

		// Walk through the flags that have been set in lexicographical order.
		var flags []string
		cmd.Flags().Visit(func(f *pflag.Flag) {
			flags = append(flags, f.Name)
		})

		err := reportCommanInvocation(apitype.CommandInvocation{
			Command: namePrefix,
			Flags:   flags,
		})
		if err != nil {
			logging.V(7).Infof("error reporting command: %v", err)
		}
	}

	// Report child commands if invoked, too.
	for _, childCmd := range cmd.Commands() {
		reportCommandInvocations(fmt.Sprintf("%s-%s", namePrefix, childCmd.Name()), childCmd)
	}
}

// reportCommanInvocation sends the command invocation event to a web endpoint.
func reportCommanInvocation(c apitype.CommandInvocation) error {
	const endpoint = "https://api.pulumi.com/api/cli/invocation"

	json, err := json.Marshal(c)
	if err != nil {
		return err
	}

	client := http.DefaultClient
	resp, err := client.Post(endpoint, "application/json", bytes.NewBuffer(json))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 299 {
		return errors.Errorf("got non-200 response code: %d", resp.StatusCode)
	}
	return nil
}
