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

package plugin

import (
	"context"
	"io"
	"sync"

	"github.com/opentracing/opentracing-go"
	interceptors "github.com/pulumi/pulumi/sdk/v3/go/pulumi-internal/rpcdebug"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
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

	DebugTraceMutex *sync.Mutex // used internally to syncronize debug tracing

	tracingSpan opentracing.Span // the OpenTracing span to parent requests within.

	cancel      context.CancelFunc
	baseContext context.Context
}

// NewContext allocates a new context with a given sink and host. Note
// that the host is "owned" by this context from here forwards, such
// that when the context's resources are reclaimed, so too are the
// host's.
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
		disableProviderPreview, parentSpan, plugins, packages, nil, nil)
}

// NewContextWithRoot is a variation of NewContext that also sets known project Root. Additionally accepts Plugins
func NewContextWithRoot(ctx context.Context, d, statusD diag.Sink, host Host,
	pwd, root string, runtimeOptions map[string]any, disableProviderPreview bool,
	parentSpan opentracing.Span, plugins *workspace.Plugins, packages map[string]workspace.PackageSpec,
	config map[config.Key]string, debugging DebugContext,
) (*Context, error) {
	if d == nil {
		d = diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	}
	if statusD == nil {
		statusD = diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	}

	var projectName tokens.PackageName
	projPath, err := workspace.DetectProjectPath()
	if err == nil && projPath != "" {
		project, err := workspace.LoadProject(projPath)
		if err == nil {
			projectName = project.Name
		}
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
		baseContext:     ctx,
		cancel:          cancel,
	}
	if host == nil {
		h, err := NewDefaultHost(
			pctx, runtimeOptions, disableProviderPreview, plugins, packages, config, debugging, projectName,
		)
		if err != nil {
			return nil, err
		}
		pctx.Host = h
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
// Unilke [NewDefaultContext] or [NewContextWithRoot], NewContextWithHost does not accept
// a nil host.
//
// d, statusD and parentSpan may all be nil.
func NewContextWithHost(
	ctx context.Context,
	d, statusD diag.Sink,
	host Host,
	pwd, root string,
	parentSpan opentracing.Span,
) *Context {
	contract.Assertf(host != nil, "NewContextWithHost requires a non-nil host")
	if d == nil {
		d = diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	}
	if statusD == nil {
		statusD = diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	}

	ctx, cancel := context.WithCancel(ctx)

	return &Context{
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

// Close reclaims all resources associated with this context.
func (ctx *Context) Close() error {
	defer ctx.cancel()
	if ctx.tracingSpan != nil {
		ctx.tracingSpan.Finish()
	}
	err := ctx.Host.Close()
	if err != nil && !rpcutil.IsBenignCloseErr(err) {
		return err
	}
	return nil
}
