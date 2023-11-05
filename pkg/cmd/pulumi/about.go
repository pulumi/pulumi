// Copyright 2016-2023, Pulumi Corporation.
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
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/blang/semver"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newAboutCmd() *cobra.Command {
	var jsonOut bool
	var transitiveDependencies bool
	var stack string
	short := "Print information about the Pulumi environment."
	cmd := &cobra.Command{
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
			ctx := commandContext()
			summary := getSummaryAbout(ctx, transitiveDependencies, stack)
			if jsonOut {
				return printJSON(summary)
			}
			summary.Print()
			return nil
		}),
	}

	cmd.AddCommand(newAboutEnvCmd())

	cmd.PersistentFlags().BoolVarP(
		&jsonOut, "json", "j", false, "Emit output as JSON")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to get info on. Defaults to the current stack")
	cmd.PersistentFlags().BoolVarP(
		&transitiveDependencies, "transitive", "t", false, "Include transitive dependencies")

	return cmd
}

type summaryAbout struct {
	// We use pointers here to allow the field to be nullable. When
	// constructing, we either fill in a field or add an error. We still
	// indicate that the field should be present when we serialize the struct.
	Plugins       []pluginAbout            `json:"plugins"`
	Host          *hostAbout               `json:"host"`
	Backend       *backendAbout            `json:"backend"`
	CurrentStack  *currentStackAbout       `json:"currentStack"`
	CLI           *cliAbout                `json:"cliAbout"`
	Runtime       *projectRuntimeAbout     `json:"runtime"`
	Dependencies  []programDependencyAbout `json:"dependencies"`
	ErrorMessages []string                 `json:"errors"`
	Errors        []error                  `json:"-"`
	LogMessage    string                   `json:"-"`
}

func getSummaryAbout(ctx context.Context, transitiveDependencies bool, selectedStack string) summaryAbout {
	var err error
	cli := getCLIAbout()
	result := summaryAbout{
		CLI:           &cli,
		Errors:        []error{},
		ErrorMessages: []string{},
		LogMessage:    formatLogAbout(),
	}
	addError := func(err error, message string) {
		err = fmt.Errorf("%s: %w", message, err)
		result.ErrorMessages = append(result.ErrorMessages, err.Error())
		result.Errors = append(result.Errors, err)
	}

	var host hostAbout
	if host, err = getHostAbout(); err != nil {
		addError(err, "Failed to get information about the host")
	} else {
		result.Host = &host
	}

	var proj *workspace.Project
	var pwd string
	if proj, pwd, err = readProject(); err != nil {
		addError(err, "Failed to read project")
	} else {
		projinfo := &engine.Projinfo{Proj: proj, Root: pwd}
		pwd, program, pluginContext, err := engine.ProjectInfoContext(
			projinfo, nil, cmdutil.Diag(), cmdutil.Diag(), false, nil, nil)
		if err != nil {
			addError(err, "Failed to create plugin context")
		} else {
			defer pluginContext.Close()

			// Only try to get project plugins if we managed to read a project
			if plugins, err := getPluginsAbout(pluginContext, proj, pwd, program); err != nil {
				addError(err, "Failed to get information about the plugin")
			} else {
				result.Plugins = plugins
			}

			lang, err := pluginContext.Host.LanguageRuntime(projinfo.Root, pwd, proj.Runtime.Name(), proj.Runtime.Options())
			if err != nil {
				addError(err, fmt.Sprintf("Failed to load language plugin %s", proj.Runtime.Name()))
			} else {
				aboutResponse, err := lang.About()
				if err != nil {
					addError(err, "Failed to get information about the project runtime")
				} else {
					result.Runtime = &projectRuntimeAbout{
						other:      aboutResponse.Metadata,
						Language:   proj.Runtime.Name(),
						Executable: aboutResponse.Executable,
						Version:    aboutResponse.Version,
					}
				}

				progInfo := plugin.ProgInfo{Proj: proj, Pwd: pwd, Program: program}
				deps, err := lang.GetProgramDependencies(progInfo, transitiveDependencies)
				if err != nil {
					addError(err, "Failed to get information about the Pulumi program's dependencies")
				} else {
					result.Dependencies = make([]programDependencyAbout, len(deps))
					for i, dep := range deps {
						result.Dependencies[i] = programDependencyAbout{
							Name:    dep.Name,
							Version: dep.Version.String(),
						}
					}
				}
			}
		}
	}

	var backend backend.Backend
	backend, err = nonInteractiveCurrentBackend(ctx, proj)
	if err != nil {
		addError(err, "Could not access the backend")
	} else if backend != nil {
		var stack currentStackAbout
		if stack, err = getCurrentStackAbout(ctx, backend, selectedStack); err != nil {
			addError(err, "Failed to get information about the current stack")
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
	if summary.Dependencies != nil {
		fmt.Println(formatProgramDependenciesAbout(summary.Dependencies))
	}
	fmt.Println(summary.LogMessage)
	for _, err := range summary.Errors {
		cmdutil.Diag().Warningf(&diag.Diag{Message: err.Error()})
	}
}

type pluginAbout struct {
	Name    string          `json:"name"`
	Version *semver.Version `json:"version"`
}

func getPluginsAbout(ctx *plugin.Context, proj *workspace.Project, pwd, main string) ([]pluginAbout, error) {
	pluginSpec, err := getProjectPluginsSilently(ctx, proj, pwd, main)
	if err != nil {
		return nil, err
	}
	sort.Slice(pluginSpec, func(i, j int) bool {
		pi, pj := pluginSpec[i], pluginSpec[j]
		if pi.Name < pj.Name {
			return true
		} else if pi.Name == pj.Name && pi.Kind == pj.Kind &&
			(pi.Version == nil || (pj.Version != nil && pi.Version.GT(*pj.Version))) {
			return true
		}
		return false
	})

	plugins := make([]pluginAbout, len(pluginSpec))
	for i, p := range pluginSpec {
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
	return "Plugins\n" + renderTable(table, nil)
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
	return renderTable(cmdutil.Table{
		Headers: []string{"Host", ""},
		Rows: simpleTableRows([][]string{
			{"OS", host.Os},
			{"Version", host.Version},
			{"Arch", host.Arch},
		}),
	}, nil)
}

type backendAbout struct {
	Name             string                      `json:"name"`
	URL              string                      `json:"url"`
	User             string                      `json:"user"`
	Organizations    []string                    `json:"organizations"`
	TokenInformation *workspace.TokenInformation `json:"tokenInformation,omitempty"`
}

func getBackendAbout(b backend.Backend) backendAbout {
	currentUser, currentOrgs, tokenInfo, err := b.CurrentUser()
	if err != nil {
		currentUser = "Unknown"
	}
	return backendAbout{
		Name:             b.Name(),
		URL:              b.URL(),
		User:             currentUser,
		Organizations:    currentOrgs,
		TokenInformation: tokenInfo,
	}
}

func (b backendAbout) String() string {
	rows := [][]string{
		{"Name", b.Name},
		{"URL", b.URL},
		{"User", b.User},
		{"Organizations", strings.Join(b.Organizations, ", ")},
	}

	if b.TokenInformation != nil {
		var tokenType string
		if b.TokenInformation.Team != "" {
			tokenType = fmt.Sprintf("team: %s", b.TokenInformation.Team)
		} else {
			contract.Assertf(b.TokenInformation.Organization != "", "token must have an organization or team")
			tokenType = fmt.Sprintf("organization: %s", b.TokenInformation.Organization)
		}
		rows = append(rows, []string{"Token type", tokenType})
		rows = append(rows, []string{"Token type", b.TokenInformation.Name})
	} else {
		rows = append(rows, []string{"Token type", "personal"})
	}

	return renderTable(cmdutil.Table{
		Headers: []string{"Backend", ""},
		Rows:    simpleTableRows(rows),
	}, nil)
}

type currentStackAbout struct {
	Name               string       `json:"name"`
	FullyQualifiedName string       `json:"fullyQualifiedName"`
	Resources          []aboutState `json:"resources"`
	PendingOps         []aboutState `json:"pendingOps"`
}

type aboutState struct {
	Type string `json:"type"`
	URN  string `json:"urn"`
}

func getCurrentStackAbout(ctx context.Context, b backend.Backend, selectedStack string) (currentStackAbout, error) {
	var s backend.Stack
	var err error
	if selectedStack == "" {
		s, err = state.CurrentStack(ctx, b)
	} else {
		var ref backend.StackReference
		ref, err = b.ParseStackReference(selectedStack)
		if err != nil {
			return currentStackAbout{}, err
		}
		s, err = b.GetStack(ctx, ref)
	}
	if err != nil {
		return currentStackAbout{}, err
	}
	if s == nil {
		return currentStackAbout{}, errors.New("No current stack")
	}

	name := s.Ref().String()
	var snapshot *deploy.Snapshot
	snapshot, err = s.Snapshot(ctx, stack.DefaultSecretsProvider)
	if err != nil {
		return currentStackAbout{}, err
	} else if snapshot == nil {
		return currentStackAbout{}, errors.New("No current snapshot")
	}
	var resources []*resource.State = snapshot.Resources
	var pendingOps []resource.Operation = snapshot.PendingOperations

	aboutResources := make([]aboutState, len(resources))
	for i, r := range resources {
		aboutResources[i] = aboutState{
			Type: string(r.Type),
			URN:  string(r.URN),
		}
	}
	aboutPending := make([]aboutState, len(pendingOps))
	for i, p := range pendingOps {
		aboutPending[i] = aboutState{
			Type: string(p.Type),
			URN:  string(p.Resource.URN),
		}
	}
	return currentStackAbout{
		Name:               name,
		FullyQualifiedName: s.Ref().FullyQualifiedName().String(),
		Resources:          aboutResources,
		PendingOps:         aboutPending,
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
				Columns: []string{r.Type, r.URN},
			}
		}
		resources = renderTable(cmdutil.Table{
			Headers: []string{"TYPE", "URN"},
			Rows:    rows,
		}, nil) + "\n"
	}
	var pending string
	if len(current.PendingOps) == 0 {
		pending = fmt.Sprintf("Found no pending operations associated with %s\n", current.Name)
	} else {
		rows := make([]cmdutil.TableRow, len(current.PendingOps))
		for i, r := range current.PendingOps {
			rows[i] = cmdutil.TableRow{
				Columns: []string{r.Type, r.URN},
			}
		}
		pending = renderTable(cmdutil.Table{
			Headers: []string{"OPP TYPE", "URN"},
			Rows:    rows,
		}, nil) + "\n"
	}
	stackName := current.Name
	if current.FullyQualifiedName != "" {
		stackName = current.FullyQualifiedName
	}
	return fmt.Sprintf("Current Stack: %s\n\n%s\n%s", stackName, resources, pending)
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

type programDependencyAbout struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func formatProgramDependenciesAbout(deps []programDependencyAbout) string {
	if len(deps) == 0 {
		return "No dependencies found\n"
	}
	rows := make([]cmdutil.TableRow, len(deps))
	for i, v := range deps {
		rows[i] = cmdutil.TableRow{
			Columns: []string{v.Name, v.Version},
		}
	}
	return "Dependencies:\n" + renderTable(cmdutil.Table{
		Headers: []string{"NAME", "VERSION"},
		Rows:    rows,
	}, nil)
}

type cliAbout struct {
	Version    string `json:"version"`
	GoVersion  string `json:"goVersion"`
	GoCompiler string `json:"goCompiler"`
}

func getCLIAbout() cliAbout {
	var ver semver.Version
	var err error
	var cliVersion string
	// Version is not supplied in test builds.
	ver, err = semver.ParseTolerant(version.Version)
	if err == nil {
		// To get semver formatting when possible
		cliVersion = ver.String()
	} else {
		cliVersion = version.Version
	}
	return cliAbout{
		Version:    cliVersion,
		GoVersion:  runtime.Version(),
		GoCompiler: runtime.Compiler,
	}
}

func (cli cliAbout) String() string {
	return renderTable(cmdutil.Table{
		Headers: []string{"CLI", ""},
		Rows: simpleTableRows([][]string{
			{"Version", cli.Version},
			{"Go Version", cli.GoVersion},
			{"Go Compiler", cli.GoCompiler},
		}),
	}, nil)
}

func formatLogAbout() string {
	logDir := flag.Lookup("log_dir")
	if logDir != nil && logDir.Value.String() != "" {
		return fmt.Sprintf("Pulumi locates its logs in %s", logDir)
	}
	return fmt.Sprintf("Pulumi locates its logs in %s by default", os.TempDir())
}

type projectRuntimeAbout struct {
	Language   string
	Executable string
	Version    string
	other      map[string]string
}

func (runtime projectRuntimeAbout) MarshalJSON() ([]byte, error) {
	m := make(map[string]string, len(runtime.other)+3)
	assignIf := func(k, v string) {
		if v != "" {
			m[k] = v
		}
	}
	for k, v := range runtime.other {
		assignIf(k, v)
	}

	assignIf("language", runtime.Language)
	assignIf("executable", runtime.Executable)
	assignIf("version", runtime.Version)
	return json.Marshal(m)
}

func (runtime projectRuntimeAbout) String() string {
	params := slice.Prealloc[string](len(runtime.other))

	if r := runtime.Executable; r != "" {
		params = append(params, fmt.Sprintf("executable='%s'", r))
	}
	if v := runtime.Version; v != "" {
		params = append(params, fmt.Sprintf("version='%s'", v))
	}
	for k, v := range runtime.other {
		params = append(params, fmt.Sprintf("%s='%s'", k, v))
	}
	paramString := ""
	if len(params) > 0 {
		paramString = fmt.Sprintf(": %s", strings.Join(params, " "))
	}
	return fmt.Sprintf("This project is written in %s%s\n",
		runtime.Language, paramString)
}

// This is necessary because dotnet invokes build during the call to
// getProjectPlugins.
func getProjectPluginsSilently(
	ctx *plugin.Context, proj *workspace.Project, pwd, main string,
) ([]workspace.PluginSpec, error) {
	_, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	stdout := os.Stdout
	defer func() { os.Stdout = stdout }()
	os.Stdout = w

	return plugin.GetRequiredPlugins(ctx.Host, ctx.Root, plugin.ProgInfo{
		Proj:    proj,
		Pwd:     pwd,
		Program: main,
	}, plugin.AllPlugins)
}
