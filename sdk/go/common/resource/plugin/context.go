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
	"io"
	"sync"

	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
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
	Root       string    // the root directory of the project.

	// If non-nil, configures custom gRPC client options. Receives pluginInfo which is a JSON-serializable bit of
	// metadata describing the plugin.
	DialOptions func(pluginInfo interface{}) []grpc.DialOption

	DebugTraceMutex *sync.Mutex // used internally to syncronize debug tracing

	tracingSpan opentracing.Span // the OpenTracing span to parent requests within.

	cancelFuncs []context.CancelFunc
	baseContext context.Context
}

// NewContext allocates a new context with a given sink and host. Note
// that the host is "owned" by this context from here forwards, such
// that when the context's resources are reclaimed, so too are the
// host's.
func NewContext(d, statusD diag.Sink, host Host, _ ConfigSource,
	pwd string, runtimeOptions map[string]interface{}, disableProviderPreview bool,
	parentSpan opentracing.Span) (*Context, error) {

	// TODO: I think really this ought to just take plugins *workspace.Plugins as an arg, but yaml depends on
	// this function so *sigh*. For now just see if there's a project we should be using, and use it if there
	// is.
	projPath, err := workspace.DetectProjectPath()
	var plugins *workspace.Plugins
	if err == nil && projPath != "" {
		project, err := workspace.LoadProject(projPath)
		if err == nil {
			plugins = project.Plugins
		}
	}

	root := ""
	return NewContextWithRoot(d, statusD, host, pwd, root, runtimeOptions,
		disableProviderPreview, parentSpan, plugins)
}

// NewContextWithRoot is a variation of NewContext that also sets known project Root. Additionally accepts Plugins
func NewContextWithRoot(d, statusD diag.Sink, host Host,
	pwd, root string, runtimeOptions map[string]interface{}, disableProviderPreview bool,
	parentSpan opentracing.Span, plugins *workspace.Plugins) (*Context, error) {

	if d == nil {
		d = diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	}
	if statusD == nil {
		statusD = diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	}

	ctx := &Context{
		Diag:            d,
		StatusDiag:      statusD,
		Host:            host,
		Pwd:             pwd,
		tracingSpan:     parentSpan,
		DebugTraceMutex: &sync.Mutex{},
	}
	if host == nil {
		h, err := NewDefaultHost(ctx, runtimeOptions, disableProviderPreview, plugins)
		if err != nil {
			return nil, err
		}
		ctx.Host = h
	}
	return ctx, nil
}

// Request allocates a request sub-context.
func (ctx *Context) Request() context.Context {
	c := ctx.baseContext
	if c == nil {
		c = context.Background()
	}
	c = opentracing.ContextWithSpan(c, ctx.tracingSpan)
	c, cancel := context.WithCancel(c)
	ctx.cancelFuncs = append(ctx.cancelFuncs, cancel)
	return c
}

// Close reclaims all resources associated with this context.
func (ctx *Context) Close() error {
	defer func() {
		for _, cancel := range ctx.cancelFuncs {
			cancel()
		}
	}()
	if ctx.tracingSpan != nil {
		ctx.tracingSpan.Finish()
	}
	err := ctx.Host.Close()
	if err != nil && !rpcutil.IsBenignCloseErr(err) {
		return err
	}
	return nil
}

// WithCancelChannel registers a close channel which will close the returned Context when
// the channel is closed.
//
// WARNING: Calling this function without ever closing `c` will leak go routines.
func (ctx *Context) WithCancelChannel(c <-chan struct{}) *Context {
	copy := *ctx
	go func() {
		select {
		case _, _ = <-c:
			copy.Close()
		}
	}()
	return &copy
}
