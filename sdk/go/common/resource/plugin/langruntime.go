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

package plugin

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	structpb "google.golang.org/protobuf/types/known/structpb"
)

// ProgramInfo contains minimal information about the program to be run.
type ProgramInfo struct {
	root       string
	program    string
	entryPoint string
	options    map[string]any
}

func NewProgramInfo(rootDirectory, programDirectory, entryPoint string, options map[string]any) ProgramInfo {
	isFileName := func(path string) bool {
		return filepath.Base(path) == path
	}

	if !filepath.IsAbs(rootDirectory) {
		panic(fmt.Sprintf("rootDirectory '%s' is not a valid path when creating ProgramInfo", rootDirectory))
	}

	if !filepath.IsAbs(programDirectory) {
		panic(fmt.Sprintf("programDirectory '%s' is not a valid path when creating ProgramInfo", programDirectory))
	}

	if !isFileName(entryPoint) && entryPoint != "." {
		panic(fmt.Sprintf("entryPoint '%s' was not a valid file name when creating ProgramInfo", entryPoint))
	}

	return ProgramInfo{
		root:       rootDirectory,
		program:    programDirectory,
		entryPoint: entryPoint,
		options:    options,
	}
}

// The programs root directory, i.e. where the Pulumi.yaml file is.
func (info ProgramInfo) RootDirectory() string {
	return info.root
}

// The programs directory, generally the same as or a subdirectory of the root directory.
func (info ProgramInfo) ProgramDirectory() string {
	return info.program
}

// The programs main entrypoint, either a file path relative to the program directory or "." for the program directory.
func (info ProgramInfo) EntryPoint() string {
	return info.entryPoint
}

// Runtime plugin options for the program
func (info ProgramInfo) Options() map[string]any {
	return info.options
}

func (info ProgramInfo) String() string {
	return fmt.Sprintf("root=%s, program=%s, entryPoint=%s", info.root, info.program, info.entryPoint)
}

func (info ProgramInfo) Marshal() (*pulumirpc.ProgramInfo, error) {
	opts, err := structpb.NewStruct(info.options)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal options: %w", err)
	}

	return &pulumirpc.ProgramInfo{
		RootDirectory:    info.root,
		ProgramDirectory: info.program,
		EntryPoint:       info.entryPoint,
		Options:          opts,
	}, nil
}

type InstallDependenciesRequest struct {
	Info                    ProgramInfo
	UseLanguageVersionTools bool
}

func (options InstallDependenciesRequest) String() string {
	return fmt.Sprintf("Info=[%s], UseLanguageVersionTools=%t", options.Info, options.UseLanguageVersionTools)
}

// LanguageRuntime is a convenient interface for interacting with language runtime plugins.  These tend to be
// dynamically loaded as plugins, although this interface hides this fact from the calling code.
type LanguageRuntime interface {
	// Closer closes any underlying OS resources associated with this plugin (like processes, RPC channels, etc).
	io.Closer
	// GetRequiredPackages computes the complete set of anticipated packages required by a program.
	GetRequiredPackages(info ProgramInfo) ([]workspace.PackageDescriptor, error)
	// Run executes a program in the language runtime for planning or deployment purposes.  If
	// info.DryRun is true, the code must not assume that side-effects or final values resulting
	// from resource deployments are actually available.  If it is false, on the other hand, a real
	// deployment is occurring and it may safely depend on these.
	//
	// Returns a triple of "error message", "bail", or real "error".  If "bail", the caller should
	// return result.Bail immediately and not print any further messages to the user.
	Run(info RunInfo) (string, bool, error)
	// GetPluginInfo returns this plugin's information.
	GetPluginInfo() (workspace.PluginInfo, error)

	// InstallDependencies will install dependencies for the project, e.g. by running `npm install` for nodejs projects.
	InstallDependencies(request InstallDependenciesRequest) error

	// RuntimeOptions returns additional options that can be set for the runtime.
	RuntimeOptionsPrompts(info ProgramInfo) ([]RuntimeOptionPrompt, error)

	// About returns information about the language runtime.
	About(info ProgramInfo) (AboutInfo, error)

	// GetProgramDependencies returns information about the dependencies for the given program.
	GetProgramDependencies(info ProgramInfo, transitiveDependencies bool) ([]DependencyInfo, error)

	// RunPlugin executes a plugin program and returns its result asynchronously.
	RunPlugin(info RunPluginInfo) (io.Reader, io.Reader, context.CancelFunc, error)

	// GenerateProject generates a program project in the given directory. This will include metadata files such
	// as Pulumi.yaml and package.json.
	GenerateProject(sourceDirectory, targetDirectory, project string,
		strict bool, loaderTarget string, localDependencies map[string]string) (hcl.Diagnostics, error)

	// GeneratePlugin generates an SDK package.
	GeneratePackage(
		directory string, schema string, extraFiles map[string][]byte,
		loaderTarget string, localDependencies map[string]string,
		local bool,
	) (hcl.Diagnostics, error)

	// GenerateProgram is similar to GenerateProject but doesn't include any metadata files, just the program
	// source code.
	GenerateProgram(program map[string]string, loaderTarget string,
		strict bool) (map[string][]byte, hcl.Diagnostics, error)

	// Pack packs a library package into a language specific artifact in the given destination directory.
	Pack(packageDirectory string, destinationDirectory string) (string, error)
}

// DependencyInfo contains information about a dependency reported by a language runtime.
// These are the languages dependencies, they are not necessarily Pulumi packages.
type DependencyInfo struct {
	// The name of the dependency.
	Name string
	// The version of the dependency. Unlike most versions in the system this is not guaranteed to be a semantic
	// version.
	Version string
}

type AboutInfo struct {
	Executable string
	Version    string
	Metadata   map[string]string
}

type RunPluginInfo struct {
	Info             ProgramInfo
	WorkingDirectory string
	Args             []string
	Env              []string
}

// RunInfo contains all of the information required to perform a plan or deployment operation.
type RunInfo struct {
	Info              ProgramInfo           // the information about the program to run.
	MonitorAddress    string                // the RPC address to the host resource monitor.
	Project           string                // the project name housing the program being run.
	Stack             string                // the stack name being evaluated.
	Pwd               string                // the program's working directory.
	Args              []string              // any arguments to pass to the program.
	Config            map[config.Key]string // the configuration variables to apply before running.
	ConfigSecretKeys  []config.Key          // the configuration keys that have secret values.
	ConfigPropertyMap resource.PropertyMap  // the configuration as a property map.
	DryRun            bool                  // true if we are performing a dry-run (preview).
	QueryMode         bool                  // true if we're only doing a query.
	Parallel          int32                 // the degree of parallelism for resource operations (<=1 for serial).
	Organization      string                // the organization name housing the program being run (might be empty).
	LoaderAddress     string                // the RPC address of the host's schema loader.
	AttachDebugger    bool                  // true if we are starting the program under a debugger.
}

type RuntimeOptionType int

const (
	PromptTypeString RuntimeOptionType = iota
	PromptTypeInt32
)

// RuntimeOptionValue represents a single value that can be selected for a runtime option.
// The value can be either a string or an int32.
type RuntimeOptionValue struct {
	PromptType  RuntimeOptionType
	StringValue string
	Int32Value  int32
	DisplayName string
}

func (v RuntimeOptionValue) Value() interface{} {
	if v.PromptType == PromptTypeString {
		return v.StringValue
	}
	return v.Int32Value
}

func (v RuntimeOptionValue) String() string {
	if v.PromptType == PromptTypeString {
		return v.StringValue
	}
	return strconv.Itoa(int(v.Int32Value))
}

func RuntimeOptionValueFromString(promptType RuntimeOptionType, value string) (RuntimeOptionValue, error) {
	switch promptType {
	case PromptTypeString:
		return RuntimeOptionValue{PromptType: PromptTypeString, StringValue: value}, nil
	case PromptTypeInt32:
		return RuntimeOptionValue{PromptType: PromptTypeInt32, Int32Value: 0}, nil
	default:
		return RuntimeOptionValue{}, fmt.Errorf("unknown prompt type %d", promptType)
	}
}

// RuntimeOptionPrompt is a prompt for a runtime option. The prompt can have multiple choices or
// be free-form if Choices is empty.
// Key is the key as used in runtime.options.<Key> in the Pulumi.yaml file.
type RuntimeOptionPrompt struct {
	Key         string
	Description string
	Choices     []RuntimeOptionValue
	Default     *RuntimeOptionValue
	PromptType  RuntimeOptionType
}

func UnmarshallRuntimeOptionPrompt(p *pulumirpc.RuntimeOptionPrompt) (RuntimeOptionPrompt, error) {
	choices := make([]RuntimeOptionValue, 0, len(p.Choices))
	for _, choice := range p.Choices {
		choices = append(choices, RuntimeOptionValue{
			PromptType:  RuntimeOptionType(choice.PromptType),
			StringValue: choice.StringValue,
			Int32Value:  choice.Int32Value,
			DisplayName: choice.DisplayName,
		})
	}

	var defaultValue *RuntimeOptionValue
	if p.Default != nil {
		defaultValue = &RuntimeOptionValue{
			PromptType:  RuntimeOptionType(p.Default.PromptType),
			StringValue: p.Default.StringValue,
			Int32Value:  p.Default.Int32Value,
			DisplayName: p.Default.DisplayName,
		}
	}

	return RuntimeOptionPrompt{
		Key:         p.Key,
		Description: p.Description,
		Choices:     choices,
		Default:     defaultValue,
		PromptType:  RuntimeOptionType(p.PromptType),
	}, nil
}

// MakeRuntimeOptionPromptChoices creates a list of runtime option values from a list of executable names.
// If an executable is not found, it will be listed with a `[not found]` suffix at the end of the list.
func MakeExecutablePromptChoices(executables ...string) []*pulumirpc.RuntimeOptionPrompt_RuntimeOptionValue {
	type packagemanagers struct {
		name  string
		found bool
	}
	pms := []packagemanagers{}
	for _, pm := range executables {
		found := true
		if _, err := exec.LookPath(pm); err != nil {
			found = false
		}
		pms = append(pms, packagemanagers{name: pm, found: found})
	}

	sort.SliceStable(pms, func(i, j int) bool {
		// Don't reorder if both are found or both are not found.
		if pms[i].found == pms[j].found {
			return false
		}
		// pms[i] is less than pms[j] if pms[i] is found.
		return pms[i].found
	})

	choices := []*pulumirpc.RuntimeOptionPrompt_RuntimeOptionValue{}
	for _, pm := range pms {
		displayName := pm.name
		if !pm.found {
			displayName += " [not found]"
		}
		choices = append(choices, &pulumirpc.RuntimeOptionPrompt_RuntimeOptionValue{
			PromptType:  pulumirpc.RuntimeOptionPrompt_STRING,
			StringValue: pm.name,
			DisplayName: displayName,
		})
	}
	return choices
}
