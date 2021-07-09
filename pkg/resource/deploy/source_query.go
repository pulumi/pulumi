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

	"github.com/blang/semver"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// QuerySource is used to synchronously wait for a query result.
type QuerySource interface {
	Wait() result.Result
}

// NewQuerySource creates a `QuerySource` for some target runtime environment specified by
// `runinfo`, and supported by language plugins provided in `plugctx`.
func NewQuerySource(cancel context.Context, plugctx *plugin.Context, client BackendClient,
	runinfo *EvalRunInfo, defaultProviderVersions map[tokens.Package]*semver.Version,
	provs ProviderSource) (QuerySource, error) {

	// Create a new builtin provider. This provider implements features such as `getStack`.
	builtins := newBuiltinProvider(client, nil)

	reg, err := providers.NewRegistry(plugctx.Host, nil, false, builtins)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to start resource monitor")
	}

	// Allows queryResmon to communicate errors loading providers.
	providerRegErrChan := make(chan result.Result)

	// First, fire up a resource monitor that will disallow all resource operations, as well as
	// service calls for things like resource ouptuts of state snapshots.
	//
	// NOTE: Using the queryResourceMonitor here is *VERY* important, as its job is to disallow
	// resource operations in query mode!
	mon, err := newQueryResourceMonitor(builtins, defaultProviderVersions, provs, reg, plugctx,
		providerRegErrChan, opentracing.SpanFromContext(cancel), runinfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start resource monitor")
	}

	// Create a new iterator with appropriate channels, and gear up to go!
	src := &querySource{
		mon:                mon,
		plugctx:            plugctx,
		runinfo:            runinfo,
		runLangPlugin:      runLangPlugin,
		langPluginFinChan:  make(chan result.Result),
		providerRegErrChan: make(chan result.Result),
		cancel:             cancel,
	}

	// Now invoke Run in a goroutine.  All subsequent resource creation events will come in over the gRPC channel,
	// and we will pump them through the channel.  If the Run call ultimately fails, we need to propagate the error.
	src.forkRun()

	// Finally, return the fresh iterator that the caller can use to take things from here.
	return src, nil
}

type querySource struct {
	mon                SourceResourceMonitor            // the resource monitor, per iterator.
	plugctx            *plugin.Context                  // the plugin context.
	runinfo            *EvalRunInfo                     // the directives to use when running the program.
	runLangPlugin      func(*querySource) result.Result // runs the language plugin.
	langPluginFinChan  chan result.Result               // communicates language plugin completion.
	providerRegErrChan chan result.Result               // communicates errors loading providers
	done               bool                             // set to true when the evaluation is done.
	res                result.Result                    // result when the channel is finished.
	cancel             context.Context
}

func (src *querySource) Close() error {
	// Cancel the monitor and reclaim any associated resources.
	src.done = true
	return src.mon.Cancel()
}

func (src *querySource) Wait() result.Result {
	// If we are done, quit.
	if src.done {
		return src.res
	}

	select {
	case src.res = <-src.langPluginFinChan:
		// Language plugin has exited. No need to call `Close`.
		src.done = true
		return src.res
	case src.res = <-src.providerRegErrChan:
		// Provider registration has failed.
		src.Close()
		return src.res
	case <-src.cancel.Done():
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
		src.langPluginFinChan <- src.runLangPlugin(src)
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
func newQueryResourceMonitor(
	builtins *builtinProvider, defaultProviderVersions map[tokens.Package]*semver.Version,
	provs ProviderSource, reg *providers.Registry, plugctx *plugin.Context,
	providerRegErrChan chan<- result.Result, tracingSpan opentracing.Span, runinfo *EvalRunInfo) (*queryResmon, error) {

	// Create our cancellation channel.
	cancel := make(chan bool)

	// Create channel for handling registrations.
	providerRegChan := make(chan *registerResourceEvent)

	// Create a new default provider manager.
	d := &defaultProviders{
		defaultVersions: defaultProviderVersions,
		providers:       make(map[string]providers.Reference),
		config:          runinfo.Target,
		requests:        make(chan defaultProviderRequest),
		providerRegChan: providerRegChan,
		cancel:          cancel,
	}

	go func() {
		for e := range providerRegChan {
			urn := syntheticProviderURN(e.goal)

			inputs, _, err := reg.Check(urn, resource.PropertyMap{}, e.goal.Properties, false)
			if err != nil {
				providerRegErrChan <- result.FromError(err)
				return
			}
			_, _, _, err = reg.Create(urn, inputs, 9999, false)
			if err != nil {
				providerRegErrChan <- result.FromError(err)
				return
			}

			e.done <- &RegisterResult{State: &resource.State{
				Type: e.goal.Type,
				URN:  urn,
			}}
		}
	}()

	// New up an engine RPC server.
	queryResmon := &queryResmon{
		builtins:         builtins,
		providers:        provs,
		defaultProviders: d,
		cancel:           cancel,
		reg:              reg,
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

	monitorAddress := fmt.Sprintf("127.0.0.1:%d", port)

	var config map[config.Key]string
	if runinfo.Target != nil {
		config, err = runinfo.Target.Config.Decrypt(runinfo.Target.Decrypter)
		if err != nil {
			return nil, err
		}
	}

	var name string
	if runinfo.Target != nil {
		name = string(runinfo.Target.Name)
	}

	queryResmon.callInfo = plugin.CallInfo{
		Project:        string(runinfo.Proj.Name),
		Stack:          name,
		Config:         config,
		DryRun:         true,
		Parallel:       math.MaxInt32,
		MonitorAddress: monitorAddress,
	}
	queryResmon.addr = monitorAddress
	queryResmon.done = done

	go d.serve()

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
	builtins         *builtinProvider    // provides builtins such as `getStack`.
	providers        ProviderSource      // the provider source itself.
	defaultProviders *defaultProviders   // the default provider manager.
	addr             string              // the address the host is listening on.
	cancel           chan bool           // a channel that can cancel the server.
	done             chan error          // a channel that resolves when the server completes.
	reg              *providers.Registry // registry for resource providers.
	callInfo         plugin.CallInfo     // information for call calls.
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

	providerReq, err := parseProviderRequest(tok.Package(), req.GetVersion())
	if err != nil {
		return nil, err
	}
	prov, err := getProviderFromSource(rm.reg, rm.defaultProviders, providerReq, req.GetProvider())
	if err != nil {
		return nil, err
	}

	args, err := plugin.UnmarshalProperties(
		req.GetArgs(), plugin.MarshalOptions{
			Label:         label,
			KeepUnknowns:  true,
			KeepSecrets:   true,
			KeepResources: true,
		})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal %v args", tok)
	}

	// Do the invoke and then return the arguments.
	logging.V(5).Infof("QueryResourceMonitor.Invoke received: tok=%v #args=%v", tok, len(args))
	ret, failures, err := prov.Invoke(tok, args)
	if err != nil {
		return nil, errors.Wrapf(err, "invocation of %v returned an error", tok)
	}
	mret, err := plugin.MarshalProperties(ret, plugin.MarshalOptions{
		Label:         label,
		KeepUnknowns:  true,
		KeepResources: req.GetAcceptResources(),
	})
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

func (rm *queryResmon) StreamInvoke(
	req *pulumirpc.InvokeRequest, stream pulumirpc.ResourceMonitor_StreamInvokeServer) error {

	tok := tokens.ModuleMember(req.GetTok())
	label := fmt.Sprintf("QueryResourceMonitor.StreamInvoke(%s)", tok)

	providerReq, err := parseProviderRequest(tok.Package(), req.GetVersion())
	if err != nil {
		return err
	}
	prov, err := getProviderFromSource(rm.reg, rm.defaultProviders, providerReq, req.GetProvider())
	if err != nil {
		return err
	}

	args, err := plugin.UnmarshalProperties(
		req.GetArgs(), plugin.MarshalOptions{Label: label, KeepUnknowns: true})
	if err != nil {
		return errors.Wrapf(err, "failed to unmarshal %v args", tok)
	}

	// Synchronously do the StreamInvoke and then return the arguments. This will block until the
	// streaming operation completes!
	logging.V(5).Infof("QueryResourceMonitor.StreamInvoke received: tok=%v #args=%v", tok, len(args))
	failures, err := prov.StreamInvoke(tok, args, func(event resource.PropertyMap) error {
		mret, err := plugin.MarshalProperties(event, plugin.MarshalOptions{Label: label, KeepUnknowns: true})
		if err != nil {
			return errors.Wrapf(err, "failed to marshal return")
		}

		return stream.Send(&pulumirpc.InvokeResponse{Return: mret})
	})
	if err != nil {
		return errors.Wrapf(err, "streaming invocation of %v returned an error", tok)
	}

	var chkfails []*pulumirpc.CheckFailure
	for _, failure := range failures {
		chkfails = append(chkfails, &pulumirpc.CheckFailure{
			Property: string(failure.Property),
			Reason:   failure.Reason,
		})
	}

	if len(chkfails) > 0 {
		return stream.Send(&pulumirpc.InvokeResponse{Failures: chkfails})
	}
	return nil
}

// Call dynamically executes a method in the provider associated with a component resource.
func (rm *queryResmon) Call(ctx context.Context, req *pulumirpc.CallRequest) (*pulumirpc.CallResponse, error) {
	tok := tokens.ModuleMember(req.GetTok())
	label := fmt.Sprintf("QueryResourceMonitor.Call(%s)", tok)

	providerReq, err := parseProviderRequest(tok.Package(), req.GetVersion())
	if err != nil {
		return nil, err
	}
	prov, err := getProviderFromSource(rm.reg, rm.defaultProviders, providerReq, req.GetProvider())
	if err != nil {
		return nil, err
	}

	args, err := plugin.UnmarshalProperties(
		req.GetArgs(), plugin.MarshalOptions{
			Label:         label,
			KeepUnknowns:  true,
			KeepSecrets:   true,
			KeepResources: true,
		})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal %v args", tok)
	}

	argDependencies := map[resource.PropertyKey][]resource.URN{}
	for name, deps := range req.GetArgDependencies() {
		urns := make([]resource.URN, len(deps.Urns))
		for i, urn := range deps.Urns {
			urns[i] = resource.URN(urn)
		}
		argDependencies[resource.PropertyKey(name)] = urns
	}
	options := plugin.CallOptions{
		ArgDependencies: argDependencies,
	}

	// Do the call and then return the arguments.
	logging.V(5).Infof(
		"QueryResourceMonitor.Call received: tok=%v #args=%v #info=%v #options=%v", tok, len(args), rm.callInfo, options)
	ret, err := prov.Call(tok, args, rm.callInfo, options)
	if err != nil {
		return nil, errors.Wrapf(err, "call of %v returned an error", tok)
	}
	mret, err := plugin.MarshalProperties(ret.Return, plugin.MarshalOptions{
		Label:         label,
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal return")
	}

	returnDependencies := map[string]*pulumirpc.CallResponse_ReturnDependencies{}
	for name, deps := range ret.ReturnDependencies {
		urns := make([]string, len(deps))
		for i, urn := range deps {
			urns[i] = string(urn)
		}
		returnDependencies[string(name)] = &pulumirpc.CallResponse_ReturnDependencies{Urns: urns}
	}

	var chkfails []*pulumirpc.CheckFailure
	for _, failure := range ret.Failures {
		chkfails = append(chkfails, &pulumirpc.CheckFailure{
			Property: string(failure.Property),
			Reason:   failure.Reason,
		})
	}

	return &pulumirpc.CallResponse{Return: mret, ReturnDependencies: returnDependencies, Failures: chkfails}, nil
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

// syntheticProviderURN will create a "fake" URN for a resource provider in query mode. Query mode
// has no stack, no project, and no parent, so there is otherwise no way to generate a principled
// URN.
func syntheticProviderURN(goal *resource.Goal) resource.URN {
	return resource.NewURN(
		"query-stack", "query-project", "parent-type", goal.Type, goal.Name)
}
