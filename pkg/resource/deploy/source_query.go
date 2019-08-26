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

package deploy

import (
	"context"
	"fmt"
	"math"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/result"
	"github.com/pulumi/pulumi/pkg/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

// QuerySource evaluates a query program, and provides the ability to synchronously wait for
// completion.
type QuerySource interface {
	Wait() result.Result
}

// NewQuerySource creates a `QuerySource` for some target runtime environment specified by
// `runinfo`, and supported by language plugins provided in `plugctx`.
func NewQuerySource(ctx context.Context, plugctx *plugin.Context, client BackendClient,
	runinfo *EvalRunInfo) (QuerySource, error) {

	// Create a new builtin provider. This provider implements features such as `getStack`.
	builtins := newBuiltinProvider(client)

	// First, fire up a resource monitor that will disallow all resource operations, as well as
	// service calls for things like resource ouptuts of state snapshots.
	//
	// NOTE: Using the queryResourceMonitor here is *VERY* important, as its job is to disallow
	// resource operations in query mode!
	mon, err := newQueryResourceMonitor(builtins, opentracing.SpanFromContext(ctx))
	if err != nil {
		return nil, errors.Wrap(err, "failed to start resource monitor")
	}

	// Create a new iterator with appropriate channels, and gear up to go!
	src := &querySource{
		mon:           mon,
		plugctx:       plugctx,
		runinfo:       runinfo,
		runLangPlugin: runLangPlugin,
		finChan:       make(chan result.Result),
		cancel:        ctx,
	}

	// Now invoke Run in a goroutine.  All subsequent resource creation events will come in over the gRPC channel,
	// and we will pump them through the channel.  If the Run call ultimately fails, we need to propagate the error.
	src.forkRun()

	// Finally, return the fresh iterator that the caller can use to take things from here.
	return src, nil
}

type querySource struct {
	mon           SourceResourceMonitor            // the resource monitor, per iterator.
	plugctx       *plugin.Context                  // the plugin context.
	runinfo       *EvalRunInfo                     // the directives to use when running the program.
	runLangPlugin func(*querySource) result.Result // runs the language plugin.
	finChan       chan result.Result               // the channel that communicates completion.
	done          bool                             // set to true when the evaluation is done.
	res           result.Result                    // result when the channel is finished.
	cancel        context.Context
}

func (src *querySource) Close() error {
	// Cancel the monitor and reclaim any associated resources.
	src.done = true
	close(src.finChan)
	return src.mon.Cancel()
}

func (src *querySource) Wait() result.Result {
	// If we are done, quit.
	if src.done {
		return src.res
	}

	select {
	case src.res = <-src.finChan:
		// Language plugin has exited. No need to call `Close`.
		src.done = true
		return src.res
	case <-src.cancel.Done():
		src.done = true
		src.Close()
		return src.res
	}
}

// forkRun evaluate the query program in a separate goroutine. Completion or cancellation will cause
// `Wait` to stop blocking and return.
func (src *querySource) forkRun() {
	// Fire up the goroutine to make the RPC invocation against the language runtime.  As this executes, calls
	// to queue things up in the resource channel will occur, and we will serve them concurrently.
	go func() {
		// Next, launch the language plugin. Communicate the error, if it exists, or nil if the
		// program exited cleanly.
		src.finChan <- src.runLangPlugin(src)
	}()
}

func runLangPlugin(src *querySource) result.Result {
	rt := src.runinfo.Proj.Runtime.Name()
	langhost, err := src.plugctx.Host.LanguageRuntime(rt)
	if err != nil {
		return result.FromError(errors.Wrapf(err, "failed to launch language host %s", rt))
	}
	contract.Assertf(langhost != nil, "expected non-nil language host %s", rt)

	// Make sure to clean up before exiting.
	defer contract.IgnoreClose(langhost)

	// Decrypt the configuration.
	var config map[config.Key]string
	if src.runinfo.Target != nil {
		config, err = src.runinfo.Target.Config.Decrypt(src.runinfo.Target.Decrypter)
		if err != nil {
			return result.FromError(err)
		}
	}

	var name string
	if src.runinfo.Target != nil {
		name = string(src.runinfo.Target.Name)
	}

	// Now run the actual program.
	progerr, bail, err := langhost.Run(plugin.RunInfo{
		MonitorAddress: src.mon.Address(),
		Stack:          name,
		Project:        string(src.runinfo.Proj.Name),
		Pwd:            src.runinfo.Pwd,
		Program:        src.runinfo.Program,
		Args:           src.runinfo.Args,
		Config:         config,
		DryRun:         true,
		QueryMode:      true,
		Parallel:       math.MaxInt32,
	})

	// Check if we were asked to Bail.  This a special random constant used for that
	// purpose.
	if err == nil && bail {
		return result.Bail()
	}

	if err == nil && progerr != "" {
		// If the program had an unhandled error; propagate it to the caller.
		err = errors.Errorf("an unhandled error occurred: %v", progerr)
	}
	return result.WrapIfNonNil(err)
}

// newQueryResourceMonitor creates a new resource monitor RPC server intended to be used in Pulumi's
// "query mode".
func newQueryResourceMonitor(builtins *builtinProvider, tracingSpan opentracing.Span) (*queryResmon, error) {

	// Create our cancellation channel.
	cancel := make(chan bool)

	// New up an engine RPC server.
	queryResmon := &queryResmon{
		builtins: builtins,
		cancel:   cancel,
	}

	// Fire up a gRPC server and start listening for incomings.
	port, done, err := rpcutil.Serve(0, queryResmon.cancel, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceMonitorServer(srv, queryResmon)
			return nil
		},
	}, tracingSpan)
	if err != nil {
		return nil, err
	}

	queryResmon.addr = fmt.Sprintf("127.0.0.1:%d", port)
	queryResmon.done = done

	return queryResmon, nil
}

// queryResmon is a pulumirpc.ResourceMonitor that is meant to run in Pulumi's "query mode". It
// performs two critical functions:
//
// 1. Disallows all resource operations. `queryResmon` intercepts all resource operations and
//    returns an error instead of allowing them to proceed.
// 2. Services requests for stack snapshots. This is primarily to allow us to allow queries across
//    stack snapshots.
type queryResmon struct {
	builtins *builtinProvider // provides builtins such as `getStack`.
	addr     string           // the address the host is listening on.
	cancel   chan bool        // a channel that can cancel the server.
	done     chan error       // a channel that resolves when the server completes.
}

var _ SourceResourceMonitor = (*queryResmon)(nil)

// Address returns the address at which the monitor's RPC server may be reached.
func (rm *queryResmon) Address() string {
	return rm.addr
}

// Cancel signals that the engine should be terminated, awaits its termination, and returns any
// errors that result.
func (rm *queryResmon) Cancel() error {
	close(rm.cancel)
	return <-rm.done
}

// Invoke performs an invocation of a member located in a resource provider.
func (rm *queryResmon) Invoke(ctx context.Context, req *pulumirpc.InvokeRequest) (*pulumirpc.InvokeResponse, error) {
	tok := tokens.ModuleMember(req.GetTok())
	label := fmt.Sprintf("QueryResourceMonitor.Invoke(%s)", tok)

	// Fail on all calls to `Invoke` except this one.
	if tok != readStackResourceOutputs {
		return nil, fmt.Errorf("Query mode does not support invoke call for operation '%s'", tok)
	}

	args, err := plugin.UnmarshalProperties(
		req.GetArgs(), plugin.MarshalOptions{Label: label, KeepUnknowns: true})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal %v args", tok)
	}

	// Dispatch request for resource outputs to builtin provider.
	ret, failures, err := rm.builtins.Invoke(tok, args)
	if err != nil {
		return nil, errors.Wrapf(err, "invoke %s failed", tok)
	}

	mret, err := plugin.MarshalProperties(ret, plugin.MarshalOptions{Label: label, KeepUnknowns: true})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal return")
	}

	var chkfails []*pulumirpc.CheckFailure
	for _, failure := range failures {
		chkfails = append(chkfails, &pulumirpc.CheckFailure{
			Property: string(failure.Property),
			Reason:   failure.Reason,
		})
	}

	return &pulumirpc.InvokeResponse{Return: mret, Failures: chkfails}, nil
}

// ReadResource reads the current state associated with a resource from its provider plugin.
func (rm *queryResmon) ReadResource(ctx context.Context,
	req *pulumirpc.ReadResourceRequest) (*pulumirpc.ReadResourceResponse, error) {

	return nil, fmt.Errorf("Query mode does not support reading resources")
}

// RegisterResource is invoked by a language process when a new resource has been allocated.
func (rm *queryResmon) RegisterResource(ctx context.Context,
	req *pulumirpc.RegisterResourceRequest) (*pulumirpc.RegisterResourceResponse, error) {

	return nil, fmt.Errorf("Query mode does not support creating, updating, or deleting resources")
}

// RegisterResourceOutputs records some new output properties for a resource that have arrived after its initial
// provisioning.  These will make their way into the eventual checkpoint state file for that resource.
func (rm *queryResmon) RegisterResourceOutputs(ctx context.Context,
	req *pulumirpc.RegisterResourceOutputsRequest) (*pbempty.Empty, error) {

	return nil, fmt.Errorf("Query mode does not support registering resource operations")
}

// SupportsFeature the query resmon is able to have secrets passed to it, which may be arguments to invoke calls.
func (rm *queryResmon) SupportsFeature(ctx context.Context,
	req *pulumirpc.SupportsFeatureRequest) (*pulumirpc.SupportsFeatureResponse, error) {

	hasSupport := false
	return &pulumirpc.SupportsFeatureResponse{
		HasSupport: hasSupport,
	}, nil
}
