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
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
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

func Query(ctx *Context, q QueryInfo, opts UpdateOptions) result.Result {
	contract.Require(q != nil, "update")
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

	emitter, err := makeQueryEventEmitter(ctx.Events)
	if err != nil {
		return result.FromError(err)
	}
	defer emitter.Close()

	// First, load the package metadata and the deployment target in preparation for executing the package's program
	// and creating resources.  This includes fetching its pwd and main overrides.
	diag := newEventSink(emitter, false)
	statusDiag := newEventSink(emitter, true)

	proj := q.GetProject()
	contract.Assert(proj != nil)

	pwd, main, plugctx, err := ProjectInfoContext(&Projinfo{Proj: proj, Root: q.GetRoot()},
		opts.Host, nil, diag, statusDiag, false, tracingSpan)
	if err != nil {
		return result.FromError(err)
	}
	defer plugctx.Close()

	return query(ctx, q, QueryOptions{
		Events:      emitter,
		Diag:        diag,
		StatusDiag:  statusDiag,
		host:        opts.Host,
		pwd:         pwd,
		main:        main,
		plugctx:     plugctx,
		tracingSpan: tracingSpan,
	})
}

func newQuerySource(cancel context.Context, client deploy.BackendClient, q QueryInfo,
	opts QueryOptions) (deploy.QuerySource, error) {

	allPlugins, defaultProviderVersions, err := installPlugins(q.GetProject(), opts.pwd, opts.main,
		nil, opts.plugctx, false /*returnInstallErrors*/)
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
		cancel = opentracing.ContextWithSpan(cancel, opts.tracingSpan)
	}

	// If that succeeded, create a new source that will perform interpretation of the compiled program.
	// TODO[pulumi/pulumi#88]: we are passing `nil` as the arguments map; we need to allow a way to pass these.
	return deploy.NewQuerySource(cancel, opts.plugctx, client, &deploy.EvalRunInfo{
		Proj:    q.GetProject(),
		Pwd:     opts.pwd,
		Program: opts.main,
	}, defaultProviderVersions, nil)
}

func query(ctx *Context, q QueryInfo, opts QueryOptions) result.Result {
	// Make the current working directory the same as the program's, and restore it upon exit.
	done, chErr := fsutil.Chdir(opts.plugctx.Pwd)
	if chErr != nil {
		return result.FromError(chErr)
	}
	defer done()

	if res := runQuery(ctx, q, opts); res != nil {
		if res.IsBail() {
			return res
		}
		return result.Errorf("an error occurred while running the query: %v", res.Error())
	}
	return nil
}

func runQuery(cancelCtx *Context, q QueryInfo, opts QueryOptions) result.Result {
	ctx, cancelFunc := context.WithCancel(context.Background())
	contract.Ignore(cancelFunc)

	src, err := newQuerySource(ctx, cancelCtx.BackendClient, q, opts)
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
