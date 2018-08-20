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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/djherbis/times"
	"github.com/pulumi/pulumi/pkg/workspace"

	"github.com/blang/semver"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/cloud"
	"github.com/pulumi/pulumi/pkg/backend/cloud/client"
	"github.com/pulumi/pulumi/pkg/backend/local"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/version"
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
	var color colorFlag

	cmd := &cobra.Command{
		Use: "pulumi",
		PersistentPreRun: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// For all commands, attempt to grab out the --color value provided so we
			// can set the GlobalColorization value to be used by any code that doesn't
			// get DisplayOptions passed in.
			cmdFlag := cmd.Flag("color")
			if cmdFlag != nil {
				err := color.Set(cmdFlag.Value.String())
				if err != nil {
					return err
				}

				cmdutil.SetGlobalColorization(color.Colorization())
			}

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

			checkForUpdate()

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
	cmd.PersistentFlags().Var(
		&color, "color", "Colorize output. Choices are: always, never, raw, auto")

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
	cmd.AddCommand(newUpCmd())
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newWhoAmICmd())

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

	return cmd
}

// checkForUpdate checks to see if the CLI needs to be updated, and if so emits a warning, as well as information
// as to how it can be upgraded.
func checkForUpdate() {
	curVer, err := semver.ParseTolerant(version.Version)
	if err != nil {
		glog.V(3).Infof("error parsing current version: %s", err)
	}

	// We don't care about warning for you to update if you have installed a developer version
	if isDevVersion(curVer) {
		return
	}

	latestVer, oldestAllowedVer, err := getCLIVersionInfo()
	if err != nil {
		glog.V(3).Infof("error fetching latest version information: %s", err)
	}

	if oldestAllowedVer.GT(curVer) {
		cmdutil.Diag().Warningf(diag.RawMessage("", getUpgradeMessage(latestVer, curVer)))
	}
}

// getCLIVersionInfo returns information about the latest version of the CLI and the oldest version that should be
// allowed without warning. It caches data from the server for a day.
func getCLIVersionInfo() (semver.Version, semver.Version, error) {
	latest, oldest, err := getCachedVersionInfo()
	if err == nil {
		return latest, oldest, err
	}

	client := client.NewClient(cloud.DefaultURL(), "")
	latest, oldest, err = client.GetCLIVersionInfo(commandContext())
	if err != nil {
		return semver.Version{}, semver.Version{}, err
	}

	err = cacheVersionInfo(latest, oldest)
	if err != nil {
		glog.V(3).Infof("failed to cache version info: %s", err)
	}

	return latest, oldest, err
}

// cacheVersionInfo saves version information in a cache file to be looked up later.
func cacheVersionInfo(latest semver.Version, oldest semver.Version) error {
	updateCheckFile, err := workspace.GetCachedVersionFilePath()
	if err != nil {
		return err
	}

	file, err := os.OpenFile(updateCheckFile, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(file)

	return json.NewEncoder(file).Encode(cachedVersionInfo{
		LatestVersion:        latest.String(),
		OldestWithoutWarning: oldest.String(),
	})
}

// getCachedVersionInfo reads cached information about the newest CLI version, returning the newest version avaliaible
// as well as the oldest version that should be allowed without warning the user they should upgrade.
func getCachedVersionInfo() (semver.Version, semver.Version, error) {
	updateCheckFile, err := workspace.GetCachedVersionFilePath()
	if err != nil {
		return semver.Version{}, semver.Version{}, err
	}

	ts, err := times.Stat(updateCheckFile)
	if err != nil {
		return semver.Version{}, semver.Version{}, err
	}

	if time.Now().After(ts.ModTime().Add(24 * time.Hour)) {
		return semver.Version{}, semver.Version{}, errors.New("cached expired")
	}

	file, err := os.OpenFile(updateCheckFile, os.O_RDONLY, 0600)
	if err != nil {
		return semver.Version{}, semver.Version{}, err
	}
	defer contract.IgnoreClose(file)

	var cached cachedVersionInfo
	if err = json.NewDecoder(file).Decode(&cached); err != nil {
		return semver.Version{}, semver.Version{}, err
	}

	latest, err := semver.ParseTolerant(cached.LatestVersion)
	if err != nil {
		return semver.Version{}, semver.Version{}, err
	}

	oldest, err := semver.ParseTolerant(cached.OldestWithoutWarning)
	if err != nil {
		return semver.Version{}, semver.Version{}, err
	}

	return latest, oldest, err
}

// cachedVersionInfo is the on disk format of the version information the CLI caches between runs.
type cachedVersionInfo struct {
	LatestVersion        string `json:"latestVersion"`
	OldestWithoutWarning string `json:"oldestWithoutWarning"`
}

// getUpgradeMessage gets a message to display to a user instructing them they are out of date and how to move from
// current to latest.
func getUpgradeMessage(latest semver.Version, current semver.Version) string {
	cmd := getUpgradeCommand()

	msg := fmt.Sprintf("A new version of Pulumi is available. To upgrade from version '%s' to '%s', ", current, latest)
	if cmd != "" {
		msg += "run \n   " + cmd + "\nor "
	}

	msg += "visit https://pulumi.io/install for manual instructions and release notes."
	return msg
}

// getUpgradeCommand returns a command that will upgrade the CLI to the newest version. If we can not determine how
// the CLI was installed, the empty string is returned.
func getUpgradeCommand() string {
	curUser, err := user.Current()
	if err != nil {
		return ""
	}

	exe, err := os.Executable()
	if err != nil {
		return ""
	}

	if filepath.Dir(exe) != filepath.Join(curUser.HomeDir, ".pulumi", "bin") {
		return ""
	}

	if runtime.GOOS != "windows" {
		return "$ curl -sSL https://get.pulumi.com | sh"
	}

	powershellCmd := `"%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe"`

	if _, err := exec.LookPath("powershell"); err == nil {
		powershellCmd = "powershell"
	}

	return "> " + powershellCmd + ` -NoProfile -InputFormat None -ExecutionPolicy Bypass -Command "iex ` +
		`((New-Object System.Net.WebClient).DownloadString('https://get.pulumi.com/install.ps1'))"`
}

func isDevVersion(s semver.Version) bool {
	if len(s.Pre) == 0 {
		return false
	}

	return !s.Pre[0].IsNum && strings.HasPrefix("dev", s.Pre[0].VersionStr)
}

func confirmPrompt(prompt string, name string, opts backend.DisplayOptions) bool {
	if prompt != "" {
		fmt.Print(
			opts.Color.Colorize(
				fmt.Sprintf("%s%s%s\n", colors.SpecAttention, prompt, colors.Reset)))
	}

	fmt.Print(
		opts.Color.Colorize(
			fmt.Sprintf("%sPlease confirm that this is what you'd like to do by typing (%s\"%s\"%s):%s ",
				colors.SpecAttention, colors.BrightWhite, name, colors.SpecAttention, colors.Reset)))

	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line) == name
}
