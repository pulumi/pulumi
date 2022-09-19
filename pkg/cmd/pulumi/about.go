// Copyright 2016-2021, Pulumi Corporation.
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
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/blang/semver"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/spf13/cobra"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	enginerpc "github.com/pulumi/pulumi/sdk/v3/proto/go/engine"

	"github.com/pulumi/pulumi/pkg/v3/engineInterface"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newAboutCmd() *cobra.Command {
	var jsonOut bool
	var transitiveDependencies bool
	var stack string
	short := "Print information about the Pulumi environment."
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
				ctx := commandContext()
				engine, err := engineInterface.Start(ctx)
				if err != nil {
					return err
				}

				req := &enginerpc.AboutRequest{
					TransitiveDependencies: transitiveDependencies,
					Stack:                  stack,
				}
				res, err := engine.About(ctx, req)
				if err != nil {
					return err
				}

				summary := getSummaryAbout(res)
				if jsonOut {
					return printJSON(summary)
				}
				summary.Print()
				return nil
			}),
		}
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
	LogMessage    string                   `json:"-"`
}

func getSummaryAbout(res *enginerpc.AboutResponse) summaryAbout {
	var err error
	cli := getCLIAbout(res)
	result := summaryAbout{
		CLI:           &cli,
		ErrorMessages: []string{},
		LogMessage:    formatLogAbout(),
	}

	result.ErrorMessages = res.Errors
	addError := func(err error, message string) {
		err = fmt.Errorf("%s: %w", message, err)
		result.ErrorMessages = append(result.ErrorMessages, err.Error())
	}

	var host hostAbout
	if host, err = getHostAbout(); err != nil {
		addError(err, "Failed to get information about the host")
	} else {
		result.Host = &host
	}

	result.Plugins = getPluginsAbout(res.Plugins)
	result.Runtime = getLanguageAbout(res)
	result.Dependencies = getLanguageDependencies(res.Dependencies)
	result.CurrentStack = getCurrentStackAbout(res.Stack)
	result.Backend = getBackendAbout(res.Backend)

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
	for _, err := range summary.ErrorMessages {
		cmdutil.Diag().Warningf(&diag.Diag{Message: err})
	}
}

type pluginAbout struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func getPluginsAbout(plugins []*pulumirpc.PluginDependency) []pluginAbout {
	var result = make([]pluginAbout, len(plugins))
	for i, p := range plugins {
		result[i] = pluginAbout{
			Name:    p.Name,
			Version: p.Version,
		}
	}
	return result
}

func getLanguageAbout(response *enginerpc.AboutResponse) *projectRuntimeAbout {
	if response.Language == nil {
		return nil
	}

	return &projectRuntimeAbout{
		other:      response.Language.Metadata,
		Language:   response.Runtime,
		Executable: response.Language.Executable,
		Version:    response.Language.Version,
	}
}

func getLanguageDependencies(response []*pulumirpc.DependencyInfo) []programDependencyAbout {
	result := make([]programDependencyAbout, len(response))
	for _, dep := range response {
		result = append(result, programDependencyAbout{
			Name:    dep.Name,
			Version: dep.Version,
		})
	}
	return result
}

func formatPlugins(p []pluginAbout) string {
	rows := []cmdutil.TableRow{}
	for _, plugin := range p {
		name := plugin.Name
		var version string
		if plugin.Version != "" {
			version = plugin.Version
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
			{"OS", host.Os},
			{"Version", host.Version},
			{"Arch", host.Arch},
		})}.String()
}

type backendAbout struct {
	Name          string   `json:"name"`
	URL           string   `json:"url"`
	User          string   `json:"user"`
	Organizations []string `json:"organizations"`
}

func getBackendAbout(b *enginerpc.AboutBackend) *backendAbout {
	if b == nil {
		return nil
	}

	currentUser := b.User
	if currentUser == "" {
		currentUser = "Unknown"
	}

	return &backendAbout{
		Name:          b.Name,
		URL:           b.Url,
		User:          currentUser,
		Organizations: b.Organizations,
	}
}

func (b backendAbout) String() string {
	return cmdutil.Table{
		Headers: []string{"Backend", ""},
		Rows: simpleTableRows([][]string{
			{"Name", b.Name},
			{"URL", b.URL},
			{"User", b.User},
			{"Organizations", strings.Join(b.Organizations, ", ")},
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

func getCurrentStackAbout(currentStack *enginerpc.AboutStack) *currentStackAbout {
	if currentStack == nil {
		return nil
	}

	var aboutResources = make([]aboutState, len(currentStack.Resources))
	for i, r := range currentStack.Resources {
		aboutResources[i] = aboutState{
			Type: r.Type,
			URN:  r.Urn,
		}
	}
	var aboutPending = make([]aboutState, len(currentStack.PendingOperations))
	for i, p := range currentStack.PendingOperations {
		aboutPending[i] = aboutState{
			Type: p.Type,
			URN:  p.Urn,
		}
	}
	return &currentStackAbout{
		Name:       currentStack.Name,
		Resources:  aboutResources,
		PendingOps: aboutPending,
	}
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
				Columns: []string{r.Type, r.URN},
			}
		}
		pending = cmdutil.Table{
			Headers: []string{"OPP TYPE", "URN"},
			Rows:    rows,
		}.String() + "\n"
	}
	return fmt.Sprintf("Current Stack: %s\n\n%s\n%s", current.Name, resources, pending)
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
	return "Dependencies:\n" + cmdutil.Table{
		Headers: []string{"NAME", "VERSION"},
		Rows:    rows,
	}.String()
}

type cliAbout struct {
	Version    string `json:"version"`
	GoVersion  string `json:"goVersion"`
	GoCompiler string `json:"goCompiler"`
}

func getCLIAbout(res *enginerpc.AboutResponse) cliAbout {
	var engineVersion string
	// Version is not supplied in test builds.
	ver, err := semver.ParseTolerant(res.Version)
	if err == nil {
		// To get semver formatting when possible
		engineVersion = ver.String()
	} else {
		engineVersion = res.Version
	}
	return cliAbout{
		Version:    engineVersion,
		GoVersion:  res.GoVersion,
		GoCompiler: res.GoCompiler,
	}
}

func (cli cliAbout) String() string {
	return cmdutil.Table{
		Headers: []string{"CLI", ""},
		Rows: simpleTableRows([][]string{
			{"Version", cli.Version},
			{"Go Version", cli.GoVersion},
			{"Go Compiler", cli.GoCompiler},
		}),
	}.String()
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
	var params []string

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
