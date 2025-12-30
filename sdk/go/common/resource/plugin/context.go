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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Context is used to group related operations together so that
// associated OS resources can be cached, shared, and reclaimed as
// appropriate.
type Context struct {
	Diag       diag.Sink // the diagnostics sink to use for messages.
	StatusDiag diag.Sink // the diagnostics sink to use for status messages.

	// If non-nil, configures custom gRPC client options. Receives pluginInfo which is a JSON-serializable bit of
	// metadata describing the plugin.
	DialOptions func(pluginInfo any) []grpc.DialOption

	DebugTraceMutex *sync.Mutex // used internally to synchronize debug tracing

	tracingSpan opentracing.Span // the OpenTracing span to parent requests within.

	cancelFuncs []context.CancelFunc
	cancelLock  *sync.Mutex // Guards cancelFuncs.
	baseContext context.Context
}

// NewContext allocates a new context with a given sink.
func NewContext(ctx context.Context, d, statusD diag.Sink, parentSpan opentracing.Span) (*Context, error) {
	if d == nil {
		d = diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	}
	if statusD == nil {
		statusD = diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	}

	pctx := &Context{
		Diag:            d,
		StatusDiag:      statusD,
		tracingSpan:     parentSpan,
		DebugTraceMutex: &sync.Mutex{},
		cancelLock:      &sync.Mutex{},
		baseContext:     ctx,
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

// TODO: Deprecate
func NewContextWithRoot(ctx context.Context, d, statusD diag.Sink, parentSpan opentracing.Span) (*Context, error) {
	return NewContext(ctx, d, statusD, parentSpan)
}

// Base returns this plugin context's base context; this is useful for things like cancellation.
func (ctx *Context) Base() context.Context {
	return ctx.baseContext
}

// Request allocates a request sub-context.
func (ctx *Context) Request() context.Context {
	c := ctx.baseContext
	contract.Assertf(c != nil, "Context must have a base context")
	c = opentracing.ContextWithSpan(c, ctx.tracingSpan)
	c, cancel := context.WithCancel(c)
	ctx.cancelLock.Lock()
	ctx.cancelFuncs = append(ctx.cancelFuncs, cancel)
	ctx.cancelLock.Unlock()
	return c
}

// Close reclaims all resources associated with this context.
func (ctx *Context) Close() error {
	defer func() {
		// It is possible that cancelFuncs may be appended while this function is running.
		// Capture the current value of cancelFuncs and set cancelFuncs to nil to prevent cancelFuncs
		// from being appended to while we are iterating over it.
		ctx.cancelLock.Lock()
		cancelFuncs := ctx.cancelFuncs
		ctx.cancelFuncs = nil
		ctx.cancelLock.Unlock()
		for _, cancel := range cancelFuncs {
			cancel()
		}
	}()
	if ctx.tracingSpan != nil {
		ctx.tracingSpan.Finish()
	}
	return nil
}

// WithCancelChannel registers a close channel which will close the returned Context when
// the channel is closed.
//
// WARNING: Calling this function without ever closing `c` will leak go routines.
func (ctx *Context) WithCancelChannel(c <-chan struct{}) *Context {
	newCtx := *ctx
	go func() {
		<-c
		newCtx.Close()
	}()
	return &newCtx
}
