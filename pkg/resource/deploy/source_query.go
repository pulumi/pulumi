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

package deploy

import (
	"context"
	"errors"
	"fmt"
	"math"

	opentracing "github.com/opentracing/opentracing-go"
	"google.golang.org/protobuf/types/known/emptypb"

	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// QuerySource is used to synchronously wait for a query result.
type QuerySource interface {
	Wait() error
}

// NewQuerySource creates a `QuerySource` for some target runtime environment specified by
// `runinfo`, and supported by language plugins provided in `plugctx`.
func NewQuerySource(cancel context.Context, plugctx *plugin.Context, client BackendClient,
	runinfo *EvalRunInfo, defaultProviderVersions map[tokens.Package]workspace.PluginSpec,
	provs ProviderSource,
) (QuerySource, error) {
	// Create a new builtin provider. This provider implements features such as `getStack`.
	builtins := newBuiltinProvider(
		client,
		nil, /*news*/
		nil, /*reads*/
		plugctx.Diag,
	)

	reg := providers.NewRegistry(plugctx.Host, false, builtins)

	// Allows queryResmon to communicate errors loading providers.
	providerRegErrChan := make(chan error)

	// First, fire up a resource monitor that will disallow all resource operations, as well as
	// service calls for things like resource ouptuts of state snapshots.
	//
	// NOTE: Using the queryResourceMonitor here is *VERY* important, as its job is to disallow
	// resource operations in query mode!
	mon, err := newQueryResourceMonitor(builtins, defaultProviderVersions, provs, reg, plugctx,
		providerRegErrChan, opentracing.SpanFromContext(cancel), runinfo)
	if err != nil {
		return nil, fmt.Errorf("failed to start resource monitor: %w", err)
	}

	// Create a new iterator with appropriate channels, and gear up to go!
	src := &querySource{
		mon:                mon,
		plugctx:            plugctx,
		runinfo:            runinfo,
		runLangPlugin:      runLangPlugin,
		langPluginFinChan:  make(chan error),
		providerRegErrChan: make(chan error),
		cancel:             cancel,
	}

	// Now invoke Run in a goroutine.  All subsequent resource creation events will come in over the gRPC channel,
	// and we will pump them through the channel.  If the Run call ultimately fails, we need to propagate the error.
	src.forkRun()

	// Finally, return the fresh iterator that the caller can use to take things from here.
	return src, nil
}

type querySource struct {
	mon                SourceResourceMonitor    // the resource monitor, per iterator.
	plugctx            *plugin.Context          // the plugin context.
	runinfo            *EvalRunInfo             // the directives to use when running the program.
	runLangPlugin      func(*querySource) error // runs the language plugin.
	langPluginFinChan  chan error               // communicates language plugin completion.
	providerRegErrChan chan error               // communicates errors loading providers
	done               bool                     // set to true when the evaluation is done.
	res                error                    // result when the channel is finished.
	cancel             context.Context
}

func (src *querySource) Close() error {
	// Cancel the monitor and reclaim any associated resources.
	src.done = true
	return src.mon.Cancel()
}

func (src *querySource) Wait() error {
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

func runLangPlugin(src *querySource) error {
	rt := src.runinfo.Proj.Runtime.Name()
	rtopts := src.runinfo.Proj.Runtime.Options()
	programInfo := plugin.NewProgramInfo(
		/* rootDirectory */ src.runinfo.ProjectRoot,
		/* programDirectory */ src.runinfo.Pwd,
		/* entryPoint */ src.runinfo.Program,
		/* options */ rtopts)
	langhost, err := src.plugctx.Host.LanguageRuntime(rt, programInfo)
	if err != nil {
		return fmt.Errorf("failed to launch language host %s: %w", rt, err)
	}
	contract.Assertf(langhost != nil, "expected non-nil language host %s", rt)

	// Decrypt the configuration.
	var config map[config.Key]string
	if src.runinfo.Target != nil {
		config, err = src.runinfo.Target.Config.Decrypt(src.runinfo.Target.Decrypter)
		if err != nil {
			return err
		}
	}

	var name, organization string
	if src.runinfo.Target != nil {
		name = src.runinfo.Target.Name.String()
		organization = string(src.runinfo.Target.Organization)
	}

	// Now run the actual program.
	progerr, bail, err := langhost.Run(plugin.RunInfo{
		MonitorAddress: src.mon.Address(),
		Stack:          name,
		Project:        string(src.runinfo.Proj.Name),
		Pwd:            src.runinfo.Pwd,
		Args:           src.runinfo.Args,
		Config:         config,
		DryRun:         true,
		QueryMode:      true,
		Parallel:       math.MaxInt32,
		Organization:   organization,
		Info:           programInfo,
	})

	// Check if we were asked to Bail.  This a special random constant used for that
	// purpose.
	if err == nil && bail {
		return result.BailErrorf("run bailed")
	}

	if err == nil && progerr != "" {
		// If the program had an unhandled error; propagate it to the caller.
		err = fmt.Errorf("an unhandled error occurred: %v", progerr)
	}
	return err
}

// newQueryResourceMonitor creates a new resource monitor RPC server intended to be used in Pulumi's
// "query mode".
func newQueryResourceMonitor(
	builtins *builtinProvider, defaultProviderInfo map[tokens.Package]workspace.PluginSpec,
	provs ProviderSource, reg *providers.Registry, plugctx *plugin.Context,
	providerRegErrChan chan<- error, tracingSpan opentracing.Span, runinfo *EvalRunInfo,
) (*queryResmon, error) {
	// Create our cancellation channel.
	cancel := make(chan bool)

	// Create channel for handling registrations.
	providerRegChan := make(chan *registerResourceEvent)

	// Create a new default provider manager.
	d := &defaultProviders{
		defaultProviderInfo: defaultProviderInfo,
		providers:           make(map[string]providers.Reference),
		config:              runinfo.Target,
		requests:            make(chan defaultProviderRequest),
		providerRegChan:     providerRegChan,
		cancel:              cancel,
	}

	go func() {
		for e := range providerRegChan {
			urn := syntheticProviderURN(e.goal)

			checkResponse, err := reg.Check(context.TODO(), plugin.CheckRequest{
				URN:          urn,
				Olds:         resource.PropertyMap{},
				News:         e.goal.Properties,
				Organization: string(runinfo.Target.Organization),
			})
			if err != nil {
				providerRegErrChan <- err
				return
			}
			resp, err := reg.Create(context.TODO(), plugin.CreateRequest{
				URN:        urn,
				Properties: checkResponse.Properties,
				Timeout:    9999,
			})
			if err != nil {
				providerRegErrChan <- err
				return
			}

			contract.Assertf(resp.ID != "", "expected non-empty provider ID")
			contract.Assertf(resp.ID != providers.UnknownID, "expected non-unknown provider ID")

			e.done <- &RegisterResult{State: &resource.State{
				Type: e.goal.Type,
				URN:  urn,
				ID:   resp.ID,
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
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: queryResmon.cancel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceMonitorServer(srv, queryResmon)
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(tracingSpan),
	})
	if err != nil {
		return nil, err
	}

	monitorAddress := fmt.Sprintf("127.0.0.1:%d", handle.Port)

	var config map[config.Key]string
	if runinfo.Target != nil {
		config, err = runinfo.Target.Config.Decrypt(runinfo.Target.Decrypter)
		if err != nil {
			return nil, err
		}
	}

	var name string
	if runinfo.Target != nil {
		name = runinfo.Target.Name.String()
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
	queryResmon.done = handle.Done

	go d.serve()

	return queryResmon, nil
}

// queryResmon is a pulumirpc.ResourceMonitor that is meant to run in Pulumi's "query mode". It
// performs two critical functions:
//
//  1. Disallows all resource operations. `queryResmon` intercepts all resource operations and
//     returns an error instead of allowing them to proceed.
//  2. Services requests for stack snapshots. This is primarily to allow us to allow queries across
//     stack snapshots.
type queryResmon struct {
	pulumirpc.UnimplementedResourceMonitorServer

	builtins         *builtinProvider    // provides builtins such as `getStack`.
	providers        ProviderSource      // the provider source itself.
	defaultProviders *defaultProviders   // the default provider manager.
	addr             string              // the address the host is listening on.
	cancel           chan bool           // a channel that can cancel the server.
	done             <-chan error        // a channel that resolves when the server completes.
	reg              *providers.Registry // registry for resource providers.
	callInfo         plugin.CallInfo     // information for call calls.
}

var _ SourceResourceMonitor = (*queryResmon)(nil)

// Query doesn't do anything with the abort channel, so we just construct a new one that we'll never send anything to.
func (rm *queryResmon) AbortChan() <-chan bool {
	return make(<-chan bool)
}

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

// getProviderReference fetches the provider reference for a resource, read, or invoke from the given package with the
// given unparsed provider reference. If the unparsed provider reference is empty, this function returns a reference
// to the default provider for the indicated package.
func getProviderReference(defaultProviders *defaultProviders, req providers.ProviderRequest,
	rawProviderRef string,
) (providers.Reference, error) {
	if rawProviderRef != "" {
		ref, err := providers.ParseReference(rawProviderRef)
		if err != nil {
			return providers.Reference{}, fmt.Errorf("could not parse provider reference: %w", err)
		}
		return ref, nil
	}

	ref, err := defaultProviders.getDefaultProviderRef(req)
	if err != nil {
		return providers.Reference{}, err
	}
	return ref, nil
}

// getProviderFromSource fetches the provider plugin for a resource, read, or invoke from the given
// package with the given unparsed provider reference. If the unparsed provider reference is empty,
// this function returns the plugin for the indicated package's default provider.
func getProviderFromSource(
	providerSource ProviderSource, defaultProviders *defaultProviders,
	req providers.ProviderRequest, rawProviderRef string,
	token tokens.ModuleMember,
) (plugin.Provider, error) {
	providerRef, err := getProviderReference(defaultProviders, req, rawProviderRef)
	if err != nil {
		return nil, fmt.Errorf("getProviderFromSource: %w", err)
	} else if providers.IsDenyDefaultsProvider(providerRef) {
		msg := diag.GetDefaultProviderDenied("Invoke").Message
		return nil, fmt.Errorf(msg, req.Package(), token)
	}

	provider, ok := providerSource.GetProvider(providerRef)
	if !ok {
		return nil, fmt.Errorf("unknown provider '%v' -> '%v'", rawProviderRef, providerRef)
	}
	return provider, nil
}

// Invoke performs an invocation of a member located in a resource provider.
func (rm *queryResmon) Invoke(
	ctx context.Context, req *pulumirpc.ResourceInvokeRequest,
) (*pulumirpc.InvokeResponse, error) {
	tok := tokens.ModuleMember(req.GetTok())
	label := fmt.Sprintf("QueryResourceMonitor.Invoke(%s)", tok)

	providerReq, err := parseProviderRequest(
		tok.Package(), req.GetVersion(),
		req.GetPluginDownloadURL(), req.GetPluginChecksums(), nil)
	if err != nil {
		return nil, err
	}
	prov, err := getProviderFromSource(rm.reg, rm.defaultProviders, providerReq, req.GetProvider(), tok)
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
		return nil, fmt.Errorf("failed to unmarshal %v args: %w", tok, err)
	}

	// Do the invoke and then return the arguments.
	logging.V(5).Infof("QueryResourceMonitor.Invoke received: tok=%v #args=%v", tok, len(args))
	resp, err := prov.Invoke(ctx, plugin.InvokeRequest{
		Tok:  tok,
		Args: args,
	})
	if err != nil {
		return nil, fmt.Errorf("invocation of %v returned an error: %w", tok, err)
	}
	mret, err := plugin.MarshalProperties(resp.Properties, plugin.MarshalOptions{
		Label:         label,
		KeepUnknowns:  true,
		KeepResources: req.GetAcceptResources(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal return: %w", err)
	}

	chkfails := slice.Prealloc[*pulumirpc.CheckFailure](len(resp.Failures))
	for _, failure := range resp.Failures {
		chkfails = append(chkfails, &pulumirpc.CheckFailure{
			Property: string(failure.Property),
			Reason:   failure.Reason,
		})
	}

	return &pulumirpc.InvokeResponse{Return: mret, Failures: chkfails}, nil
}

func (rm *queryResmon) StreamInvoke(
	req *pulumirpc.ResourceInvokeRequest, stream pulumirpc.ResourceMonitor_StreamInvokeServer,
) error {
	tok := tokens.ModuleMember(req.GetTok())
	label := fmt.Sprintf("QueryResourceMonitor.StreamInvoke(%s)", tok)

	providerReq, err := parseProviderRequest(
		tok.Package(), req.GetVersion(),
		req.GetPluginDownloadURL(), req.GetPluginChecksums(), nil)
	if err != nil {
		return err
	}
	prov, err := getProviderFromSource(rm.reg, rm.defaultProviders, providerReq, req.GetProvider(), tok)
	if err != nil {
		return err
	}

	args, err := plugin.UnmarshalProperties(
		req.GetArgs(), plugin.MarshalOptions{Label: label, KeepUnknowns: true})
	if err != nil {
		return fmt.Errorf("failed to unmarshal %v args: %w", tok, err)
	}

	// Synchronously do the StreamInvoke and then return the arguments. This will block until the
	// streaming operation completes!
	logging.V(5).Infof("QueryResourceMonitor.StreamInvoke received: tok=%v #args=%v", tok, len(args))
	resp, err := prov.StreamInvoke(context.TODO(), plugin.StreamInvokeRequest{
		Tok:  tok,
		Args: args,
		OnNext: func(event resource.PropertyMap) error {
			mret, err := plugin.MarshalProperties(event, plugin.MarshalOptions{
				Label:         label,
				KeepUnknowns:  true,
				KeepResources: req.GetAcceptResources(),
			})
			if err != nil {
				return fmt.Errorf("failed to marshal return: %w", err)
			}

			return stream.Send(&pulumirpc.InvokeResponse{Return: mret})
		},
	})
	if err != nil {
		return fmt.Errorf("streaming invocation of %v returned an error: %w", tok, err)
	}

	chkfails := slice.Prealloc[*pulumirpc.CheckFailure](len(resp.Failures))
	for _, failure := range resp.Failures {
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
func (rm *queryResmon) Call(ctx context.Context, req *pulumirpc.ResourceCallRequest) (*pulumirpc.CallResponse, error) {
	tok := tokens.ModuleMember(req.GetTok())
	label := fmt.Sprintf("QueryResourceMonitor.Call(%s)", tok)

	providerReq, err := parseProviderRequest(
		tok.Package(), req.GetVersion(),
		req.GetPluginDownloadURL(), req.GetPluginChecksums(), nil)
	if err != nil {
		return nil, err
	}
	prov, err := getProviderFromSource(rm.reg, rm.defaultProviders, providerReq, req.GetProvider(), tok)
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
		return nil, fmt.Errorf("failed to unmarshal %v args: %w", tok, err)
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
	ret, err := prov.Call(ctx, plugin.CallRequest{
		Tok:     tok,
		Args:    args,
		Info:    rm.callInfo,
		Options: options,
	})
	if err != nil {
		return nil, fmt.Errorf("call of %v returned an error: %w", tok, err)
	}
	mret, err := plugin.MarshalProperties(ret.Return, plugin.MarshalOptions{
		Label:         label,
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal return: %w", err)
	}

	returnDependencies := map[string]*pulumirpc.CallResponse_ReturnDependencies{}
	for name, deps := range ret.ReturnDependencies {
		urns := make([]string, len(deps))
		for i, urn := range deps {
			urns[i] = string(urn)
		}
		returnDependencies[string(name)] = &pulumirpc.CallResponse_ReturnDependencies{Urns: urns}
	}

	chkfails := slice.Prealloc[*pulumirpc.CheckFailure](len(ret.Failures))
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
	req *pulumirpc.ReadResourceRequest,
) (*pulumirpc.ReadResourceResponse, error) {
	return nil, errors.New("Query mode does not support reading resources")
}

// RegisterResource is invoked by a language process when a new resource has been allocated.
func (rm *queryResmon) RegisterResource(ctx context.Context,
	req *pulumirpc.RegisterResourceRequest,
) (*pulumirpc.RegisterResourceResponse, error) {
	return nil, errors.New("Query mode does not support creating, updating, or deleting resources")
}

// RegisterResourceOutputs records some new output properties for a resource that have arrived after its initial
// provisioning.  These will make their way into the eventual checkpoint state file for that resource.
func (rm *queryResmon) RegisterResourceOutputs(ctx context.Context,
	req *pulumirpc.RegisterResourceOutputsRequest,
) (*emptypb.Empty, error) {
	return nil, errors.New("Query mode does not support registering resource operations")
}

// SupportsFeature the query resmon is able to have secrets passed to it, which may be arguments to invoke calls.
func (rm *queryResmon) SupportsFeature(ctx context.Context,
	req *pulumirpc.SupportsFeatureRequest,
) (*pulumirpc.SupportsFeatureResponse, error) {
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
