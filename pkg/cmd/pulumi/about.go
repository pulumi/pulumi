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
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/goversion"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/python"
)

const (
	langPython = "python"
	langNodejs = "nodejs"
	langDotnet = "dotnet"
	langGo     = "go"
)

func newAboutCmd() *cobra.Command {
	var jsonOut bool
	var transitive Dependencies bool
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
				summary := getSummaryAbout(transitiveDependencies)
				if jsonOut {
					return printJSON(summary)
				}
				summary.Print()
				return nil
			},
			),
		}
	cmd.PersistentFlags().BoolVarP(
		&jsonOut, "json", "j", false, "Emit output as JSON")
	cmd.PersistentFlags().BoolVarP(
		&transitiveDependencies, "transitive", "t", false, "Include transitive dependencies")

	return cmd
}

type summaryAbout struct {
	// We use pointers here to allow the field to be nullable. When
	// constructing, we either fill in a field or add an error. We still
	// indicate that the field should be present when we serialize the struct.
	Plugins       []pluginAbout             `json:"plugins"`
	Host          *hostAbout                `json:"host"`
	Backend       *backendAbout             `json:"backend"`
	CurrentStack  *currentStackAbout        `json:"currentStack"`
	CLI           *cliAbout                 `json:"cliAbout"`
	Runtime       *projectRuntimeAbout      `json:"runtime"`
	Dependencies  []programDependencieAbout `json:"dependencies"`
	ErrorMessages []string                  `json:"errors"`
	Errors        []error                   `json:"-"`
	LogMessage    string                    `json:"-"`
}

func getSummaryAbout(transitiveDependencies bool) summaryAbout {
	var err error
	cli := getCLIAbout()
	result := summaryAbout{
		CLI:           &cli,
		Errors:        []error{},
		ErrorMessages: []string{},
		LogMessage:    formatLogAbout(),
	}
	var plugins []pluginAbout
	addError := func(err error, message string) {
		err = errors.Wrap(err, message)
		result.ErrorMessages = append(result.ErrorMessages, err.Error())
		result.Errors = append(result.Errors, err)
	}
	if plugins, err = getPluginsAbout(); err != nil {
		addError(err, "Failed to get information about the plugin")
	} else {
		result.Plugins = plugins
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
		var runtime projectRuntimeAbout
		if runtime, err = getProjectRuntimeAbout(proj); err != nil {
			addError(err, "Failed to get information about the project runtime")
		} else {
			result.Runtime = &runtime
		}
		if deps, err := getProgramDependenciesAbout(proj, pwd, transitiveDependencies); err != nil {
			addError(err, "Failed to get information about the Puluimi program's plugins")
		} else {
			result.Dependencies = deps
		}
	}

	var backend backend.Backend
	backend, err = currentBackend(display.Options{Color: cmdutil.GetGlobalColorization()})
	if err != nil {
		addError(err, "Could not access the backend")
	} else {
		var stack currentStackAbout
		if stack, err = getCurrentStackAbout(backend); err != nil {
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
			{"OS", host.Os},
			{"Version", host.Version},
			{"Arch", host.Arch},
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
			{"Name", b.Name},
			{"URL", b.URL},
			{"User", b.User},
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
		return currentStackAbout{}, errors.New("No current stack")
	}
	name := stack.Ref().String()
	var snapshot *deploy.Snapshot
	snapshot, err = stack.Snapshot(context)
	if err != nil {
		return currentStackAbout{}, err
	} else if snapshot == nil {
		return currentStackAbout{}, errors.New("No current snapshot")
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

type programDependencieAbout struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type goModule struct {
	Path     string
	Version  string
	Time     string
	Indirect bool
	Dir      string
	GoMod    string
	Main     bool
}

func getGoProgramDependencies(transitive bool) ([]programDependencieAbout, error) {
	// go list -m ...
	//
	//Go has a --json flag, but it doesn't emit a single json object (which
	//makes it invalid json).
	ex, err := executable.FindExecutable("go")
	if err != nil {
		return nil, err
	}
	if err := goversion.CheckMinimumGoVersion(ex); err != nil {
		return nil, err
	}
	cmdArgs := []string{"list", "--json", "-m", "..."}
	cmd := exec.Command(ex, cmdArgs...)
	var out []byte
	if out, err = cmd.Output(); err != nil {
		return nil, errors.Wrap(err, "Failed to get modules")
	}

	dec := json.NewDecoder(bytes.NewReader(out))
	parsed := []goModule{}
	for {
		var m goModule
		if err := dec.Decode(&m); err != nil {
			if err == io.EOF {
				break
			}
			return nil, errors.Wrapf(err, "Failed to parse \"%s %s\" output", ex, strings.Join(cmdArgs, " "))
		}
		parsed = append(parsed, m)

	}

	result := []programDependencieAbout{}
	for _, d := range parsed {
		if (!d.Indirect || transitive) && !d.Main {
			datum := programDependencieAbout{
				Name:    d.Path,
				Version: d.Version,
			}
			result = append(result, datum)
		}
	}
	return result, nil
}

// Calls a python command as pulumi would. This means we need to accommodate for
// a virtual environment if it exists.
func callPythonCommand(proj *workspace.Project, root string, args ...string) (string, error) {
	if proj == nil {
		return "", errors.New("Project must not be nil")
	}
	options := proj.Runtime.Options()
	if options == nil {
		return callPythonCommandNoEnviroment(args...)
	}
	virtualEnv, exists := options["virtualenv"]
	if !exists {
		return callPythonCommandNoEnviroment(args...)
	}
	virtualEnvPath := virtualEnv.(string)
	// We now know that a virtual environment exists.
	if virtualEnv != "" && !filepath.IsAbs(virtualEnvPath) {
		virtualEnvPath = filepath.Join(root, virtualEnvPath)
	}
	cmd := python.VirtualEnvCommand(virtualEnvPath, "python", args...)
	result, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(result), nil
}

// Call a python command in a runtime agnostic way. Call python from the path.
// Do not use a virtual environment.
func callPythonCommandNoEnviroment(args ...string) (string, error) {
	cmd, err := python.Command(args...)
	if err != nil {
		return "", err
	}

	var result []byte
	if result, err = cmd.Output(); err != nil {
		return "", err
	}
	return string(result), nil
}

func getPythonProgramDependencies(proj *workspace.Project, rootDir string,
	transitive bool) ([]programDependencieAbout, error) {
	cmdArgs := []string{"-m", "pip", "list", "--format=json"}
	if !transitive {
		cmdArgs = append(cmdArgs, "--not-required")

	}
	out, err := callPythonCommand(proj, rootDir, cmdArgs...)
	if err != nil {
		return nil, err
	}
	var result []programDependencieAbout
	err = json.Unmarshal([]byte(out), &result)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse \"python %s\" result", strings.Join(cmdArgs, " "))
	}

	return result, nil
}

func getDotNetProgramDependencies(proj *workspace.Project, transitive bool) ([]programDependencieAbout, error) {
	// dotnet list package

	var err error
	options := proj.Runtime.Options()
	if options != nil {
		if _, exists := options["binary"]; exists {
			return nil, errors.New("Could not get dependencies because pulumi specifies a binary")
		}
	}
	var ex string
	var out []byte
	ex, err = executable.FindExecutable("dotnet")
	if err != nil {
		return nil, err
	}
	cmdArgs := []string{"list", "package"}
	if transitive {
		cmdArgs = append(cmdArgs, "--include-transitive")
	}
	cmd := exec.Command(ex, cmdArgs...)
	if out, err = cmd.Output(); err != nil {
		return nil, errors.Wrapf(err, "Failed to call \"%s\"", ex)
	}
	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	var packages []programDependencieAbout

	for _, p := range lines {
		p := strings.TrimSpace(p)
		if strings.HasPrefix(p, ">") {
			p = strings.TrimPrefix(p, "> ")
			segments := strings.Split(p, " ")
			var nameRequiredVersion []string
			for _, s := range segments {
				if s != "" {
					nameRequiredVersion = append(nameRequiredVersion, s)
				}
			}
			var version int
			if len(nameRequiredVersion) == 3 {
				// Top level package => name required version
				version = 2
			} else if len(nameRequiredVersion) == 2 {
				// Transitive package => name version
				version = 1
			} else {
				return nil, errors.Errorf("Failed to parse \"%s\"", p)
			}
			packages = append(packages, programDependencieAbout{
				Name:    nameRequiredVersion[0],
				Version: nameRequiredVersion[version],
			})
		}
	}
	return packages, nil
}

func getNodeProgramDependencies(rootDir string, transitive bool) ([]programDependencieAbout, error) {
	// Neither "yarn list" or "npm ls" can describe what packages are required
	// (direct dependencies). Only what packages they have installed (transitive
	// dependencies). This means that to accurately report only direct
	// dependencies, we need to also parse "package.json" and intersect it with
	// reported dependencies.
	var err error
	yarnFile := filepath.Join(rootDir, "yarn.lock")
	npmFile := filepath.Join(rootDir, "package-lock.json")
	packageFile := filepath.Join(rootDir, "package.json")
	var result []programDependencieAbout
	var ex string
	var out []byte
	if _, err = os.Stat(yarnFile); err == nil {
		ex, err = executable.FindExecutable("yarn")
		if err != nil {
			return nil, errors.Wrapf(err, "Found %s but not yarn", yarnFile)
		}
		cmdArgs := []string{"list", "--json"}
		cmd := exec.Command(ex, cmdArgs...)
		out, err = cmd.Output()
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to run \"%s %s\"", ex, strings.Join(cmdArgs, " "))
		}
		var output interface{}
		if err = json.Unmarshal(out, &output); err != nil {
			return nil, errors.Wrapf(err, "Failed to parse\"%s %s\"", ex, strings.Join(cmdArgs, " "))
		}
		var data interface{}
		if data = output.(map[string]interface{})["data"]; data == nil {
			return nil, errors.New("Expected \"data\" in yarn json")
		}
		var trees interface{}
		if trees = data.(map[string]interface{})["trees"]; trees == nil {
			return nil, errors.New("Expected \"trees\" in yarn json")
		}
		var leafs []interface{}
		var ok bool
		if leafs, ok = trees.([]interface{}); !ok {
			return nil, errors.New("yarn list (trees) has an unexpected form")
		}
		result = make([]programDependencieAbout, len(leafs))
		// Has the form name@version
		splitName := func(index int, nameVersion string) (string, string, error) {
			if nameVersion == "" {
				return "", "", errors.Errorf("Expected \"name\" in dependency %d", index)
			}
			split := strings.LastIndex(nameVersion, "@")
			if split == -1 {
				return "", "", errors.Errorf("Failed to parse name and version from %s", nameVersion)
			}
			return nameVersion[:split], nameVersion[split+1:], nil
		}
		for i, v := range leafs {
			vMap, ok := v.(map[string]interface{})
			if !ok {
				return nil, errors.New("package had an unexpected form")
			}
			nameVersion := vMap["name"]

			name, version, err := splitName(i, nameVersion.(string))
			if err != nil {
				return nil, err
			}
			result[i] = programDependencieAbout{
				Name:    name,
				Version: version,
			}
			children := vMap["children"]
			for _, c := range children.([]interface{}) {
				name := c.(map[string]interface{})["name"].(string)
				name, version, err := splitName(i, name)
				if err != nil {
					return nil, err
				}
				result = append(result, programDependencieAbout{
					Name:    name,
					Version: version,
				})
			}

		}
	} else if _, err = os.Stat(npmFile); err == nil {
		ex, err = executable.FindExecutable("npm")
		if err != nil {
			return nil, errors.Wrapf(err, "Found %s but not npm", npmFile)
		}
		cmdArgs := []string{"ls", "--json", "--depth=0"}
		cmd := exec.Command(ex, cmdArgs...)
		out, err = cmd.Output()
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to run \"%s %s\"", ex, strings.Join(cmdArgs, " "))
		}
		var output interface{}
		if err = json.Unmarshal(out, &output); err != nil {
			return nil, errors.Wrapf(err, "Failed to parse \"%s %s\"", ex, strings.Join(cmdArgs, " "))
		}
		outputMap := output.(map[string]interface{})
		if outputMap == nil {
			return nil, errors.Errorf("Failed to parse \"%s %s\" output", ex, strings.Join(cmdArgs, " "))
		}
		dependencies := outputMap["dependencies"].(map[string]interface{})
		if dependencies == nil {
			return nil, errors.Errorf("Failed to find \"dependencies\" in \"%s %s\" output",
				ex, strings.Join(cmdArgs, " "))
		}
		result = make([]programDependencieAbout, len(dependencies))
		var i int
		for k, v := range dependencies {
			value := v.(map[string]interface{})
			if value == nil {
				return nil, errors.Errorf("Failed to convert dependency %s to map", v)
			}
			result[i].Name = k
			result[i].Version = value["version"].(string)
			i++
		}
	} else if os.IsNotExist(err) {
		return nil, errors.Errorf("Could not find either %s or %s", yarnFile, npmFile)
	} else {
		return nil, errors.Wrap(err, "Could not get node dependency data")
	}
	if !transitive {
		var packageJSON interface{}
		file, err := ioutil.ReadFile(packageFile)
		if os.IsNotExist(err) {
			return nil, errors.Errorf("Could not find %s. "+
				"Please report this and run \"pulumi about --transitive\" to get a list of used packages",
				packageFile)
		} else if err != nil {
			return nil, errors.Wrapf(err, "Could not read %s", packageFile)
		}
		err = json.Unmarshal(file, &packageJSON)
		if err != nil {
			return nil, errors.Wrapf(err, "Could not parse %s", packageFile)
		}
		dependenciesInterface, exists := packageJSON.(map[string]interface{})["dependencies"]
		dependencies := map[string]interface{}{}
		if exists {
			var ok bool
			dependencies, ok = dependenciesInterface.(map[string]interface{})
			if !ok {
				return nil, errors.New("package.json (dependencies) had an unexpected form")
			}
		}
		devDependenciesInterface, exists := packageJSON.(map[string]interface{})["devDependencies"]
		if exists {
			for k, v := range devDependenciesInterface.(map[string]interface{}) {
				dependencies[k] = v
			}
		}
		allResults := result
		// There should be 1 (& only 1) instantiated dependency for each
		// dependency in package.json. We do this because we want to get the
		// actual version (not the range) that exists in lock files.
		result = make([]programDependencieAbout, len(dependencies))
		i := 0
		for _, v := range allResults {
			if _, exists := dependencies[v.Name]; exists {
				result[i] = v
				// Some direct dependenceis are also transitive dependencies. We
				// only want to grap them once.
				delete(dependencies, v.Name)
				i++
			}
		}
	}
	return result, nil
}

func getProgramDependenciesAbout(proj *workspace.Project, root string,
	transitive bool) ([]programDependencieAbout, error) {
	language := proj.Runtime.Name()
	switch language {
	case langNodejs:
		return getNodeProgramDependencies(root, transitive)
	case langPython:
		return getPythonProgramDependencies(proj, root, transitive)
	case langGo:
		return getGoProgramDependencies(transitive)
	case langDotnet:
		return getDotNetProgramDependencies(proj, transitive)
	default:
		return nil, errors.Errorf("Unknown Language: %s", language)
	}
}

func formatProgramDependenciesAbout(deps []programDependencieAbout) string {
	rows := make([]cmdutil.TableRow, len(deps))
	for i, v := range deps {
		rows[i] = cmdutil.TableRow{
			Columns: []string{v.Name, v.Version},
		}
	}
	return cmdutil.Table{
		Headers: []string{"NAME", "VERSION"},
		Rows:    rows,
	}.String()
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
	Language   string `json:"language"`
	Executable string `json:"executable"`
	// We want Version to conform to the semvar format: v0.0.0
	Version string `json:"version"`
}

func getProjectRuntimeAbout(proj *workspace.Project) (projectRuntimeAbout, error) {
	var ex, version string
	var err error
	var out []byte
	// TODO[pulumi/pulumi#1334]: move to the language plugins so we don't have
	// to hard code here.
	language := proj.Runtime.Name()
	switch language {
	case langNodejs:
		ex, err = executable.FindExecutable("node")
		if err != nil {
			return projectRuntimeAbout{}, errors.Wrap(err, "Could not find node executable")
		}
		cmd := exec.Command(ex, "--version")
		if out, err = cmd.Output(); err != nil {
			return projectRuntimeAbout{}, errors.Wrap(err, "Failed to get node version")
		}
		version = string(out)
	case langPython:
		var cmd *exec.Cmd
		// if CommandPath has an error, then so will Command. The error can
		// therefore be ignored as redundant.
		ex, _, _ = python.CommandPath()
		cmd, err = python.Command("--version")
		if err != nil {
			return projectRuntimeAbout{}, err
		}
		if out, err = cmd.Output(); err != nil {
			return projectRuntimeAbout{}, errors.Wrap(err, "Failed to get python version")
		}
		version = "v" + strings.TrimPrefix(string(out), "Python ")
	case langGo:
		ex, err = executable.FindExecutable("go")
		if err != nil {
			return projectRuntimeAbout{}, errors.Wrap(err, "Could not find python executable")
		}
		cmd := exec.Command(ex, "version")
		if out, err = cmd.Output(); err != nil {
			return projectRuntimeAbout{}, errors.Wrap(err, "Failed to get go version")
		}
		version = "v" + strings.TrimPrefix(string(out), "go version go")
	case langDotnet:
		ex, err = executable.FindExecutable("dotnet")
		if err != nil {
			return projectRuntimeAbout{}, errors.Wrap(err, "Could not find dotnet executable")
		}
		cmd := exec.Command(ex, "--version")
		if out, err = cmd.Output(); err != nil {
			return projectRuntimeAbout{}, errors.Wrap(err, "Failed to get dotnet version")
		}
		version = "v" + string(out)
	default:
		return projectRuntimeAbout{}, errors.Errorf("Unknown Language: %s", language)
	}
	version = strings.TrimSpace(version)
	return projectRuntimeAbout{
		Language:   language,
		Executable: ex,
		Version:    version,
	}, nil
}

func (runtime projectRuntimeAbout) String() string {
	return fmt.Sprintf("This project is written in %s (%s %s)\n",
		runtime.Language, runtime.Executable, runtime.Version)
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
