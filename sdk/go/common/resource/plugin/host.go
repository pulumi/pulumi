// Copyright 2016, Pulumi Corporation.
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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

// A Host hosts provider plugins and makes them easily accessible by package name.
//
// A host is stateless with respect to workspaces: methods that boot or resolve plugins take a
// [Context] carrying the per-workspace state (working directory, project plugins, stack
// configuration, and so on), so a single host may be shared by several contexts. The host is
// not owned by any context it is used with; it must be closed by whoever constructed it.
type Host interface {
	// ServerAddr returns the address at which the host's RPC interface may be found.
	ServerAddr() string

	// Log logs a message, including errors and warnings.  Messages can have a resource URN
	// associated with them.  If no urn is provided, the message is global.
	Log(sev diag.Severity, urn resource.URN, msg string, streamID int32)

	// LogStatus logs a status message message, including errors and warnings. Status messages show
	// up in the `Info` column of the progress display, but not in the final output. Messages can
	// have a resource URN associated with them.  If no urn is provided, the message is global.
	LogStatus(sev diag.Severity, urn resource.URN, msg string, streamID int32)

	// Analyzer fetches the analyzer with a given name, possibly lazily allocating the plugins for
	// it.  If an analyzer could not be found, or an error occurred while creating it, a non-nil
	// error is returned.
	Analyzer(ctx *Context, nm tokens.QName) (Analyzer, error)

	// PolicyAnalyzer boots the nodejs analyzer plugin located at a given path. This is useful
	// because policy analyzers generally do not need to be "discovered" -- the engine is given a
	// set of policies that are required to be run during an update, so they tend to be in a
	// well-known place.
	PolicyAnalyzer(ctx *Context, name tokens.QName, path string, opts *PolicyAnalyzerOptions) (Analyzer, error)

	// Provider loads a new copy of the provider for a given package.  If a provider for this package could not be
	// found, or an error occurs while creating it, a non-nil error is returned. The provider is booted with the
	// workspace state carried by ctx (stack configuration, runtime options, project name).
	Provider(ctx *Context, descriptor workspace.PluginDescriptor, e env.Env) (Provider, error)
	// LanguageRuntime fetches the language runtime plugin for a given language, lazily allocating if necessary.  If
	// an implementation of this language runtime wasn't found, on an error occurs, a non-nil error is returned.
	LanguageRuntime(ctx *Context, runtime string) (LanguageRuntime, error)

	// ResolvePlugin resolves a pluginspec to a candidate plugin to load, consulting the project
	// plugins carried by ctx.
	ResolvePlugin(ctx *Context, spec workspace.PluginDescriptor) (*workspace.PluginInfo, error)

	// SignalCancellation asks all resource providers to gracefully shut down and abort any ongoing
	// operations. Operation aborted in this way will return an error (e.g., `Update` and `Create`
	// will either a creation error or an initialization error. SignalCancellation is advisory and
	// non-blocking; it is up to the host to decide how long to wait after SignalCancellation is
	// called before (e.g.) hard-closing any gRPC connection.
	SignalCancellation() error

	// StartDebugging asks the host to start a debugging session with the given configuration.
	StartDebugging(info DebuggingInfo) error

	// AttachDebugger returns true if debugging is enabled.
	AttachDebugger(spec DebugSpec) bool

	// Close reclaims any resources associated with the host.
	Close() error
}

// IsLocalPluginPath determines if a plugin source refers to a local path rather than a downloadable plugin.
// A plugin is considered local if it doesn't match the plugin name regexp and doesn't have a download URL.
func IsLocalPluginPath(ctx context.Context, source string) bool {
	// If the source starts with ./ or ../ or / it's definitely a local path
	if strings.HasPrefix(source, "./") || strings.HasPrefix(source, "..") || strings.HasPrefix(source, "/") {
		return true
	}

	// For other cases, we need to be careful about how we interpret the source, so let's parse the spec
	// and check if it has a download URL.
	pluginSpec, err := workspace.NewPluginDescriptor(ctx, source, apitype.ResourcePlugin, nil, "", nil)
	var pluginErr workspace.PluginVersionNotFoundError
	if err != nil && !errors.As(err, &pluginErr) {
		// If we can't parse it as a plugin spec, assume it's a local path
		return true
	}

	if pluginSpec.IsGitPlugin() {
		// If it's a git plugin, it's not a local path
		return false
	}

	// If there is a download URL or the name matches the plugin name regexp after parsing, it's not a local path
	return pluginSpec.PluginDownloadURL == "" && !workspace.PluginNameRegexp.MatchString(pluginSpec.Name)
}

// collectPluginsFromPackages recursively processes packages to get a complete list of plugins
func collectPluginsFromPackages(
	ctx *Context, packages map[string]workspace.PackageSpec, visited map[string]bool,
) ([]workspace.ProjectPlugin, error) {
	result := []workspace.ProjectPlugin{}

	for name, pkg := range packages {
		// Skip downloadable plugins, so that only local folder paths remain.
		if !IsLocalPluginPath(ctx.baseContext, pkg.Source) {
			continue
		}

		if visited[name] {
			continue
		}
		visited[name] = true

		path, err := resolvePluginPath(ctx.Root, pkg.Source)
		if err != nil {
			return nil, err
		}
		pluginProjectFile, err := workspace.DetectPluginPathFrom(path)
		pluginProjectFileNotFound := errors.Is(err, workspace.ErrPluginNotFound)
		if err != nil && !pluginProjectFileNotFound {
			return nil, err
		}
		if !pluginProjectFileNotFound {
			pp, err := workspace.LoadPluginProject(pluginProjectFile)
			if err != nil {
				return nil, err
			}

			subPackages := pp.GetPackageSpecs()
			if len(subPackages) > 0 {
				subPlugins, err := collectPluginsFromPackages(ctx, subPackages, visited)
				if err != nil {
					return nil, err
				}
				result = append(result, subPlugins...)
			}
		}

		result = append(result, workspace.ProjectPlugin{
			Kind: apitype.ResourcePlugin,
			Name: name,
			Path: path,
		})
	}

	return result, nil
}

// NewLoaderFunc constructs the schema loader service bound to a context. The Context supplies
// the workspace view the loader resolves and boots plugins against.
type NewLoaderFunc = func(ctx *Context) codegenrpc.LoaderServer

// NewMapperFunc constructs the conversion mapper service bound to a context. The Context
// supplies the workspace view the mapper boots conversion plugins against.
type NewMapperFunc = func(ctx *Context) codegenrpc.MapperServer

// LanguageInstaller downloads and installs an unbundled language runtime on demand, so that
// loading it via Host.LanguageRuntime works even when the runtime is not bundled with the CLI
// or already cached. It is the language-runtime analogue of the engine's plugin install path.
//
// The install machinery lives in the pkg module, which the SDK cannot import, so a host is
// given its installer at construction. Language hosts are self-contained executables — they
// are shared across workspaces and are never run with the support of another language runtime
// — so installation is a plain download-and-unpack and needs no workspace state. A nil
// LanguageInstaller disables on-demand install (the host then relies on the runtime already
// being present).
type LanguageInstaller = func(ctx context.Context, runtime string) error

// projectPluginsFromProject parses the plugins and packages declared by a project into the list
// of project plugins that take precedence over installed plugins when resolving plugin binaries.
func projectPluginsFromProject(
	ctx *Context, plugins *workspace.Plugins, packages map[string]workspace.PackageSpec,
) ([]workspace.ProjectPlugin, error) {
	projectPlugins := make([]workspace.ProjectPlugin, 0)
	if plugins != nil {
		for _, providerOpts := range plugins.Providers {
			info, err := parsePluginOpts(ctx.Root, providerOpts, apitype.ResourcePlugin)
			if err != nil {
				return nil, err
			}
			projectPlugins = append(projectPlugins, info)
		}
		for _, languageOpts := range plugins.Languages {
			info, err := parsePluginOpts(ctx.Root, languageOpts, apitype.LanguagePlugin)
			if err != nil {
				return nil, err
			}
			projectPlugins = append(projectPlugins, info)
		}
		for _, analyzerOpts := range plugins.Analyzers {
			info, err := parsePluginOpts(ctx.Root, analyzerOpts, apitype.AnalyzerPlugin)
			if err != nil {
				return nil, err
			}
			projectPlugins = append(projectPlugins, info)
		}
	}

	pluginsFromPackages, err := collectPluginsFromPackages(ctx, packages, make(map[string]bool))
	if err != nil {
		return nil, err
	}
	return append(projectPlugins, pluginsFromPackages...), nil
}

func resolvePluginPath(root string, path string) (string, error) {
	// The path is relative to the project root. Make it absolute here so we don't need to track that everywhere its used.
	var err error
	if !filepath.IsAbs(path) {
		path, err = filepath.Abs(filepath.Join(root, path))
		if err != nil {
			return "", fmt.Errorf("getting absolute path for plugin path %s: %w", path, err)
		}
	}

	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("no folder at path '%s'", path)
	} else if err != nil {
		return "", fmt.Errorf("checking provider folder: %w", err)
	} else if !stat.IsDir() {
		return "", fmt.Errorf("provider folder '%s' is not a directory", path)
	}

	return path, nil
}

func parsePluginOpts(
	root string, providerOpts workspace.PluginOptions, k apitype.PluginKind,
) (workspace.ProjectPlugin, error) {
	handleErr := func(msg string, a ...any) (workspace.ProjectPlugin, error) {
		return workspace.ProjectPlugin{},
			fmt.Errorf("parsing plugin options for '%s': %w", providerOpts.Name, fmt.Errorf(msg, a...))
	}
	if providerOpts.Name == "" {
		return handleErr("name must not be empty")
	}
	var v *semver.Version
	if providerOpts.Version != "" {
		ver, err := semver.Parse(providerOpts.Version)
		if err != nil {
			return workspace.ProjectPlugin{}, err
		}
		v = &ver
	}

	path, err := resolvePluginPath(root, providerOpts.Path)
	if err != nil {
		return handleErr("%s", err.Error())
	}

	pluginInfo := workspace.ProjectPlugin{
		Name:    providerOpts.Name,
		Path:    path,
		Kind:    k,
		Version: v,
	}
	return pluginInfo, nil
}

// PolicyAnalyzerOptions includes a bag of options to pass along to a policy analyzer.
type PolicyAnalyzerOptions struct {
	Organization     string
	Project          string
	Stack            string
	Config           map[config.Key]string
	ConfigSecretKeys []config.Key
	DryRun           bool
	Tags             map[string]string // Tags for the current stack.
	AdditionalEnv    map[string]string // Per-pack environment variables (e.g., from ESC).
}

// Flags can be used to filter out plugins during loading that aren't necessary.
type Flags int

const (
	// AnalyzerPlugins is used to only load analyzers.
	AnalyzerPlugins Flags = 1 << iota
	// LanguagePlugins is used to only load language plugins.
	LanguagePlugins
	// ResourcePlugins is used to only load resource provider plugins.
	ResourcePlugins
)
