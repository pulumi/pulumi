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

package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

// the host is independent of any workspace, and ctx is its lifetime context — cancelling it is the hard stop that
// aborts graceful shutdown, so callers wanting a graceful teardown must call Close before cancelling ctx. The host's
// RPC server parents its tracing interceptors on the span carried by ctx, if any.
func New(
	ctx context.Context, d, statusD diag.Sink, debugging plugin.DebugContext, installLang plugin.LanguageInstaller,
	newLoader plugin.NewLoaderFunc, newMapper plugin.NewMapperFunc, newResolver plugin.NewResolverFunc,
) (plugin.Host, error) {
	// d and statusD may be nil; default them to a discarding sink so that logging through the host
	// (e.g. from a plugin download-progress callback) never dereferences a nil sink.
	if d == nil {
		d = diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	}
	if statusD == nil {
		statusD = diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	}

	hostCtx, hostCancel := context.WithCancel(ctx)
	host := &defaultHost{
		diag:                    d,
		statusDiag:              statusD,
		hostCtx:                 hostCtx,
		hostCancel:              hostCancel,
		analyzerPlugins:         map[analyzerPluginKey]*analyzerPlugin{},
		languagePlugins:         map[languagePluginKey]*languagePlugin{},
		resourcePlugins:         map[plugin.Provider]*resourcePlugin{},
		reportedResourcePlugins: map[string]struct{}{},
		languageLoadRequests:    make(chan pluginLoadRequest),
		loadRequests:            make(chan pluginLoadRequest),
		closer:                  new(sync.Once),
		debugContext:            debugging,
		installLang:             installLang,
		newLoader:               newLoader,
		newMapper:               newMapper,
		newResolver:             newResolver,
		contextServers:          map[*plugin.Context][]*plugin.GrpcServer{},
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
	resourcePlugins         map[plugin.Provider]*resourcePlugin   // the set of loaded resource plugins.
	reportedResourcePlugins map[string]struct{}                   // the set of unique resource plugins we'll report.
	languageLoadRequests    chan pluginLoadRequest                // a channel used to satisfy language load requests.
	loadRequests            chan pluginLoadRequest                // a channel used to satisfy plugin load requests.
	server                  *hostServer                           // the server's RPC machinery.
	debugContext            plugin.DebugContext

	// Used to synchronize shutdown with in-progress plugin loads.
	pluginLock sync.RWMutex

	closer *sync.Once

	installLang plugin.LanguageInstaller // installs unbundled language runtimes on demand; may be nil.

	// newLoader, newMapper, and newResolver build the schema loader, conversion mapper, and package
	// resolver services bound to a given context's workspace view. They live on the host so callers
	// no longer thread them through every context constructor; each is workspace-independent and may
	// be nil, in which case the host serves no loader / no mapper / no resolver.
	newLoader   plugin.NewLoaderFunc
	newMapper   plugin.NewMapperFunc
	newResolver plugin.NewResolverFunc

	// contextServers holds the loader and mapper gRPC servers the host hosts on behalf of a
	// context. The host creates them in Loader/Mapper and shuts them down in ReleaseContext --
	// after that context's plugins, since the servers boot plugins through the host -- or in
	// Close for any context never released.
	contextServers   map[*plugin.Context][]*plugin.GrpcServer
	contextServersMu sync.Mutex
}

var _ plugin.Host = (*defaultHost)(nil)

// Loader returns a schema loader service bound to ctx's workspace view, built from the loader
// factory the host was constructed with. It returns nil if the host has no loader factory. The
// server is hosted by the host and shut down when ctx is released.
func (host *defaultHost) Loader(ctx *plugin.Context) (*plugin.GrpcServer, error) {
	return hostedService(host, ctx, host.newLoader, codegenrpc.RegisterLoaderServer)
}

// Mapper returns a conversion mapper service bound to ctx's workspace view, built from the
// mapper factory the host was constructed with. It returns nil if the host has no mapper
// factory. The server is hosted by the host and shut down when ctx is released.
func (host *defaultHost) Mapper(ctx *plugin.Context) (*plugin.GrpcServer, error) {
	return hostedService(host, ctx, host.newMapper, codegenrpc.RegisterMapperServer)
}

// Resolver returns a package resolver service bound to ctx's workspace view, built from the
// resolver factory the host was constructed with. It returns nil if the host has no resolver
// factory. The server is hosted by the host and shut down when ctx is released.
func (host *defaultHost) Resolver(ctx *plugin.Context) (*plugin.GrpcServer, error) {
	return hostedService(host, ctx, host.newResolver, pulumirpc.RegisterPackageResolverServer)
}

func hostedService[T any](
	host *defaultHost, ctx *plugin.Context,
	mk func(*plugin.Context) T, register func(s grpc.ServiceRegistrar, srv T),
) (*plugin.GrpcServer, error) {
	if mk == nil {
		return nil, nil
	}
	srv, err := plugin.NewServer(ctx, func(srv *grpc.Server) {
		register(srv, mk(ctx))
	})
	if err != nil {
		return nil, err
	}
	host.trackContextServer(ctx, srv)
	return srv, nil
}

func (host *defaultHost) trackContextServer(ctx *plugin.Context, srv *plugin.GrpcServer) {
	host.contextServersMu.Lock()
	defer host.contextServersMu.Unlock()
	host.contextServers[ctx] = append(host.contextServers[ctx], srv)
}

// releaseContextServers shuts down and forgets the gRPC servers hosted on behalf of ctx.
func (host *defaultHost) releaseContextServers(ctx *plugin.Context) error {
	host.contextServersMu.Lock()
	servers := host.contextServers[ctx]
	delete(host.contextServers, ctx)
	host.contextServersMu.Unlock()

	var errs []error
	for _, srv := range servers {
		if err := srv.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

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

type pluginLoadRequest struct {
	load   func() error
	result chan<- error
}

type analyzerPlugin struct {
	Plugin plugin.Analyzer
	Info   plugin.PluginInfo
	Name   string
	refs   map[*plugin.Context]struct{} // the contexts this plugin was loaded for; it dies with the last one.
}

// languagePluginKey identifies a booted language plugin. The working directory is part of the
// key because language plugins are spawned in the workspace's working directory: a host shared
// across workspaces must not serve one workspace's language process to another.
type languagePluginKey struct {
	runtime          string
	workingDirectory string
}

type languagePlugin struct {
	Plugin plugin.LanguageRuntime
	Info   plugin.PluginInfo
	Name   string
	refs   map[*plugin.Context]struct{} // the contexts this plugin was loaded for; it dies with the last one.
}

type resourcePlugin struct {
	Plugin plugin.Provider
	Info   plugin.PluginInfo
	Name   string
	ctx    *plugin.Context // the context the provider was booted for; providers are never shared across contexts.
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

func (host *defaultHost) StartDebugging(info plugin.DebuggingInfo) error {
	if host.debugContext == nil {
		return errors.New("debugging is not enabled")
	}
	return host.debugContext.StartDebugging(info)
}

func (host *defaultHost) AttachDebugger(spec plugin.DebugSpec) bool {
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

func (host *defaultHost) Analyzer(ctx *plugin.Context, name tokens.QName) (plugin.Analyzer, error) {
	key := analyzerPluginKey{name: name, workingDirectory: ctx.Pwd}
	hostedPlugin, err := host.loadPlugin(host.loadRequests, func() (any, error) {
		// First see if we already loaded this plugin.
		if plug, has := host.analyzerPlugins[key]; has {
			contract.Assertf(plug != nil, "analyzer plugin %v was loaded but is nil", name)
			plug.refs[ctx] = struct{}{}
			return plug.Plugin, nil
		}

		// If not, try to load and bind to a plugin.
		plug, err := plugin.NewAnalyzer(host, ctx, name)
		if err == nil && plug != nil {
			info, infoerr := plug.GetPluginInfo(ctx.Request())
			if infoerr != nil {
				return nil, infoerr
			}

			// Memoize the result.
			host.analyzerPlugins[key] = &analyzerPlugin{
				Plugin: plug, Info: info, Name: string(name), refs: map[*plugin.Context]struct{}{ctx: {}},
			}
		}

		return plug, err
	})
	if hostedPlugin == nil || err != nil {
		return nil, err
	}
	return hostedPlugin.(plugin.Analyzer), nil
}

func (host *defaultHost) PolicyAnalyzer(
	ctx *plugin.Context, name tokens.QName, path string, opts *plugin.PolicyAnalyzerOptions,
) (plugin.Analyzer, error) {
	// The options are part of the cache key: they configure the analyzer process (stack,
	// configuration, environment), so a cached analyzer may only be reused for a call that
	// would boot an identical one. fmt prints maps with sorted keys, making the
	// representation deterministic.
	optsKey := ""
	if opts != nil {
		optsKey = fmt.Sprintf("%v", *opts)
	}
	key := analyzerPluginKey{name: name, policy: true, path: path, options: optsKey}
	hostedPlugin, err := host.loadPlugin(host.loadRequests, func() (any, error) {
		// First see if we already loaded this plugin.
		if plug, has := host.analyzerPlugins[key]; has {
			contract.Assertf(plug != nil, "analyzer plugin %v was loaded but is nil", name)
			plug.refs[ctx] = struct{}{}
			return plug.Plugin, nil
		}

		// If not, try to load and bind to a plugin.
		plug, err := plugin.NewPolicyAnalyzer(host, ctx, name, path, opts, nil)
		if err == nil && plug != nil {
			info, infoerr := plug.GetPluginInfo(ctx.Request())
			if infoerr != nil {
				return nil, infoerr
			}

			// Memoize the result.
			host.analyzerPlugins[key] = &analyzerPlugin{
				Plugin: plug, Info: info, Name: string(name), refs: map[*plugin.Context]struct{}{ctx: {}},
			}
		}

		return plug, err
	})
	if hostedPlugin == nil || err != nil {
		return nil, err
	}
	return hostedPlugin.(plugin.Analyzer), nil
}

func (host *defaultHost) Provider(
	ctx *plugin.Context, descriptor workspace.PluginDescriptor, e env.Env,
) (plugin.Provider, error) {
	hostedPlugin, err := host.loadPlugin(host.loadRequests, func() (any, error) {
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
		plug, err := plugin.NewProvider(
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
			host.resourcePlugins[plug] = &resourcePlugin{
				Plugin: plug, Info: info, Name: pkg, ctx: ctx.LifetimeContext(),
			}
		}

		return plug, err
	})
	if hostedPlugin == nil || err != nil {
		return nil, err
	}

	provider := hostedPlugin.(plugin.Provider)
	return hostManagedProvider{provider, host}, nil
}

// hostManagedProvider wraps a Provider such that it can be closed by the host that created it.
type hostManagedProvider struct {
	plugin.Provider

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

func (host *defaultHost) LanguageRuntime(ctx *plugin.Context, runtime string,
) (plugin.LanguageRuntime, error) {
	key := languagePluginKey{runtime: runtime, workingDirectory: ctx.Pwd}
	// Language runtimes use their own loading channel not the main one
	hostedPlugin, err := host.loadPlugin(host.languageLoadRequests, func() (any, error) {
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
		plug, err := plugin.NewLanguageRuntime(host, ctx, runtime, ctx.Pwd)
		if err == nil && plug != nil {
			info, infoerr := plug.GetPluginInfo(ctx.Request())
			if infoerr != nil {
				return nil, infoerr
			}

			// Memoize the result.
			host.languagePlugins[key] = &languagePlugin{
				Plugin: plug, Info: info, Name: runtime, refs: map[*plugin.Context]struct{}{ctx: {}},
			}
		}

		return plug, err
	})
	if hostedPlugin == nil || err != nil {
		return nil, err
	}
	return hostedPlugin.(plugin.LanguageRuntime), nil
}

func (host *defaultHost) ResolvePlugin(
	ctx *plugin.Context, spec workspace.PluginDescriptor,
) (*workspace.PluginInfo, error) {
	return workspace.GetPluginInfo(ctx.Base(), ctx.Diag, spec, ctx.ProjectPlugins())
}

// ReleaseContext gracefully shuts down and releases the plugins booted on behalf of ctx: every
// provider the context booted, and every cached language runtime or analyzer that no other context
// still references. The load-request channels serialize access to the plugin maps, so this
// synchronizes with in-flight loads the same way Close and SignalCancellation do; it blocks until
// the plugins have shut down, so any diagnostics they emit are delivered before it returns.
// [Context.Close] calls this to reclaim the context's plugins; host.Close remains the synchronous
// backstop that tears down anything still running.
//
// A plugin load that is in flight while its context is released is not torn down here; it stays
// cached until the host closes.
func (host *defaultHost) ReleaseContext(ctx *plugin.Context) error {
	var errs []error
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
				errs = append(errs, fmt.Errorf("closing resource plugin %q: %w", plug.Name, err))
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
				errs = append(errs, fmt.Errorf("closing analyzer plugin %q: %w", plug.Name, err))
			}
			delete(host.analyzerPlugins, key)
		}
	})
	if err != nil {
		// The only error loadPlugin returns here is that the host is shutting down, in which
		// case Close is already tearing every plugin down; there is nothing left to release.
		return nil //nolint:nilerr
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
				errs = append(errs, fmt.Errorf("closing language plugin %q: %w", plug.Name, err))
			}
			delete(host.languagePlugins, key)
		}
	})
	if err != nil {
		return nil //nolint:nilerr
	}

	// Shut down the loader and mapper gRPC servers hosted for ctx, after the plugins they may have
	// booted have been released.
	errs = append(errs, host.releaseContextServers(ctx))

	return errors.Join(errs...)
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
		host.resourcePlugins = map[plugin.Provider]*resourcePlugin{}

		// Shut down the loader/mapper gRPC servers hosted for any context never released, after
		// the plugins they may have booted.
		host.contextServersMu.Lock()
		for _, servers := range host.contextServers {
			for _, srv := range servers {
				contract.IgnoreClose(srv)
			}
		}
		host.contextServers = map[*plugin.Context][]*plugin.GrpcServer{}
		host.contextServersMu.Unlock()

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
