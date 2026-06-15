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
	"io"
	"sync"

	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	interceptors "github.com/pulumi/pulumi/sdk/v3/go/pulumi-internal/rpcdebug"
)

// Context is used to group related operations together so that
// associated OS resources can be cached, shared, and reclaimed as
// appropriate. It also carries shared plugin configuration.
type Context struct {
	Diag       diag.Sink // the diagnostics sink to use for messages.
	StatusDiag diag.Sink // the diagnostics sink to use for status messages.
	Host       Host      // the host that can be used to fetch providers.
	Pwd        string    // the working directory to spawn all plugins in.
	Root       string    // the root directory of the context.

	// If non-nil, configures custom gRPC client options. Receives pluginInfo which is a JSON-serializable bit of
	// metadata describing the plugin.
	DialOptions func(pluginInfo any) []grpc.DialOption

	// Environment injected into every plugin launched with this context, appended by ExecPlugin.
	CloudCredentialEnv map[string]string

	DebugTraceMutex *sync.Mutex // used internally to syncronize debug tracing

	tracingSpan opentracing.Span // the OpenTracing span to parent requests within.

	cancel      context.CancelFunc
	baseContext context.Context

	// Per-workspace state used when booting plugins. A Host is stateless with respect to
	// workspaces; each Host method takes a Context and reads this state from it, so a single
	// host can serve plugins for many workspaces.
	runtimeOptions         map[string]any
	disableProviderPreview bool
	config                 map[config.Key]string
	projectName            tokens.PackageName
	projectPlugins         []workspace.ProjectPlugin

	// loaderServer serves the schema loader bound to this context's workspace view, if any.
	// The loader is a workspace service, not a host service: it boots plugins to load
	// schemas, and which plugins resolve depends on the workspace. It dies with the context.
	loaderServer *GrpcServer

	// mapperServer serves the conversion mapper bound to this context's workspace view, if
	// any. Like the loader, the mapper is a workspace service: it boots plugins to source
	// mappings, and which plugins resolve depends on the workspace. It dies with the context.
	mapperServer *GrpcServer
}

// LoaderAddr returns the address of the schema loader service bound to this context, or the
// empty string if the context has none.
func (ctx *Context) LoaderAddr() string {
	if ctx.loaderServer == nil {
		return ""
	}
	return ctx.loaderServer.Addr()
}

// MapperAddr returns the address of the conversion mapper service bound to this context, or
// the empty string if the context has none.
func (ctx *Context) MapperAddr() string {
	if ctx.mapperServer == nil {
		return ""
	}
	return ctx.mapperServer.Addr()
}

// startServices binds this context's loader and mapper services, sourced from host. Each
// service is workspace-scoped: it boots plugins against this context's view and is shut down
// when the context is closed. A host may serve no loader and/or no mapper, in which case the
// corresponding service is left unset.
func (ctx *Context) startServices(host Host) error {
	loader, err := host.Loader(ctx)
	if err != nil {
		return err
	}
	ctx.loaderServer = loader

	mapper, err := host.Mapper(ctx)
	if err != nil {
		return err
	}
	ctx.mapperServer = mapper
	return nil
}

// RuntimeOptions returns the runtime options of the project this context was built for, passed
// to resource providers to support dynamic providers.
func (ctx *Context) RuntimeOptions() map[string]any {
	return ctx.runtimeOptions
}

// DisableProviderPreview returns true if provider plugins booted via this context should have
// previews disabled.
func (ctx *Context) DisableProviderPreview() bool {
	return ctx.disableProviderPreview
}

// Config returns the stack configuration this context was built with, if any.
func (ctx *Context) Config() map[config.Key]string {
	return ctx.config
}

// ProjectName returns the name of the project this context was built for, if any.
func (ctx *Context) ProjectName() tokens.PackageName {
	return ctx.projectName
}

// ProjectPlugins returns the plugins defined by the project this context was built for. These
// take precedence over installed plugins when resolving plugin binaries.
func (ctx *Context) ProjectPlugins() []workspace.ProjectPlugin {
	return ctx.projectPlugins
}

// NewContext allocates a new context with a given sink and host. The host is required and is
// owned by the caller: closing the context does not close the host, since a single host may be
// shared by several contexts.
func NewContext(ctx context.Context, d, statusD diag.Sink, host Host, _ ConfigSource,
	pwd string, runtimeOptions map[string]any, disableProviderPreview bool,
	parentSpan opentracing.Span,
) (*Context, error) {
	// TODO: really this ought to just take plugins *workspace.Plugins and packages map[string]workspace.PackageSpec
	// as args, but yaml depends on this function so *sigh*. For now just see if there's a project we should be using,
	// and use it if there is.
	projPath, err := workspace.DetectProjectPath()
	var plugins *workspace.Plugins
	var packages map[string]workspace.PackageSpec
	if err == nil && projPath != "" {
		project, err := workspace.LoadProject(projPath)
		if err == nil {
			plugins = project.Plugins
			packages = project.GetPackageSpecs()
		}
	}

	return NewContextWithRoot(ctx, d, statusD, host, pwd, pwd, runtimeOptions,
		disableProviderPreview, parentSpan, plugins, packages, nil)
}

// NewContextWithRoot is a variation of NewContext that also sets known project Root. Additionally accepts Plugins
func NewContextWithRoot(ctx context.Context, d, statusD diag.Sink, host Host,
	pwd, root string, runtimeOptions map[string]any, disableProviderPreview bool,
	parentSpan opentracing.Span, plugins *workspace.Plugins, packages map[string]workspace.PackageSpec,
	config map[config.Key]string,
) (*Context, error) {
	contract.Assertf(host != nil, "host cannot be nil")
	if d == nil {
		d = diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	}
	if statusD == nil {
		statusD = diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	}

	var projectName tokens.PackageName
	var project *workspace.Project
	projPath, err := workspace.DetectProjectPath()
	if err == nil && projPath != "" {
		if p, loadErr := workspace.LoadProject(projPath); loadErr == nil {
			project = p
			projectName = p.Name
		}
	}

	ctx, cancel := context.WithCancel(ctx)

	pctx := &Context{
		Diag:                   d,
		StatusDiag:             statusD,
		Host:                   host,
		Pwd:                    pwd,
		Root:                   root,
		tracingSpan:            parentSpan,
		DebugTraceMutex:        &sync.Mutex{},
		baseContext:            ctx,
		cancel:                 cancel,
		runtimeOptions:         runtimeOptions,
		disableProviderPreview: disableProviderPreview,
		config:                 config,
		projectName:            projectName,
		CloudCredentialEnv:     pulumiCloudCredentialEnv(project),
	}

	projectPlugins, err := projectPluginsFromProject(pctx, plugins, packages)
	if err != nil {
		cancel()
		return nil, err
	}
	pctx.projectPlugins = projectPlugins

	if err := pctx.startServices(host); err != nil {
		contract.IgnoreClose(pctx)
		return nil, err
	}

	if logFile := env.DebugGRPC.Value(); logFile != "" {
		di, err := interceptors.NewDebugInterceptor(interceptors.DebugInterceptorOptions{
			LogFile: logFile,
			Mutex:   pctx.DebugTraceMutex,
		})
		if err != nil {
			return nil, err
		}
		pctx.DialOptions = func(metadata any) []grpc.DialOption {
			return di.DialOptions(interceptors.LogOptions{
				Metadata: metadata,
			})
		}
	}

	return pctx, nil
}

// NewContextWithHost creates a new [Context] without interacting with global state.
//
// Unlike [NewContext] or [NewContextWithRoot], NewContextWithHost does not accept a nil host.
// The host is owned by the caller: closing the returned context does not close the host.
//
// d, statusD and parentSpan may all be nil.
func NewContextWithHost(
	ctx context.Context,
	d, statusD diag.Sink,
	host Host,
	pwd, root string,
	parentSpan opentracing.Span,
) (*Context, error) {
	contract.Assertf(host != nil, "NewContextWithHost requires a non-nil host")
	if d == nil {
		d = diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	}
	if statusD == nil {
		statusD = diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	}

	ctx, cancel := context.WithCancel(ctx)

	pctx := &Context{
		Diag:            d,
		StatusDiag:      statusD,
		Host:            host,
		Pwd:             pwd,
		Root:            root,
		tracingSpan:     parentSpan,
		DebugTraceMutex: &sync.Mutex{},
		cancel:          cancel,
		baseContext:     ctx,
	}

	if err := pctx.startServices(host); err != nil {
		contract.IgnoreClose(pctx)
		return nil, err
	}

	return pctx, nil
}

// Base returns this plugin context's base context; this is useful for things like cancellation.
func (ctx *Context) Base() context.Context {
	return ctx.baseContext
}

// Request allocates a request sub-context.
func (ctx *Context) Request() context.Context {
	contract.Assertf(ctx.baseContext != nil, "Context must have a base context")
	return opentracing.ContextWithSpan(ctx.baseContext, ctx.tracingSpan)
}

// Close reclaims all resources associated with this context. The host is owned by the caller
// and is not closed here, since a single host may be shared by several contexts; the caller
// that constructed the host must close it separately.
func (ctx *Context) Close() error {
	defer ctx.cancel()

	// Release everything the host booted on behalf of this context: its plugins and the loader and
	// mapper gRPC servers the host hosts for it (those are shut down after the plugins, since they
	// boot plugins through the host). ReleaseContext is synchronous, so when it returns those have
	// shut down and any diagnostics they emitted have been delivered through this context's sinks
	// -- before the caller that owns those sinks tears them down.
	err := ctx.Host.ReleaseContext(ctx)

	if ctx.tracingSpan != nil {
		ctx.tracingSpan.Finish()
	}
	return err
}
