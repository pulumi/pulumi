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

package engine

import (
	"context"

	"github.com/opentracing/opentracing-go"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/fsutil"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/util/result"
)

type QueryOptions struct {
	Events      eventEmitter // the channel to write events from the engine to.
	Diag        diag.Sink    // the sink to use for diag'ing.
	StatusDiag  diag.Sink    // the sink to use for diag'ing status messages.
	host        plugin.Host  // the plugin host to use for this query.
	pwd, main   string
	plugctx     *plugin.Context
	tracingSpan opentracing.Span
}

func Query(ctx *Context, u UpdateInfo, opts UpdateOptions) result.Result {
	contract.Require(u != nil, "update")
	contract.Require(ctx != nil, "ctx")

	defer func() { ctx.Events <- cancelEvent() }()

	tracingSpan := func(opName string, parentSpan opentracing.SpanContext) opentracing.Span {
		// Create a root span for the operation
		opts := []opentracing.StartSpanOption{}
		if opName != "" {
			opts = append(opts, opentracing.Tag{Key: "operation", Value: opName})
		}
		if parentSpan != nil {
			opts = append(opts, opentracing.ChildOf(parentSpan))
		}
		return opentracing.StartSpan("pulumi-query", opts...)
	}("query", ctx.ParentSpan)
	defer tracingSpan.Finish()

	emitter, err := makeEventEmitter(ctx.Events, u)
	if err != nil {
		return result.FromError(err)
	}

	// First, load the package metadata and the deployment target in preparation for executing the package's program
	// and creating resources.  This includes fetching its pwd and main overrides.
	diag := newEventSink(emitter, false)
	statusDiag := newEventSink(emitter, true)

	proj, target := u.GetProject(), u.GetTarget()
	contract.Assert(proj != nil)
	contract.Assert(target != nil)

	pwd, main, plugctx, err := ProjectInfoContext(&Projinfo{Proj: proj, Root: u.GetRoot()}, opts.host,
		target, diag, statusDiag, tracingSpan)
	if err != nil {
		return result.FromError(err)
	}

	return query(ctx, u, QueryOptions{
		Events:      emitter,
		Diag:        diag,
		StatusDiag:  statusDiag,
		host:        opts.host,
		pwd:         pwd,
		main:        main,
		plugctx:     plugctx,
		tracingSpan: tracingSpan,
	})
}

func newQuerySource(ctx context.Context, client deploy.BackendClient, u UpdateInfo,
	opts QueryOptions) (deploy.QuerySource, error) {

	allPlugins, _, err := installPlugins(u.GetProject(), opts.pwd,
		opts.main,
		u.GetTarget(), opts.plugctx)
	if err != nil {
		return nil, err
	}

	// Once we've installed all of the plugins we need, make sure that all analyzers and language plugins are
	// loaded up and ready to go. Provider plugins are loaded lazily by the provider registry and thus don't
	// need to be loaded here.
	const kinds = plugin.LanguagePlugins
	if err := ensurePluginsAreLoaded(opts.plugctx, allPlugins, kinds); err != nil {
		return nil, err
	}

	if opts.tracingSpan != nil {
		ctx = opentracing.ContextWithSpan(ctx, opts.tracingSpan)
	}

	// If that succeeded, create a new source that will perform interpretation of the compiled program.
	// TODO[pulumi/pulumi#88]: we are passing `nil` as the arguments map; we need to allow a way to pass these.
	return deploy.NewQuerySource(ctx, opts.plugctx, client, &deploy.EvalRunInfo{
		Proj:    u.GetProject(),
		Pwd:     opts.pwd,
		Program: opts.main,
		Target:  u.GetTarget(),
	})
}

func query(ctx *Context, u UpdateInfo, opts QueryOptions) result.Result {
	// Make the current working directory the same as the program's, and restore it upon exit.
	done, chErr := fsutil.Chdir(opts.plugctx.Pwd)
	if chErr != nil {
		return result.FromError(chErr)
	}
	defer done()

	if res := runQuery(ctx, u, opts); res != nil {
		if res.IsBail() {
			return res
		}
		return result.Error("an error occurred while running the query")
	}
	return nil
}

func runQuery(cancelCtx *Context, u UpdateInfo, opts QueryOptions) result.Result {
	ctx, cancelFunc := context.WithCancel(context.Background())
	contract.Ignore(cancelFunc)

	src, err := newQuerySource(ctx, cancelCtx.BackendClient, u, opts)
	if err != nil {
		return result.FromError(err)
	}

	// Set up a goroutine that will signal cancellation to the plan's plugins if the caller context
	// is cancelled.
	go func() {
		<-cancelCtx.Cancel.Canceled()

		logging.V(4).Infof("engine.runQuery(...): signalling cancellation to providers...")
		cancelFunc()
		cancelErr := opts.plugctx.Host.SignalCancellation()
		if cancelErr != nil {
			logging.V(4).Infof("engine.runQuery(...): failed to signal cancellation to providers: %v", cancelErr)
		}
	}()

	done := make(chan result.Result)
	go func() {
		done <- src.Wait()
	}()

	// Block until query completes.
	select {
	case <-cancelCtx.Cancel.Terminated():
		return result.WrapIfNonNil(cancelCtx.Cancel.TerminateErr())
	case res := <-done:
		return res
	}
}
