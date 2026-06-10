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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver"
	"github.com/opentracing/opentracing-go"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
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

// newHost constructs the default host implementation. This is the shape we plan to move to
// pkg/host as NewHost: the host is independent of any workspace, and ctx is its lifetime
// context — cancelling it is the hard stop that aborts graceful shutdown, so callers wanting a
// graceful teardown must call Close before cancelling ctx. The host's RPC server parents its
// tracing interceptors on the span carried by ctx, if any.
func newHost(
	ctx context.Context, d, statusD diag.Sink, debugging DebugContext, installLang LanguageInstaller,
) (Host, error) {
	hostCtx, hostCancel := context.WithCancel(ctx)
	host := &defaultHost{
		diag:                    d,
		statusDiag:              statusD,
		hostCtx:                 hostCtx,
		hostCancel:              hostCancel,
		analyzerPlugins:         map[analyzerPluginKey]*analyzerPlugin{},
		languagePlugins:         map[languagePluginKey]*languagePlugin{},
		resourcePlugins:         map[Provider]*resourcePlugin{},
		reportedResourcePlugins: map[string]struct{}{},
		languageLoadRequests:    make(chan pluginLoadRequest),
		loadRequests:            make(chan pluginLoadRequest),
		watchedContexts:         map[*Context]struct{}{},
		closer:                  new(sync.Once),
		debugContext:            debugging,
		installLang:             installLang,
	}

	// Fire up a gRPC server to listen for requests.  This acts as a RPC interface that plugins can use
	// to "phone home" in case there are things the host must do on behalf of the plugins (like log, etc).
	svr, err := newHostServer(host, opentracing.SpanFromContext(hostCtx))
	if err != nil {
		hostCancel()
		return nil, err
	}
	host.server = svr

	// Start a goroutine we'll use to satisfy load requests serially and avoid race conditions.
	go func() {
		for req := range host.loadRequests {
			req.result <- req.load()
		}
	}()

	// Start another goroutine we'll use to satisfy load language plugin requests, this is so other plugins
	// can be started up by a language plugin.
	go func() {
		for req := range host.languageLoadRequests {
			req.result <- req.load()
		}
	}()

	return host, nil
}

// NewDefaultHost implements the standard plugin logic, using the standard installation root to find them.
//
// ctx is only used to wire up the host's RPC server and logging; the host does not retain it.
// Per-workspace state (project plugins, stack configuration, and so on) is carried by the
// Context passed to each host method, so the host may be shared across workspaces. The host is
// owned by ctx if ctx was constructed with a nil host, and by the caller otherwise.
func NewDefaultHost(ctx *Context, debugging DebugContext, installLang LanguageInstaller) (Host, error) {
	// The host's lifetime context deliberately strips the bootstrap context's cancellation: the
	// host is responsible for calling Close, and a caller's cancelled context must not turn
	// into an accidental hard kill of plugin shutdown. Once host construction moves to pkg and
	// takes a caller context directly, that context will become the exposed hard cut-off.
	return newHost(context.WithoutCancel(ctx.Request()), ctx.Diag, ctx.StatusDiag, debugging, installLang)
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

type pluginLoadRequest struct {
	load   func() error
	result chan<- error
}

type defaultHost struct {
	diag       diag.Sink // the sink to use for diagnostics, e.g. plugins logging through the host.
	statusDiag diag.Sink // the sink to use for status messages.

	// hostCtx is the host's lifetime context: the context watchers and the graceful shutdown
	// RPCs (Cancel, SignalCancellation) run under it. It is independent of any workspace
	// context, so a cancelled workspace still leaves plugin shutdown its timeout budget;
	// cancelling hostCtx is the hard stop that aborts graceful shutdown. Close cancels it as
	// its final act. It preserves the active tracing span of the context the host was built
	// with so shutdown RPCs parent onto the current operation.
	hostCtx    context.Context
	hostCancel context.CancelFunc

	analyzerPlugins         map[analyzerPluginKey]*analyzerPlugin // a cache of analyzer plugins and their processes.
	languagePlugins         map[languagePluginKey]*languagePlugin // a cache of language plugins and their processes.
	resourcePlugins         map[Provider]*resourcePlugin          // the set of loaded resource plugins.
	reportedResourcePlugins map[string]struct{}                   // the set of unique resource plugins we'll report.
	languageLoadRequests    chan pluginLoadRequest                // a channel used to satisfy language load requests.
	loadRequests            chan pluginLoadRequest                // a channel used to satisfy plugin load requests.
	server                  *hostServer                           // the server's RPC machinery.
	debugContext            DebugContext

	// Used to synchronize shutdown with in-progress plugin loads.
	pluginLock sync.RWMutex

	// The contexts whose cancellation the host is watching, so that each context's plugins are
	// released when it is cancelled (Context.Close cancels the context's base context).
	watchedContexts map[*Context]struct{}
	watchedMu       sync.Mutex

	closer *sync.Once

	installLang LanguageInstaller // installs unbundled language runtimes on demand; may be nil.
}

var _ Host = (*defaultHost)(nil)

// analyzerPluginKey identifies a booted analyzer plugin. Analyzers are spawned in the
// workspace's working directory and resolved against its project plugins; policy analyzers are
// instead booted from an explicit path and configured by their options. A host shared across
// workspaces must not serve one workspace's analyzer process to another.
type analyzerPluginKey struct {
	name             tokens.QName
	policy           bool   // true for policy analyzers.
	workingDirectory string // the workspace's working directory; empty for policy analyzers.
	path             string // the policy pack path; empty for regular analyzers.
	options          string // the policy analyzer's options, deterministically rendered; empty for regular analyzers.
}

type analyzerPlugin struct {
	Plugin Analyzer
	Info   PluginInfo
	Name   string
	refs   map[*Context]struct{} // the contexts this plugin was loaded for; it dies with the last one.
}

// languagePluginKey identifies a booted language plugin. The working directory is part of the
// key because language plugins are spawned in the workspace's working directory: a host shared
// across workspaces must not serve one workspace's language process to another.
type languagePluginKey struct {
	runtime          string
	workingDirectory string
}

type languagePlugin struct {
	Plugin LanguageRuntime
	Info   PluginInfo
	Name   string
	refs   map[*Context]struct{} // the contexts this plugin was loaded for; it dies with the last one.
}

type resourcePlugin struct {
	Plugin Provider
	Info   PluginInfo
	Name   string
	ctx    *Context // the context the provider was booted for; providers are never shared across contexts.
}

func (host *defaultHost) ServerAddr() string {
	return host.server.Address()
}

func (host *defaultHost) Log(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	host.diag.Logf(sev, diag.StreamMessage(urn, msg, streamID))
}

func (host *defaultHost) LogStatus(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	host.statusDiag.Logf(sev, diag.StreamMessage(urn, msg, streamID))
}

func (host *defaultHost) StartDebugging(info DebuggingInfo) error {
	if host.debugContext == nil {
		return errors.New("debugging is not enabled")
	}
	return host.debugContext.StartDebugging(info)
}

func (host *defaultHost) AttachDebugger(spec DebugSpec) bool {
	return host.debugContext != nil && host.debugContext.AttachDebugger(spec)
}

// loadPlugin sends an appropriate load request to the plugin loader and returns the loaded plugin (if any) and error.
func (host *defaultHost) loadPlugin(
	loadRequestChannel chan pluginLoadRequest, load func() (any, error),
) (any, error) {
	var plugin any

	locked := host.pluginLock.TryRLock()
	if !locked {
		// If we couldn't get a read lock that must be because we're shutting down, so just return an error.
		return nil, errors.New("plugin host is shutting down")
	}
	defer host.pluginLock.RUnlock()

	result := make(chan error)
	loadRequestChannel <- pluginLoadRequest{
		load: func() error {
			p, err := load()
			plugin = p
			return err
		},
		result: result,
	}
	return plugin, <-result
}

func (host *defaultHost) Analyzer(ctx *Context, name tokens.QName) (Analyzer, error) {
	host.watchContext(ctx)
	key := analyzerPluginKey{name: name, workingDirectory: ctx.Pwd}
	plugin, err := host.loadPlugin(host.loadRequests, func() (any, error) {
		// First see if we already loaded this plugin.
		if plug, has := host.analyzerPlugins[key]; has {
			contract.Assertf(plug != nil, "analyzer plugin %v was loaded but is nil", name)
			plug.refs[ctx] = struct{}{}
			return plug.Plugin, nil
		}

		// If not, try to load and bind to a plugin.
		plug, err := NewAnalyzer(host, ctx, name)
		if err == nil && plug != nil {
			info, infoerr := plug.GetPluginInfo(ctx.Request())
			if infoerr != nil {
				return nil, infoerr
			}

			// Memoize the result.
			host.analyzerPlugins[key] = &analyzerPlugin{
				Plugin: plug, Info: info, Name: string(name), refs: map[*Context]struct{}{ctx: {}},
			}
		}

		return plug, err
	})
	if plugin == nil || err != nil {
		return nil, err
	}
	return plugin.(Analyzer), nil
}

func (host *defaultHost) PolicyAnalyzer(
	ctx *Context, name tokens.QName, path string, opts *PolicyAnalyzerOptions,
) (Analyzer, error) {
	host.watchContext(ctx)
	// The options are part of the cache key: they configure the analyzer process (stack,
	// configuration, environment), so a cached analyzer may only be reused for a call that
	// would boot an identical one. fmt prints maps with sorted keys, making the
	// representation deterministic.
	optsKey := ""
	if opts != nil {
		optsKey = fmt.Sprintf("%v", *opts)
	}
	key := analyzerPluginKey{name: name, policy: true, path: path, options: optsKey}
	plugin, err := host.loadPlugin(host.loadRequests, func() (any, error) {
		// First see if we already loaded this plugin.
		if plug, has := host.analyzerPlugins[key]; has {
			contract.Assertf(plug != nil, "analyzer plugin %v was loaded but is nil", name)
			plug.refs[ctx] = struct{}{}
			return plug.Plugin, nil
		}

		// If not, try to load and bind to a plugin.
		plug, err := NewPolicyAnalyzer(host, ctx, name, path, opts, nil)
		if err == nil && plug != nil {
			info, infoerr := plug.GetPluginInfo(ctx.Request())
			if infoerr != nil {
				return nil, infoerr
			}

			// Memoize the result.
			host.analyzerPlugins[key] = &analyzerPlugin{
				Plugin: plug, Info: info, Name: string(name), refs: map[*Context]struct{}{ctx: {}},
			}
		}

		return plug, err
	})
	if plugin == nil || err != nil {
		return nil, err
	}
	return plugin.(Analyzer), nil
}

func (host *defaultHost) Provider(ctx *Context, descriptor workspace.PluginDescriptor, e env.Env) (Provider, error) {
	host.watchContext(ctx)
	plugin, err := host.loadPlugin(host.loadRequests, func() (any, error) {
		pkg := descriptor.Name
		version := descriptor.Version

		// Try to load and bind to a plugin.

		result := make(map[string]string)
		for k, v := range ctx.Config() {
			if k.Namespace() != pkg {
				continue
			}
			result[k.Name()] = v
		}
		jsonConfig, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("Could not marshal config to JSON: %w", err)
		}
		plug, err := NewProvider(
			host, ctx, descriptor,
			ctx.RuntimeOptions(), ctx.DisableProviderPreview(), string(jsonConfig), ctx.ProjectName(), e)
		if err == nil && plug != nil {
			info, infoerr := plug.GetPluginInfo(ctx.Request())
			if infoerr != nil {
				return nil, infoerr
			}

			// Warn if the plugin version was not what we expected
			if version != nil && !env.Dev.Value() {
				if info.Version == nil || !info.Version.GTE(*version) {
					var v string
					if info.Version != nil {
						v = info.Version.String()
					}
					ctx.Diag.Warningf(
						diag.Message("", /*urn*/
							"resource plugin %s is expected to have version >=%s, but has %s; "+
								"the wrong version may be on your path, or this may be a bug in the plugin"),
						pkg, version.String(), v)
				}
			}

			// Record the result and add the plugin's info to our list of loaded plugins if it's the first copy of its
			// kind.
			key := pkg
			if info.Version != nil {
				key += info.Version.String()
			}
			_, alreadyReported := host.reportedResourcePlugins[key]
			if !alreadyReported {
				host.reportedResourcePlugins[key] = struct{}{}
			}
			host.resourcePlugins[plug] = &resourcePlugin{Plugin: plug, Info: info, Name: pkg, ctx: ctx}
		}

		return plug, err
	})
	if plugin == nil || err != nil {
		return nil, err
	}

	provider := plugin.(Provider)
	return hostManagedProvider{provider, host}, nil
}

// hostManagedProvider wraps a Provider such that it can be closed by the host that created it.
type hostManagedProvider struct {
	Provider

	host *defaultHost
}

// Overrides the wrapped provider's implementation of Provider.Close to ask the managing plugin host to close the
// provider.
func (pc hostManagedProvider) Close() error {
	// Send Cancel before tearing the plugin down so that the plugin can acknowledge a graceful shutdown and
	// Plugin.Close does not treat the subsequent exit as a premature crash. defaultHost.Close does the same for
	// providers still in resourcePlugins at shutdown, but callers that Close individual providers (e.g. the
	// convert mapper) bypass that path.
	cancelCtx, cancelCancel := context.WithTimeout(pc.host.hostCtx, 5*time.Second)
	defer cancelCancel()
	contract.IgnoreError(pc.SignalCancellation(cancelCtx))

	// NOTE: we're abusing loadPlugin in order to ensure proper synchronization.
	_, err := pc.host.loadPlugin(pc.host.loadRequests, func() (any, error) {
		if err := pc.Provider.Close(); err != nil {
			return nil, err
		}
		delete(pc.host.resourcePlugins, pc.Provider)
		return nil, nil
	})
	return err
}

func (host *defaultHost) LanguageRuntime(ctx *Context, runtime string,
) (LanguageRuntime, error) {
	host.watchContext(ctx)
	key := languagePluginKey{runtime: runtime, workingDirectory: ctx.Pwd}
	// Language runtimes use their own loading channel not the main one
	plugin, err := host.loadPlugin(host.languageLoadRequests, func() (any, error) {
		// First see if we already loaded this plugin.
		if plug, has := host.languagePlugins[key]; has {
			contract.Assertf(plug != nil, "language plugin %v was loaded but is nil", runtime)
			plug.refs[ctx] = struct{}{}
			return plug.Plugin, nil
		}

		// Download and install the language runtime on demand if it is unbundled and missing.
		if host.installLang != nil {
			if err := host.installLang(ctx.Request(), runtime); err != nil {
				return nil, fmt.Errorf("failed to install language plugin %s: %w", runtime, err)
			}
		}

		// If not, allocate a new one.
		plug, err := NewLanguageRuntime(host, ctx, runtime, ctx.Pwd)
		if err == nil && plug != nil {
			info, infoerr := plug.GetPluginInfo(ctx.Request())
			if infoerr != nil {
				return nil, infoerr
			}

			// Memoize the result.
			host.languagePlugins[key] = &languagePlugin{
				Plugin: plug, Info: info, Name: runtime, refs: map[*Context]struct{}{ctx: {}},
			}
		}

		return plug, err
	})
	if plugin == nil || err != nil {
		return nil, err
	}
	return plugin.(LanguageRuntime), nil
}

func (host *defaultHost) ResolvePlugin(ctx *Context, spec workspace.PluginDescriptor) (*workspace.PluginInfo, error) {
	return workspace.GetPluginInfo(ctx.baseContext, ctx.Diag, spec, ctx.ProjectPlugins())
}

// watchContext arranges for the plugins booted on behalf of ctx to be released once ctx's base
// context is cancelled, which Context.Close guarantees. Watching is idempotent per context.
// Release is asynchronous: it happens shortly after cancellation, and host.Close remains the
// synchronous backstop that tears down anything still running.
func (host *defaultHost) watchContext(ctx *Context) {
	base := ctx.Base()
	if base == nil {
		// A context without a base can never signal cancellation; its plugins live until the
		// host closes.
		return
	}

	host.watchedMu.Lock()
	if _, has := host.watchedContexts[ctx]; has {
		host.watchedMu.Unlock()
		return
	}
	host.watchedContexts[ctx] = struct{}{}
	host.watchedMu.Unlock()

	go func() {
		select {
		case <-base.Done():
			host.releaseContextPlugins(ctx)
		case <-host.hostCtx.Done():
			// The host closed every plugin already; nothing left to release.
		}
	}()
}

// releaseContextPlugins closes the plugins booted on behalf of ctx: every provider the context
// booted, and every cached language runtime or analyzer that no other context still references.
// The load-request channels serialize access to the plugin maps, so this synchronizes with
// in-flight loads the same way Close and SignalCancellation do.
//
// A plugin load that is in flight while its context is released is not torn down here; it stays
// cached until the host closes.
func (host *defaultHost) releaseContextPlugins(ctx *Context) {
	closePlugins := func(channel chan pluginLoadRequest, close func(cancelCtx context.Context)) error {
		_, err := host.loadPlugin(channel, func() (any, error) {
			cancelCtx, cancelCancel := context.WithTimeout(host.hostCtx, 5*time.Second)
			defer cancelCancel()
			close(cancelCtx)
			return nil, nil
		})
		return err
	}

	err := closePlugins(host.loadRequests, func(cancelCtx context.Context) {
		for key, plug := range host.resourcePlugins {
			if plug.ctx != ctx {
				continue
			}
			contract.IgnoreError(plug.Plugin.SignalCancellation(cancelCtx))
			if err := plug.Plugin.Close(); err != nil {
				logging.V(5).Infof("Error closing '%s' resource plugin during context close; ignoring: %v", plug.Name, err)
			}
			delete(host.resourcePlugins, key)
		}
		for key, plug := range host.analyzerPlugins {
			if _, has := plug.refs[ctx]; !has {
				continue
			}
			delete(plug.refs, ctx)
			if len(plug.refs) > 0 {
				continue
			}
			contract.IgnoreError(plug.Plugin.Cancel(cancelCtx))
			if err := plug.Plugin.Close(); err != nil {
				logging.V(5).Infof("Error closing '%s' analyzer plugin during context close; ignoring: %v", plug.Name, err)
			}
			delete(host.analyzerPlugins, key)
		}
	})
	if err != nil {
		// The only error loadPlugin returns here is that the host is shutting down, in which
		// case Close is already tearing every plugin down; there is nothing left to release.
		return
	}

	// Language plugins are guarded by their own load channel.
	err = closePlugins(host.languageLoadRequests, func(cancelCtx context.Context) {
		for key, plug := range host.languagePlugins {
			if _, has := plug.refs[ctx]; !has {
				continue
			}
			delete(plug.refs, ctx)
			if len(plug.refs) > 0 {
				continue
			}
			contract.IgnoreError(plug.Plugin.Cancel(cancelCtx))
			if err := plug.Plugin.Close(); err != nil {
				logging.V(5).Infof("Error closing '%s' language plugin during context close; ignoring: %v", plug.Name, err)
			}
			delete(host.languagePlugins, key)
		}
	})
	contract.IgnoreError(err)
}

func (host *defaultHost) SignalCancellation() error {
	// NOTE: we're abusing loadPlugin in order to ensure proper synchronization.
	_, err := host.loadPlugin(host.loadRequests, func() (any, error) {
		cancelCtx, cancelCancel := context.WithTimeout(host.hostCtx, 30*time.Second)
		defer cancelCancel()

		// Cancel in two phases: first resource providers and analyzers, then language hosts. RunPlugin-based providers
		// run inside a language host, so we cancel non-language host plugins first to give them a chance to shut down
		// cleanly before cancelling the language host that spawned them.
		var (
			mu   sync.Mutex
			errs []error
		)

		var wg sync.WaitGroup
		for _, plug := range host.resourcePlugins {
			wg.Go(func() {
				if err := plug.Plugin.SignalCancellation(cancelCtx); err != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf(
						"error signaling cancellation to resource provider '%s': %w", plug.Name, err))
					mu.Unlock()
				}
			})
		}
		for _, plug := range host.analyzerPlugins {
			wg.Go(func() {
				if err := plug.Plugin.Cancel(cancelCtx); err != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf(
						"error signaling cancellation to analyzer '%s': %w", plug.Name, err))
					mu.Unlock()
				}
			})
		}
		wg.Wait()

		for _, plug := range host.languagePlugins {
			wg.Go(func() {
				if err := plug.Plugin.Cancel(cancelCtx); err != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf(
						"error signaling cancellation to language runtime '%s': %w", plug.Name, err))
					mu.Unlock()
				}
			})
		}
		wg.Wait()

		return nil, errors.Join(errs...)
	})
	return err
}

func (host *defaultHost) Close() (err error) {
	host.closer.Do(func() {
		// Wait for all plugins to finish loading, we do this by taking a Write lock on the pluginLock. This
		// won't take until all read locks are released (indicating that no plugins are currently loading) and
		// it will then block further read locks from being taken (preventing any new plugins from loading).
		host.pluginLock.Lock()
		// N.B We purposefully do not unlock this.

		cancelCtx, cancelCancel := context.WithTimeout(host.hostCtx, 5*time.Second)
		defer cancelCancel()

		// Close plugins in two phases: first resource providers and analyzers, then language hosts. RunPlugin-based
		// providers run inside a language host, so we close them first to give them a chance to shut down cleanly
		// before closing the language host that spawned them. Each plugin gets a Cancel RPC before being killed, giving
		// it a chance to shut down gracefully.
		var wg sync.WaitGroup
		for _, plug := range host.resourcePlugins {
			wg.Go(func() {
				contract.IgnoreError(plug.Plugin.SignalCancellation(cancelCtx))
				if err := plug.Plugin.Close(); err != nil {
					logging.V(5).Infof("Error closing '%s' resource plugin during shutdown; ignoring: %v", plug.Name, err)
				}
			})
		}
		for _, plug := range host.analyzerPlugins {
			wg.Go(func() {
				contract.IgnoreError(plug.Plugin.Cancel(cancelCtx))
				if err := plug.Plugin.Close(); err != nil {
					logging.V(5).Infof("Error closing '%s' analyzer plugin during shutdown; ignoring: %v", plug.Name, err)
				}
			})
		}
		wg.Wait()

		for _, plug := range host.languagePlugins {
			wg.Go(func() {
				contract.IgnoreError(plug.Plugin.Cancel(cancelCtx))
				if err := plug.Plugin.Close(); err != nil {
					logging.V(5).Infof("Error closing '%s' language plugin during shutdown; ignoring: %v", plug.Name, err)
				}
			})
		}
		wg.Wait()

		// Empty out all maps.
		host.analyzerPlugins = map[analyzerPluginKey]*analyzerPlugin{}
		host.languagePlugins = map[languagePluginKey]*languagePlugin{}
		host.resourcePlugins = map[Provider]*resourcePlugin{}

		// Shut down the plugin loader.
		close(host.languageLoadRequests)
		close(host.loadRequests)

		// Shut down the host's gRPC server.
		err = host.server.Cancel()

		// Finally, cancel the host's lifetime context. This stops the context watchers, and is
		// the hard stop for anything still running; it must come last so the graceful close-out
		// above keeps its timeout budget.
		host.hostCancel()
	})
	return err
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
