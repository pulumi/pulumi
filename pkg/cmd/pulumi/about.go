// Copyright 2016-2021, Pulumi Corporation.
// pts
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
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/host"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/python"
)

const (
	windows = "windows"
)

func newAboutCmd() *cobra.Command {
	var jsonOut bool
	short := "Print information about the Pulumi enviroment."
	cmd :=
		&cobra.Command{
			Use:   "about",
			Short: short,
			Long: short + "\n" +
				"\n" +
				"Prints out information helpful for debugging the Pulumi CLI." +
				"\n" +
				"This includes information about:\n" +
				" - the CLI and how it was built\n" +
				" - which OS Pulumi was run from\n" +
				" - the current project\n" +
				" - the current stack\n" +
				" - the current backend\n",
			Args: cmdutil.MaximumNArgs(0),
			Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
				summary := getSummaryAbout()
				if jsonOut {
					return printJSON(summary)
				} else {
					summary.Print()
				}
				return nil
			},
			),
		}
	cmd.PersistentFlags().BoolVarP(
		&jsonOut, "json", "j", false, "Emit output as JSON")
	return cmd
}

type summaryAbout struct {
	// We use pointers here to allow the field to be nullable. When
	// constructing, we either fill in a field or add an error. We still
	// indicate that the field should be present when we serialize the struct.
	Plugins           []pluginAbout        `json:"plugins"`
	Host              *hostAbout           `json:"host"`
	Backend           *backendAbout        `json:"backend"`
	CurrentStack      *currentStackAbout   `json:"currentStack"`
	CLI               *cliAbout            `json:"cliAbout"`
	Runtime           *projectRuntimeAbout `json:"runtime"`
	Errors            []error              `json:"errors"`
	logMessage        string
	dependencyMessage string
}

func getSummaryAbout() summaryAbout {
	var err error
	cli := getCLIAbout()
	result := summaryAbout{
		CLI:        &cli,
		Errors:     []error{},
		logMessage: formatLogAbout(),
	}
	var plugins []pluginAbout
	if plugins, err = getPluginsAbout(); err != nil {
		err = errors.Wrap(err, "Failed to get information about the plugin")
		result.Errors = append(result.Errors, err)
	} else {
		result.Plugins = plugins
	}

	var host hostAbout
	if host, err = getHostAbout(); err != nil {
		err = errors.Wrap(err, "Failed to get information about the host")
		result.Errors = append(result.Errors, err)
	} else {
		result.Host = &host
	}

	var proj *workspace.Project
	var pwd string
	if proj, pwd, err = readProject(); err != nil {
		err = errors.Wrap(err, "Failed to read project for diagnosis")
		result.Errors = append(result.Errors, err)
	} else {
		var runtime projectRuntimeAbout
		if runtime, err = getProjectRuntimeAbout(proj); err != nil {
			err = errors.Wrap(err, "Failed to get diagnostic about the project runtime")
			result.Errors = append(result.Errors, err)
		} else {
			result.Runtime = &runtime
		}
		var depMsg string
		if depMsg, err = formatProgramDependenciesAbout(proj.Runtime.Name(), pwd); err != nil {
			err = errors.Wrap(err, "Failed to get diagnositc information about the puluimi program's plugins")
			result.Errors = append(result.Errors, err)
		} else {
			result.dependencyMessage = depMsg
		}
	}

	var backend backend.Backend
	backend, err = currentBackend(display.Options{Color: cmdutil.GetGlobalColorization()})
	if err != nil {
		err = errors.Wrapf(err, "Could not access the backend to give diagnosis.")
		result.Errors = append(result.Errors, err)
	} else {
		var stack currentStackAbout
		if stack, err = getCurrentStackAbout(backend); err != nil {
			err = errors.Wrap(err, "Failed to get diagnostic information about the current stack")
			result.Errors = append(result.Errors, err)
		} else {
			result.CurrentStack = &stack
		}

		tmp := getBackendAbout(backend)
		result.Backend = &tmp
	}
	return result
}

func (summary *summaryAbout) Print() {
	fmt.Println(summary.CLI)
	if summary.Plugins != nil {
		fmt.Println(formatPlugins(summary.Plugins))
	}
	if summary.Host != nil {
		fmt.Println(summary.Host)
	}
	if summary.Runtime != nil {
		fmt.Println(summary.Runtime)
	}
	if summary.CurrentStack != nil {
		fmt.Println(summary.CurrentStack)
	}
	if summary.Backend != nil {
		fmt.Println(summary.Backend)
	}
	if summary.dependencyMessage != "" {
		fmt.Println(summary.dependencyMessage)
	}
	fmt.Println(summary.logMessage)
	for _, err := range summary.Errors {
		cmdutil.Diag().Warningf(&diag.Diag{Message: err.Error()})
	}
}

type pluginAbout struct {
	Name    string          `json:"name"`
	Version *semver.Version `json:"version"`
}

func getPluginsAbout() ([]pluginAbout, error) {
	var pluginInfo []workspace.PluginInfo
	var err error
	pluginInfo, err = getProjectPluginsSilently()

	if err != nil {
		return nil, err
	}
	sort.Slice(pluginInfo, func(i, j int) bool {
		pi, pj := pluginInfo[i], pluginInfo[j]
		if pi.Name < pj.Name {
			return true
		} else if pi.Name == pj.Name && pi.Kind == pj.Kind &&
			(pi.Version == nil || (pj.Version != nil && pi.Version.GT(*pj.Version))) {
			return true
		}
		return false
	})

	var plugins = make([]pluginAbout, len(pluginInfo))
	for i, p := range pluginInfo {
		plugins[i] = pluginAbout{
			Name:    p.Name,
			Version: p.Version,
		}
	}
	return plugins, nil
}

func formatPlugins(p []pluginAbout) string {
	rows := []cmdutil.TableRow{}
	for _, plugin := range p {
		name := plugin.Name
		var version string
		if plugin.Version != nil {
			version = plugin.Version.String()
		} else {
			version = "unknown"
		}
		rows = append(rows, cmdutil.TableRow{
			Columns: []string{name, version},
		})
	}
	table := cmdutil.Table{
		Headers: []string{"NAME", "VERSION"},
		Rows:    rows,
	}
	return "Plugins\n" + table.String()
}

type hostAbout struct {
	Os      string `json:"os"`
	Version string `json:"version"`
	Arch    string `json:"arch"`
}

func getHostAbout() (hostAbout, error) {
	stats, err := host.Info()
	if err != nil {
		return hostAbout{}, err
	}
	return hostAbout{
		Os:      stats.Platform,
		Version: stats.PlatformVersion,
		Arch:    stats.KernelArch,
	}, nil
}

func (host hostAbout) String() string {
	return cmdutil.Table{
		Headers: []string{"Host", ""},
		Rows: simpleTableRows([][]string{
			[]string{"OS", host.Os},
			[]string{"Version", host.Version},
			[]string{"Arch", host.Arch},
		})}.String()
}

type backendAbout struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	User string `json:"user"`
}

func getBackendAbout(b backend.Backend) backendAbout {
	var err error
	var currentUser string
	currentUser, err = b.CurrentUser()
	if err != nil {
		currentUser = "Unknown"
	}
	return backendAbout{
		Name: b.Name(),
		URL:  b.URL(),
		User: currentUser,
	}
}

func (b backendAbout) String() string {
	return cmdutil.Table{
		Headers: []string{"Backend", ""},
		Rows: simpleTableRows([][]string{
			[]string{"Name", b.Name},
			[]string{"URL", b.URL},
			[]string{"User", b.User},
		}),
	}.String()
}

type currentStackAbout struct {
	Name       string       `json:"name"`
	Resources  []aboutState `json:"resources"`
	PendingOps []aboutState `json:"pendingOps"`
}

type aboutState struct {
	Type string `json:"type"`
	URN  string `json:"urn"`
}

func getCurrentStackAbout(b backend.Backend) (currentStackAbout, error) {
	context := commandContext()
	var stack backend.Stack
	var err error
	stack, err = state.CurrentStack(context, b)
	if err != nil {
		return currentStackAbout{}, err
	}
	if stack == nil {
		return currentStackAbout{}, errors.New("stack is nil")
	}
	if stack.Ref() == nil {
		return currentStackAbout{}, errors.New("Stack.Ref() is nil")
	}
	name := stack.Ref().String()
	var snapshot *deploy.Snapshot
	snapshot, err = stack.Snapshot(context)
	if err != nil {
		return currentStackAbout{}, err
	}
	var resources []*resource.State = snapshot.Resources
	var pendingOps []resource.Operation = snapshot.PendingOperations

	var aboutResources = make([]aboutState, len(resources))
	for i, r := range resources {
		aboutResources[i] = aboutState{
			Type: string(r.Type),
			URN:  string(r.URN),
		}
	}
	var aboutPending = make([]aboutState, len(pendingOps))
	for i, p := range pendingOps {
		aboutPending[i] = aboutState{
			Type: string(p.Type),
			URN:  string(p.Resource.URN),
		}
	}
	return currentStackAbout{
		Name:       name,
		Resources:  aboutResources,
		PendingOps: aboutPending,
	}, nil
}

func (current currentStackAbout) String() string {
	var resources string
	if len(current.Resources) == 0 {
		resources = fmt.Sprintf("Found no resources associated with %s\n", current.Name)
	} else {
		rows := make([]cmdutil.TableRow, len(current.Resources))
		for i, r := range current.Resources {
			rows[i] = cmdutil.TableRow{
				Columns: []string{string(r.Type), string(r.URN)},
			}
		}
		resources = cmdutil.Table{
			Headers: []string{"TYPE", "URN"},
			Rows:    rows,
		}.String() + "\n"
	}
	var pending string
	if len(current.PendingOps) == 0 {
		pending = fmt.Sprintf("Found no pending operations associated with %s\n", current.Name)
	} else {
		rows := make([]cmdutil.TableRow, len(current.PendingOps))
		for i, r := range current.PendingOps {
			rows[i] = cmdutil.TableRow{
				Columns: []string{string(r.Type), string(r.URN)},
			}
		}
		pending = cmdutil.Table{
			Headers: []string{"OPP TYPE", "URN"},
			Rows:    rows,
		}.String() + "\n"
	}
	return fmt.Sprintf("Current Stack: %s\n\n%s\n%s\n", current.Name, resources, pending)
}

func simpleTableRows(arr [][]string) []cmdutil.TableRow {
	rows := make([]cmdutil.TableRow, len(arr))
	for i, e := range arr {
		rows[i] = cmdutil.TableRow{
			Columns: e,
		}
	}
	return rows
}

// This does not have an associated struct. It also does not make sense to
// serialize.
func formatProgramDependenciesAbout(language, root string) (string, error) {
	var depInfo = ""
	switch language {
	case "nodejs":
		depInfo = "package.json"
	case "python":
		depInfo = "requirements.txt"
	case "go":
		depInfo = "go.mod"
	case "dotnet":
		return fmt.Sprintf("Please include the result of \"dotnet list package\""), nil
	default:
		return "", errors.New(fmt.Sprintf("Unknown Language: %s", language))
	}

	path := filepath.Join(root, depInfo)

	return fmt.Sprintf("Please include the contents of \"%s\" in your report.\n", path), nil
}

type cliAbout struct {
	Version    semver.Version `json:"version"`
	GoVersion  string         `json:"goVersion"`
	GoCompiler string         `json:"goCompiler"`
}

func getCLIAbout() cliAbout {
	return cliAbout{
		Version:    semver.MustParse(version.Version),
		GoVersion:  runtime.Version(),
		GoCompiler: runtime.Compiler,
	}
}

func (cli cliAbout) String() string {
	return cmdutil.Table{
		Headers: []string{"CLI", ""},
		Rows: simpleTableRows([][]string{
			[]string{"Version", cli.Version.String()},
			[]string{"Go Version", cli.GoVersion},
			[]string{"Go Compiler", cli.GoCompiler},
		}),
	}.String()
}

func formatLogAbout() string {
	logDir := flag.Lookup("log_dir")
	if logDir != nil && logDir.Value.String() != "" {
		return fmt.Sprintf("Pulumi locates its logs in %s\n", logDir)
	} else if runtime.GOOS != windows {
		return fmt.Sprintf("Pulumi locates its logs in $TEMPDIR by default\n")
	} else {
		// TODO: Find out
		return string(errors.New("I don't know where the logs are on windows\n").Error())
	}
}

type projectRuntimeAbout struct {
	Language   string `json:"language"`
	Executable string `json:"executable"`
	Version    string `json:"version"`
}

func getProjectRuntimeAbout(proj *workspace.Project) (projectRuntimeAbout, error) {
	var ex, version string
	var err error
	language := proj.Runtime.Name()
	switch language {
	case "nodejs":
		ex, err = executable.FindExecutable("node")
		if err != nil {
			return projectRuntimeAbout{}, errors.Wrap(err, "Could not find node executable")
		}
		cmd := exec.Command(ex, "--version")
		if out, err := cmd.Output(); err != nil {
			return projectRuntimeAbout{}, errors.Wrap(err, "Failed to get node version")
		} else {
			version = string(out)
		}
	case "python":
		var cmd *exec.Cmd
		// if CommandPath has an error, then so will Command. The error can
		// therefore be ignored as redundant.
		ex, _, _ = python.CommandPath()
		cmd, err = python.Command("--version")
		if err != nil {
			return projectRuntimeAbout{}, err
		}
		if out, err := cmd.Output(); err != nil {
			return projectRuntimeAbout{}, errors.Wrap(err, "Failed to get python version")
		} else {
			version = "v" + strings.TrimPrefix(string(out), "Python ")
		}
	case "go":
		ex, err = executable.FindExecutable("go")
		if err != nil {
			return projectRuntimeAbout{}, errors.Wrap(err, "Could not find python executable")
		}
		cmd := exec.Command(ex, "version")
		if out, err := cmd.Output(); err != nil {
			return projectRuntimeAbout{}, errors.Wrap(err, "Failed to get go version")
		} else {
			version = "v" + strings.TrimPrefix(string(out), "go version go")
		}
	case "dotnet":
		ex, err = executable.FindExecutable("dotnet")
		if err != nil {
			return projectRuntimeAbout{}, errors.Wrap(err, "Could not find dotnet executable")
		}
		cmd := exec.Command(ex, "--version")
		if out, err := cmd.Output(); err != nil {
			return projectRuntimeAbout{}, errors.Wrap(err, "Failed to get dotnet version")
		} else {
			version = "v" + string(out)
		}
	default:
		return projectRuntimeAbout{}, errors.New(fmt.Sprintf("Unknown Language: %s", language))
	}
	version = strings.TrimSpace(version)
	return projectRuntimeAbout{
		Language:   language,
		Executable: ex,
		Version:    version,
	}, nil
}

func (runtime projectRuntimeAbout) String() string {
	return fmt.Sprintf("This project is a %s project (%s %s)\n", runtime.Language, runtime.Executable, runtime.Version)
}

// This is necessary because dotnet invokes build during the call to
// getProjectPlugins.
func getProjectPluginsSilently() ([]workspace.PluginInfo, error) {
	_, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	stdout := os.Stdout
	defer func() { os.Stdout = stdout }()
	os.Stdout = w

	return getProjectPlugins()
}
