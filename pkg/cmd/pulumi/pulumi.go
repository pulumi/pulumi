// Copyright 2016-2024, Pulumi Corporation.
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

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/opentracing/opentracing-go"

	"github.com/blang/semver"
	"github.com/djherbis/times"
	"github.com/moby/term"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/about"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ai"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/auth"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cancel"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/completion"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/config"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/console"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/convert"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/deployment"
	cmdEnv "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/env"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/events"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/install"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/logs"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/markdown"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/newcmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/operations"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/org"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packagecmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/plugin"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/policy"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/project"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/schema"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/state"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/trace"
	cmdVersion "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/version"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/whoami"
	"github.com/pulumi/pulumi/pkg/v3/util/tracing"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/httputil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type commandGroup struct {
	Name     string
	Commands []*cobra.Command
}

func (c *commandGroup) commandWidth() int {
	width := 0
	for _, com := range c.Commands {
		if com.Hidden {
			continue
		}
		newWidth := len(com.Name())
		if newWidth > width {
			width = newWidth
		}
	}
	return width
}

func displayCommands(cgs []commandGroup) {
	width := 0
	for _, cg := range cgs {
		newWidth := cg.commandWidth()
		if newWidth > width {
			width = newWidth
		}
	}

	for _, cg := range cgs {
		if cg.commandWidth() == 0 {
			continue
		}
		fmt.Printf("%s:\n", cg.Name)
		for _, com := range cg.Commands {
			if com.Hidden {
				continue
			}
			spacing := strings.Repeat(" ", width-len(com.Name()))
			fmt.Println("  " + com.Name() + spacing + strings.Repeat(" ", 8) + com.Short)
		}
		fmt.Println()
	}
}

func setCommandGroups(cmd *cobra.Command, rootCgs []commandGroup) {
	for _, cg := range rootCgs {
		for _, com := range cg.Commands {
			cmd.AddCommand(com)
		}
	}

	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		header := c.Long
		if header == "" {
			header = c.Short
		}

		if header != "" {
			fmt.Println(strings.TrimSpace(header))
			fmt.Println()
		}

		if c != cmd.Root() {
			fmt.Print(c.UsageString())
			return
		}

		fmt.Println("Usage:")
		fmt.Println("  pulumi [command]")
		fmt.Println()

		displayCommands(rootCgs)

		fmt.Println("Flags:")
		fmt.Println(cmd.Flags().FlagUsages())

		fmt.Println("Use `pulumi [command] --help` for more information about a command.")
	})
}

type loggingWriter struct{}

func (loggingWriter) Write(bytes []byte) (int, error) {
	logging.Infof("%s", string(bytes))
	return len(bytes), nil
}

// NewPulumiCmd creates a new Pulumi Cmd instance.
func NewPulumiCmd() *cobra.Command {
	var cwd string
	var logFlow bool
	var logToStderr bool
	var tracingFlag string
	var tracingHeaderFlag string
	var profiling string
	var verbose int
	var color string
	var memProfileRate int

	updateCheckResult := make(chan *diag.Diag)
	var updateCancel context.CancelFunc

	cmd := &cobra.Command{
		Use:           "pulumi",
		Short:         "Pulumi command line",
		SilenceErrors: true,
		SilenceUsage:  true,
		Long: "Pulumi - Modern Infrastructure as Code\n" +
			"\n" +
			"To begin working with Pulumi, run the `pulumi new` command:\n" +
			"\n" +
			"    $ pulumi new\n" +
			"\n" +
			"This will prompt you to create a new project for your cloud and language of choice.\n" +
			"\n" +
			"The most common commands from there are:\n" +
			"\n" +
			"    - pulumi up       : Deploy code and/or resource changes\n" +
			"    - pulumi stack    : Manage instances of your project\n" +
			"    - pulumi config   : Alter your stack's configuration or secrets\n" +
			"    - pulumi destroy  : Tear down your stack's resources entirely\n" +
			"\n" +
			"For more information, please visit the project page: https://www.pulumi.com/docs/",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// We run this method for its side-effects. On windows, this will enable the windows terminal
			// to understand ANSI escape codes.
			_, _, _ = term.StdStreams()

			// If we fail before we start the async update check, go ahead and close the
			// channel since we know it will never receive a value.
			var waitForUpdateCheck bool
			var updateCtx context.Context
			updateCtx, updateCancel = context.WithCancel(cmd.Context())
			defer func() {
				if !waitForUpdateCheck {
					close(updateCheckResult)
				}
			}()

			// For all commands, attempt to grab out the --color value provided so we
			// can set the GlobalColorization value to be used by any code that doesn't
			// get DisplayOptions passed in.
			cmdFlag := cmd.Flag("color")
			if cmdFlag != nil {
				err := cmdutil.SetGlobalColorization(cmdFlag.Value.String())
				if err != nil {
					return err
				}
			}

			if cwd != "" {
				if err := os.Chdir(cwd); err != nil {
					return err
				}
			}

			logging.InitLogging(logToStderr, verbose, logFlow)
			cmdutil.InitTracing("pulumi-cli", "pulumi", tracingFlag)

			ctx := cmd.Context()
			if cmdutil.IsTracingEnabled() {
				if cmdutil.TracingRootSpan != nil {
					ctx = opentracing.ContextWithSpan(ctx, cmdutil.TracingRootSpan)
				}

				// This is used to control the contents of the tracing header.
				tracingHeader := os.Getenv("PULUMI_TRACING_HEADER")
				if tracingHeaderFlag != "" {
					tracingHeader = tracingHeaderFlag
				}

				tracingOptions := tracing.Options{
					PropagateSpans: true,
					TracingHeader:  tracingHeader,
				}
				ctx = tracing.ContextWithOptions(ctx, tracingOptions)
			}
			cmd.SetContext(ctx)

			if logging.Verbose >= 11 {
				logging.Warningf("log level 11 will print sensitive information such as api tokens and request headers")
			}

			// The gocloud drivers use the log package to write logs, which by default just writes to stdout. This overrides
			// that so that log messages go to the logging package that we use everywhere else instead.
			loggingWriter := &loggingWriter{}
			log.SetOutput(loggingWriter)

			if profiling != "" {
				if err := cmdutil.InitProfiling(profiling, memProfileRate); err != nil {
					logging.Warningf("could not initialize profiling: %v", err)
				}
			}

			// Run the version check in parallel so that it doesn't block executing the command.
			// If there is a new version to report, we will do so after the command has finished.
			waitForUpdateCheck = true
			go func() {
				updateCheckResult <- checkForUpdate(updateCtx)
				close(updateCheckResult)
			}()

			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			// Before exiting, if there is a new version of the CLI available, print it out.
			jsonFlag := cmd.Flag("json")
			isJSON := jsonFlag != nil && jsonFlag.Value.String() == "true"

			updateCancel()
			checkVersionMsg, ok := <-updateCheckResult
			if ok && checkVersionMsg != nil && !isJSON {
				cmdutil.Diag().Warningf(checkVersionMsg)
			}

			logging.Flush()
			cmdutil.CloseTracing()

			if profiling != "" {
				if err := cmdutil.CloseProfiling(profiling); err != nil {
					logging.Warningf("could not close profiling: %v", err)
				}
			}
		},
	}

	cmd.PersistentFlags().StringVarP(&cwd, "cwd", "C", "",
		"Run pulumi as if it had been started in another directory")
	cmd.PersistentFlags().BoolVarP(&cmdutil.Emoji, "emoji", "e", runtime.GOOS == "darwin",
		"Enable emojis in the output")
	cmd.PersistentFlags().BoolVarP(&cmdutil.FullyQualifyStackNames, "fully-qualify-stack-names", "Q", false,
		"Show fully-qualified stack names")
	cmd.PersistentFlags().BoolVar(&backend.DisableIntegrityChecking, "disable-integrity-checking", false,
		"Disable integrity checking of checkpoint files")
	cmd.PersistentFlags().BoolVar(&logFlow, "logflow", false,
		"Flow log settings to child processes (like plugins)")
	cmd.PersistentFlags().BoolVar(&logToStderr, "logtostderr", false,
		"Log to stderr instead of to files")
	cmd.PersistentFlags().BoolVar(&cmdutil.DisableInteractive, "non-interactive", false,
		"Disable interactive mode for all commands")
	cmd.PersistentFlags().StringVar(&tracingFlag, "tracing", "",
		"Emit tracing to the specified endpoint. Use the `file:` scheme to write tracing data to a local file")
	cmd.PersistentFlags().StringVar(&profiling, "profiling", "",
		"Emit CPU and memory profiles and an execution trace to '[filename].[pid].{cpu,mem,trace}', respectively")
	cmd.PersistentFlags().IntVar(&memProfileRate, "memprofilerate", 0,
		"Enable more precise (and expensive) memory allocation profiles by setting runtime.MemProfileRate")
	cmd.PersistentFlags().IntVarP(&verbose, "verbose", "v", 0,
		"Enable verbose logging (e.g., v=3); anything >3 is very verbose")
	cmd.PersistentFlags().StringVar(
		&color, "color", "auto", "Colorize output. Choices are: always, never, raw, auto")

	setCommandGroups(cmd, []commandGroup{
		// Common commands:
		{
			Name: "Stack Management Commands",
			Commands: []*cobra.Command{
				newcmd.NewNewCmd(),
				config.NewConfigCmd(),
				cmdStack.NewStackCmd(),
				console.NewConsoleCmd(),
				operations.NewImportCmd(),
				operations.NewRefreshCmd(),
				state.NewStateCmd(),
				install.NewInstallCmd(),
			},
		},
		{
			Name: "Deployment Commands",
			Commands: []*cobra.Command{
				operations.NewUpCmd(),
				operations.NewDestroyCmd(),
				operations.NewPreviewCmd(),
				cancel.NewCancelCmd(),
			},
		},
		{
			Name: "Environment Commands",
			Commands: []*cobra.Command{
				cmdEnv.NewEnvCmd(),
			},
		},
		{
			Name: "Pulumi Cloud Commands",
			Commands: []*cobra.Command{
				auth.NewLoginCmd(),
				auth.NewLogoutCmd(),
				whoami.NewWhoAmICmd(pkgWorkspace.Instance, cmdBackend.DefaultLoginManager),
				org.NewOrgCmd(),
				project.NewProjectCmd(),
				deployment.NewDeploymentCmd(),
			},
		},
		{
			Name: "Policy Management Commands",
			Commands: []*cobra.Command{
				policy.NewPolicyCmd(),
			},
		},
		{
			Name: "Plugin Commands",
			Commands: []*cobra.Command{
				plugin.NewPluginCmd(),
				schema.NewSchemaCmd(),
				packagecmd.NewPackageCmd(),
			},
		},
		{
			Name: "Other Commands",
			Commands: []*cobra.Command{
				cmdVersion.NewVersionCmd(),
				about.NewAboutCmd(),
				completion.NewGenCompletionCmd(cmd),
			},
		},

		// Less common, and thus hidden, commands:
		{
			Name: "Hidden Commands",
			Commands: []*cobra.Command{
				markdown.NewGenMarkdownCmd(cmd),
			},
		},

		// We have a set of commands that are still experimental
		//     hidden unless PULUMI_EXPERIMENTAL is set to true.
		{
			Name: "Experimental Commands",
			Commands: []*cobra.Command{
				convert.NewConvertCmd(),
				operations.NewWatchCmd(),
				logs.NewLogsCmd(),
			},
		},
		// We have a set of options that are useful for developers of pulumi
		//    hidden unless PULUMI_DEBUG_COMMANDS is set to true.
		{
			Name: "Developer Commands",
			Commands: []*cobra.Command{
				trace.NewViewTraceCmd(),
				trace.NewConvertTraceCmd(),
				events.NewReplayEventsCmd(),
			},
		},
		// AI Commands relating to specifically the Pulumi AI service
		//     and its related features
		{
			Name: "AI Commands",
			Commands: []*cobra.Command{
				ai.NewAICommand(),
			},
		},
	})

	cmd.PersistentFlags().StringVar(&tracingHeaderFlag, "tracing-header", "",
		"Include the tracing header with the given contents.")

	if !env.DebugCommands.Value() {
		err := cmd.PersistentFlags().MarkHidden("tracing-header")
		contract.IgnoreError(err)
	}

	// Since we define a custom command for generating shell completions
	// (`gen-completion` / `newGenCompletionCmd`), we disable Cobra's default
	// completion command as a recommended best practice.
	cmd.CompletionOptions.DisableDefaultCmd = true

	return cmd
}

// haveNewerDevVersion checks whethere we have a newer dev version available.
func haveNewerDevVersion(devVersion semver.Version, curVersion semver.Version) bool {
	if devVersion.Major != curVersion.Major {
		return devVersion.Major > curVersion.Major
	}
	if devVersion.Minor != curVersion.Minor {
		return devVersion.Minor > curVersion.Minor
	}
	if devVersion.Patch != curVersion.Patch {
		return devVersion.Patch > curVersion.Patch
	}

	// The dev version string looks like: v1.0.0-11-g4ff08363.  We
	// can determine whether we have a newer dev version by
	// comparing the second part of the version string, which is
	// the number of commits since the last tag.
	devVersionParts := strings.Split(devVersion.String(), "-")
	curVersionParts := strings.Split(curVersion.String(), "-")

	// We're being leninent with parsing here.  If we can't parse
	// a version number correctly for any reason, we default to
	// pretending there is no newer version, and not warning the
	// user.  As this is only a warning this is better than
	// asserting or crashing in the error case.
	if len(devVersionParts) != 3 || len(curVersionParts) != 3 {
		return false
	}
	devCommits, err := strconv.Atoi(devVersionParts[1])
	if err != nil {
		return false
	}
	curCommits, err := strconv.Atoi(curVersionParts[1])
	if err != nil {
		return false
	}
	return devCommits > curCommits
}

func checkForExistingTempFile() (*semver.Version, error) {
	homeDir, err := workspace.GetPulumiHomeDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(homeDir, "tmp"))
	if err != nil {
		return nil, err
	}
	tgzRegex := regexp.MustCompile(`pulumi-v(.+)-.+-.+.tar.gz`)
	for _, entry := range entries {
		submatches := tgzRegex.FindStringSubmatch(entry.Name())
		if submatches[1] != "" {
			v := semver.MustParse(submatches[1])
			return &v, nil
		}
	}
	return nil, nil
}

func doDownload(ctx context.Context, version semver.Version) (string, error) {
	homeDir, err := workspace.GetPulumiHomeDir()
	if err != nil {
		return "", err
	}

	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(exe, filepath.Join(homeDir, "bin")) {
		return "", err
	}

	v, err := checkForExistingTempFile()
	if err != nil {
		return "", err
	}
	if v != nil {
		version = *v
	}
	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "arm64"
	case "aarch64":
		arch = "arm64"
	default:
		panic(fmt.Sprintf("unsupported architecture: %s", runtime.GOARCH))
	}
	tgzFile := fmt.Sprintf("pulumi-v%s-%s-%s.tar.gz", version, runtime.GOOS, arch)

	downloadURL := fmt.Sprintf("https://github.com/pulumi/pulumi/releases/download/v%s/", version)
	if isDevVersion(version) || isLocalVersion(version) {
		downloadURL = fmt.Sprintf("https://get.pulumi.com/releases/sdk/")
	}

	downloadURL = fmt.Sprintf("%s/%s", downloadURL, tgzFile)
	fmt.Println(downloadURL)

	os.MkdirAll(filepath.Join(homeDir, "tmp"), 0o700)
	tmpFile := filepath.Join(homeDir, "tmp", tgzFile)
	f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(tmpFile)
	var rangeStart int64
	if err == nil && info != nil {
		rangeStart = info.Size()
	}

	client := client.NewClient(httpstate.DefaultURL(pkgWorkspace.Instance), "", false, cmdutil.Diag())
	resp, err := client.DownloadCLI(ctx, downloadURL, rangeStart)
	if err != nil {
		return "", err
	}

	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		// Something went wrong.  Let's remove the tmp file and try again
		os.Remove(tmpFile)
		return "", err
	}

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return "", err
	}

	if ctx.Err() != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return "", err
	}
	return tmpFile, nil
}

// install a new version of the CLI into `$PULUMI_HOME/bin`. Only works if the currently
// running binary is installed in `$PULUMI_HOME/bin`.
//
// We download the new version piece by piece, stopping when the CLI would normally exit,
// so people on slower internet connections won't have a huge performance impact.
// When a download isn't finished, but there is a new version available, we still continue
// with the download that's currently in progress, so we don't have to start over, and
// never update the CLI. Once the current download is finished, we then check the latest
// version again, potentially skipping a few releases, depending on how long it took.
func installNewCLI(ctx context.Context, version semver.Version) *semver.Version {
	logging.V(3).Infof("installing new CLI version %s", version)
	tmpFile, err := doDownload(ctx, version)
	if err != nil {
		logging.V(3).Infof("error downloading new CLI: %s", err)
		return nil
	}
	homeDir, err := workspace.GetPulumiHomeDir()
	if err != nil {
		return nil
	}
	tmpDir, err := os.MkdirTemp(homeDir, "pulumi-update")
	if err != nil {
		logging.V(3).Infof("error creating temporary directory: %s", err)
		return nil
	}
	defer os.RemoveAll(tmpDir)

	f, err := os.Open(tmpFile)
	if err != nil {
		logging.V(3).Infof("error opening temporary file: %s", err)
		return nil
	}
	defer os.Remove(tmpFile)
	// Extract the tarball into the temporary directory.
	err = archive.ExtractTGZ(f, tmpDir)
	if err != nil {
		logging.V(3).Infof("error extracting new CLI: %s", err)
		return nil
	}
	if runtime.GOOS == "windows" {
		err := os.WriteFile(filepath.Join(tmpDir, "updater.ps1"), []byte(windowsScript), 0o755)
		if err != nil {
			logging.V(3).Infof("error writing updater script: %s", err)
			return nil
		}
		//nolint:gosec
		cmd := exec.Command("powershell", "-File", filepath.Join(tmpDir, "updater.ps1"),
			strconv.Itoa(os.Getpid()), tmpDir, filepath.Join(homeDir, "bin"))
		err = cmd.Start()
		if err != nil {
			logging.V(3).Infof("error running updater script: %s", err)
			return nil
		}
		return &version
	}
	entries, err := os.ReadDir(filepath.Join(tmpDir, "pulumi"))
	if err != nil {
		logging.V(3).Infof("error reading temporary directory: %s", err)
		return nil
	}
	for _, file := range entries {
		if strings.HasPrefix(file.Name(), "pulumi") {
			// Move the new CLI into the current directory.
			err := os.Rename(filepath.Join(tmpDir, "pulumi", file.Name()), filepath.Join(homeDir, "bin", file.Name()))
			if err != nil {
				logging.V(3).Infof("error moving new CLI into place: %s", err)
				return nil
			}
		}
	}
	return &version
}

// checkForUpdate checks to see if the CLI needs to be updated, and if so emits a warning, as well as information
// as to how it can be upgraded.
func checkForUpdate(ctx context.Context) *diag.Diag {
	curVer, err := semver.ParseTolerant(version.Version)
	if err != nil {
		logging.V(3).Infof("error parsing current version: %s", err)
	}

	// We don't care about warning for you to update if you have installed a locally complied version
	isDevVersion := isDevVersion(curVer) || isLocalVersion(curVer)

	var skipUpdateCheck bool
	_, _, _, err = getCachedVersionInfo(isDevVersion)
	if err == nil {
		// If we have a cached version, we already warned the user once
		// in the last 24 hours--the cache is considered stale after that.
		// So we don't need to warn again.
		skipUpdateCheck = true
	}
	latestVer, oldestAllowedVer, devVer, err := getCLIVersionInfo(ctx)
	if err != nil {
		logging.V(3).Infof("error fetching latest version information "+
			"(set `%s=true` to skip update checks): %s", env.SkipUpdateCheck.Var().Name(), err)
	}
	if ctx.Err() != nil {
		logging.V(3).Infof("context cancelled, fetched latest version information")
		return nil
	}

	if (isDevVersion && haveNewerDevVersion(devVer, curVer)) || (!isDevVersion && oldestAllowedVer.GT(curVer)) {
		if os.Getenv("PULUMI_AUTO_UPDATE_CLI") == "true" {
			version := devVer
			if !isDevVersion {
				version = latestVer
			}
			newVersion := installNewCLI(ctx, version)
			if newVersion != nil {
				return diag.RawMessage("", "Pulumi CLI has been updated to the version: "+newVersion.String())
			}
			return nil
		}

		if isDevVersion {
			latestVer = devVer
		}
		msg := getUpgradeMessage(latestVer, curVer, isDevVersion)
		if skipUpdateCheck {
			// If we're skipping the check,
			// still log this to the internal logging system
			// that users don't see by default.
			logging.Warningf("%s", msg)
			return nil
		}
		return diag.RawMessage("", msg)
	}

	return nil
}

// getCLIVersionInfo returns information about the latest version of the CLI and the oldest version that should be
// allowed without warning. It caches data from the server for a day.
func getCLIVersionInfo(ctx context.Context) (semver.Version, semver.Version, semver.Version, error) {
	client := client.NewClient(httpstate.DefaultURL(pkgWorkspace.Instance), "", false, cmdutil.Diag())
	latest, oldest, dev, err := client.GetCLIVersionInfo(ctx)
	if err != nil {
		return semver.Version{}, semver.Version{}, semver.Version{}, err
	}

	brewLatest, isBrew, err := getLatestBrewFormulaVersion()
	if err != nil {
		logging.V(3).Infof("error determining if the running executable was installed with brew: %s", err)
	}
	if isBrew {
		// When consulting Homebrew for version info, we just use the latest version as the oldest allowed.
		latest, oldest, dev = brewLatest, brewLatest, brewLatest
	}

	err = cacheVersionInfo(latest, oldest, dev)
	if err != nil {
		logging.V(3).Infof("failed to cache version info: %s", err)
	}

	return latest, oldest, dev, err
}

// cacheVersionInfo saves version information in a cache file to be looked up later.
func cacheVersionInfo(latest semver.Version, oldest semver.Version, dev semver.Version) error {
	updateCheckFile, err := workspace.GetCachedVersionFilePath()
	if err != nil {
		return err
	}

	file, err := os.OpenFile(updateCheckFile, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(file)

	return json.NewEncoder(file).Encode(cachedVersionInfo{
		LatestVersion:        latest.String(),
		OldestWithoutWarning: oldest.String(),
		LatestDevVersion:     dev.String(),
	})
}

// getCachedVersionInfo reads cached information about the newest CLI version, returning the newest version available,
// the oldest version that should be allowed without warning the user they should upgrade, as well as the
// latest dev version.
func getCachedVersionInfo(devVersion bool) (semver.Version, semver.Version, semver.Version, error) {
	updateCheckFile, err := workspace.GetCachedVersionFilePath()
	if err != nil {
		return semver.Version{}, semver.Version{}, semver.Version{}, err
	}

	ts, err := times.Stat(updateCheckFile)
	if err != nil {
		return semver.Version{}, semver.Version{}, semver.Version{}, err
	}

	cacheTime := 24 * time.Hour
	if devVersion {
		cacheTime = 1 * time.Hour
	}
	if time.Now().After(ts.ModTime().Add(cacheTime)) {
		return semver.Version{}, semver.Version{}, semver.Version{}, errors.New("cached expired")
	}

	file, err := os.OpenFile(updateCheckFile, os.O_RDONLY, 0o600)
	if err != nil {
		return semver.Version{}, semver.Version{}, semver.Version{}, err
	}
	defer contract.IgnoreClose(file)

	var cached cachedVersionInfo
	if err = json.NewDecoder(file).Decode(&cached); err != nil {
		return semver.Version{}, semver.Version{}, semver.Version{}, err
	}

	latest, err := semver.ParseTolerant(cached.LatestVersion)
	if err != nil {
		return semver.Version{}, semver.Version{}, semver.Version{}, err
	}

	oldest, err := semver.ParseTolerant(cached.OldestWithoutWarning)
	if err != nil {
		return semver.Version{}, semver.Version{}, semver.Version{}, err
	}

	dev, err := semver.ParseTolerant(cached.LatestDevVersion)
	if err != nil {
		return semver.Version{}, semver.Version{}, semver.Version{}, err
	}

	return latest, oldest, dev, err
}

// cachedVersionInfo is the on disk format of the version information the CLI caches between runs.
type cachedVersionInfo struct {
	LatestVersion        string `json:"latestVersion"`
	OldestWithoutWarning string `json:"oldestWithoutWarning"`
	LatestDevVersion     string `json:"latestDevVersion"`
}

// getUpgradeMessage gets a message to display to a user instructing them they are out of date and how to move from
// current to latest.
func getUpgradeMessage(latest semver.Version, current semver.Version, isDevVersion bool) string {
	cmd := getUpgradeCommand(isDevVersion)

	msg := fmt.Sprintf("A new version of Pulumi is available. To upgrade from version '%s' to '%s', ", current, latest)

	if cmd != "" {
		msg += "run \n   " + cmd + "\nor "
	}

	msg += "set PULUMI_AUTO_UPDATE_CLI=true in your environment to auto-update the CLI, \nor "
	msg += "visit https://pulumi.com/docs/install/ for manual instructions and release notes."
	return msg
}

// getUpgradeCommand returns a command that will upgrade the CLI to the newest version. If we can not determine how
// the CLI was installed, the empty string is returned.
func getUpgradeCommand(isDevVersion bool) string {
	curUser, err := user.Current()
	if err != nil {
		return ""
	}

	exe, err := os.Executable()
	if err != nil {
		return ""
	}

	isBrew, err := isBrewInstall(exe)
	if err != nil {
		logging.V(3).Infof("error determining if the running executable was installed with brew: %s", err)
	}
	if isBrew {
		return "$ brew update && brew upgrade pulumi"
	}

	if filepath.Dir(exe) != filepath.Join(curUser.HomeDir, workspace.BookkeepingDir, "bin") {
		return ""
	}

	if runtime.GOOS != "windows" {
		command := "$ curl -sSL https://get.pulumi.com | sh"
		if isDevVersion {
			command = command + " -s -- --version dev"
		}
		return command
	}

	powershellCmd := `"%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe"`

	if _, err := exec.LookPath("powershell"); err == nil {
		powershellCmd = "powershell"
	}

	powershellCmd = "> " + powershellCmd + ` -NoProfile -InputFormat None -ExecutionPolicy Bypass -Command "iex ` +
		`((New-Object System.Net.WebClient).DownloadString('https://get.pulumi.com/install.ps1'))"`
	if isDevVersion {
		powershellCmd = powershellCmd + " -version dev"
	}
	return powershellCmd
}

// isBrewInstall returns true if the current running executable is running on macOS and was installed with brew.
func isBrewInstall(exe string) (bool, error) {
	if runtime.GOOS != "darwin" {
		return false, nil
	}

	exePath, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return false, err
	}

	brewBin, err := exec.LookPath("brew")
	if err != nil {
		return false, err
	}

	brewPrefixCmd := exec.Command(brewBin, "--prefix", "pulumi")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	brewPrefixCmd.Stdout = &stdout
	brewPrefixCmd.Stderr = &stderr
	if err = brewPrefixCmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			ee.Stderr = stderr.Bytes()
		}
		return false, fmt.Errorf("'brew --prefix pulumi' failed: %w", err)
	}

	brewPrefixCmdOutput := strings.TrimSpace(stdout.String())
	if brewPrefixCmdOutput == "" {
		return false, errors.New("trimmed output from 'brew --prefix pulumi' is empty")
	}

	brewPrefixPath, err := filepath.EvalSymlinks(brewPrefixCmdOutput)
	if err != nil {
		return false, err
	}

	brewPrefixExePath := filepath.Join(brewPrefixPath, "bin", "pulumi")
	return exePath == brewPrefixExePath, nil
}

func getLatestBrewFormulaVersion() (semver.Version, bool, error) {
	exe, err := os.Executable()
	if err != nil {
		return semver.Version{}, false, err
	}

	isBrew, err := isBrewInstall(exe)
	if err != nil {
		return semver.Version{}, false, err
	}
	if !isBrew {
		return semver.Version{}, false, nil
	}

	const formulaJSON = "https://formulae.brew.sh/api/formula/pulumi.json"
	url, err := url.Parse(formulaJSON)
	contract.AssertNoErrorf(err, "Could not parse URL %q", formulaJSON)

	resp, err := httputil.DoWithRetry(&http.Request{
		Method: http.MethodGet,
		URL:    url,
	}, http.DefaultClient)
	if err != nil {
		return semver.Version{}, false, err
	}
	defer contract.IgnoreClose(resp.Body)

	type versions struct {
		Stable string `json:"stable"`
	}
	var formula struct {
		Versions versions `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&formula); err != nil {
		return semver.Version{}, false, err
	}

	stable, err := semver.ParseTolerant(formula.Versions.Stable)
	if err != nil {
		return semver.Version{}, false, err
	}
	return stable, true, nil
}

func isLocalVersion(s semver.Version) bool {
	if len(s.Pre) == 0 {
		return false
	}

	devStrings := regexp.MustCompile(`alpha|beta|dev|rc`)
	return !s.Pre[0].IsNum && devStrings.MatchString(s.Pre[0].VersionStr)
}

func isDevVersion(s semver.Version) bool {
	if len(s.Pre) == 0 {
		return false
	}

	devRegex := regexp.MustCompile(`\d*-g[0-9a-f]*$`)
	return !s.Pre[0].IsNum && devRegex.MatchString(s.Pre[0].VersionStr)
}

const windowsScript = `
param(
    [Parameter(Mandatory=$true)]
    [int]$ProcessId,

    [Parameter(Mandatory=$true)]
    [string]$SourceFolder,

    [Parameter(Mandatory=$true)]
    [string]$DestinationFolder
)

# Display script information
Write-Host "Process Monitor and File Mover"
Write-Host "Monitoring process ID: $ProcessId"
Write-Host "Will move files from: $SourceFolder"
Write-Host "To: $DestinationFolder"
Write-Host "Waiting for process to exit..."

# Check if source and destination folders exist
if (-not (Test-Path -Path $SourceFolder)) {
    Write-Error "Source folder does not exist: $SourceFolder"
    exit 1
}

if (-not (Test-Path -Path $DestinationFolder)) {
    Write-Host "Destination folder does not exist. Creating it now."
    New-Item -ItemType Directory -Path $DestinationFolder | Out-Null
}

# Function to check if process is running
function Test-ProcessRunningById {
    param (
        [int]$Id
    )

    $process = Get-Process -Id $Id -ErrorAction SilentlyContinue
    return $null -ne $process
}

# Wait for the process to exit if it's running
try {
    if (Test-ProcessRunningById -Id $ProcessId) {
        $processInfo = Get-Process -Id $ProcessId
        Write-Host "Process ID $ProcessId ($($processInfo.ProcessName)) is running. Waiting for it to exit..."

        do {
            Start-Sleep -Seconds 5
        } while (Test-ProcessRunningById -Id $ProcessId)

        Write-Host "Process ID $ProcessId has exited."
    } else {
        Write-Host "Process ID $ProcessId is not currently running or doesn't exist."
        exit 1
    }
} catch {
    Write-Error "Error checking process status: $_"
    exit 1
}

# Move files from source to destination
Write-Host "Moving files from $SourceFolder to $DestinationFolder"

$files = Get-ChildItem -Path $SourceFolder -File

if ($files.Count -eq 0) {
    Write-Host "No files found in source folder."
} else {
    foreach ($file in $files) {
        $destinationPath = Join-Path -Path $DestinationFolder -ChildPath $file.Name

        # Check if file already exists in destination
        if (Test-Path -Path $destinationPath) {
            Write-Warning "File already exists in destination: $($file.Name)"
            $newName = "{0}_{1}{2}" -f $file.BaseName, (Get-Date -Format "yyyyMMdd_HHmmss"), $file.Extension
            $destinationPath = Join-Path -Path $DestinationFolder -ChildPath $newName
            Write-Host "Renaming to: $newName"
        }

        # Move the file
        try {
            Move-Item -Path $file.FullName -Destination $destinationPath -Force
            Write-Host "Moved: $($file.Name)"
        } catch {
            Write-Error "Failed to move file $($file.Name): $_"
        }
    }

    Write-Host "File moving complete. Moved $($files.Count) files."
}

Write-Host "Script execution complete."
`
