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

package pulumi

import (
	"sort"
	"sync"

	structpb "github.com/golang/protobuf/ptypes/struct"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/pkg/util/logging"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

// Context handles registration of resources and exposes metadata about the current deployment context.
type Context struct {
	ctx         context.Context
	info        RunInfo
	stackR      URN
	exports     map[string]interface{}
	monitor     pulumirpc.ResourceMonitorClient
	monitorConn *grpc.ClientConn
	engine      pulumirpc.EngineClient
	engineConn  *grpc.ClientConn
	rpcs        int         // the number of outstanding RPC requests.
	rpcsDone    *sync.Cond  // an event signaling completion of RPCs.
	rpcsLock    *sync.Mutex // a lock protecting the RPC count and event.
}

// NewContext creates a fresh run context out of the given metadata.
func NewContext(ctx context.Context, info RunInfo) (*Context, error) {
	// Connect to the gRPC endpoints if we have addresses for them.
	var monitorConn *grpc.ClientConn
	var monitor pulumirpc.ResourceMonitorClient
	if addr := info.MonitorAddr; addr != "" {
		conn, err := grpc.Dial(info.MonitorAddr, grpc.WithInsecure())
		if err != nil {
			return nil, errors.Wrap(err, "connecting to resource monitor over RPC")
		}
		monitorConn = conn
		monitor = pulumirpc.NewResourceMonitorClient(monitorConn)
	}

	var engineConn *grpc.ClientConn
	var engine pulumirpc.EngineClient
	if addr := info.EngineAddr; addr != "" {
		conn, err := grpc.Dial(info.EngineAddr, grpc.WithInsecure())
		if err != nil {
			return nil, errors.Wrap(err, "connecting to engine over RPC")
		}
		engineConn = conn
		engine = pulumirpc.NewEngineClient(engineConn)
	}

	mutex := &sync.Mutex{}
	return &Context{
		ctx:         ctx,
		info:        info,
		exports:     make(map[string]interface{}),
		monitorConn: monitorConn,
		monitor:     monitor,
		engineConn:  engineConn,
		engine:      engine,
		rpcs:        0,
		rpcsLock:    mutex,
		rpcsDone:    sync.NewCond(mutex),
	}, nil
}

// Close implements io.Closer and relinquishes any outstanding resources held by the context.
func (ctx *Context) Close() error {
	if ctx.engineConn != nil {
		if err := ctx.engineConn.Close(); err != nil {
			return err
		}
	}
	if ctx.monitorConn != nil {
		if err := ctx.monitorConn.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Project returns the current project name.
func (ctx *Context) Project() string { return ctx.info.Project }

// Stack returns the current stack name being deployed into.
func (ctx *Context) Stack() string { return ctx.info.Stack }

// Parallel returns the degree of parallelism currently being used by the engine (1 being entirely serial).
func (ctx *Context) Parallel() int { return ctx.info.Parallel }

// DryRun is true when evaluating a program for purposes of planning, instead of performing a true deployment.
func (ctx *Context) DryRun() bool { return ctx.info.DryRun }

// GetConfig returns the config value, as a string, and a bool indicating whether it exists or not.
func (ctx *Context) GetConfig(key string) (string, bool) {
	v, ok := ctx.info.Config[key]
	return v, ok
}

// Invoke will invoke a provider's function, identified by its token tok.  This function call is synchronous.
func (ctx *Context) Invoke(tok string, args map[string]interface{}, opts ...InvokeOpt) (map[string]interface{}, error) {
	if tok == "" {
		return nil, errors.New("invoke token must not be empty")
	}

	// Check for a provider option.
	var provider string
	for _, opt := range opts {
		if opt.Provider != nil {
			pr, err := ctx.resolveProviderReference(opt.Provider)
			if err != nil {
				return nil, err
			}
			provider = pr
			break
		}
	}

	// Serialize arguments, first by awaiting them, and then marshaling them to the requisite gRPC values.
	// TODO[pulumi/pulumi#1483]: feels like we should be propagating dependencies to the outputs, instead of ignoring.
	rpcArgs, _, _, err := marshalInputs(args, false)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling arguments")
	}

	// Note that we're about to make an outstanding RPC request, so that we can rendezvous during shutdown.
	if err = ctx.beginRPC(); err != nil {
		return nil, err
	}
	defer ctx.endRPC()

	// Now, invoke the RPC to the provider synchronously.
	logging.V(9).Infof("Invoke(%s, #args=%d): RPC call being made synchronously", tok, len(args))
	resp, err := ctx.monitor.Invoke(ctx.ctx, &pulumirpc.InvokeRequest{
		Tok:      tok,
		Args:     rpcArgs,
		Provider: provider,
	})
	if err != nil {
		logging.V(9).Infof("Invoke(%s, ...): error: %v", tok, err)
		return nil, err
	}

	// If there were any failures from the provider, return them.
	if len(resp.Failures) > 0 {
		logging.V(9).Infof("Invoke(%s, ...): success: w/ %d failures", tok, len(resp.Failures))
		var ferr error
		for _, failure := range resp.Failures {
			ferr = multierror.Append(ferr,
				errors.Errorf("%s invoke failed: %s (%s)", tok, failure.Reason, failure.Property))
		}
		return nil, ferr
	}

	// Otherwsie, simply unmarshal the output properties and return the result.
	outs, err := unmarshalOutputs(resp.Return)
	logging.V(9).Infof("Invoke(%s, ...): success: w/ %d outs (err=%v)", tok, len(outs), err)
	return outs, err
}

// ReadResource reads an existing custom resource's state from the resource monitor.  Note that resources read in this
// way will not be part of the resulting stack's state, as they are presumed to belong to another.
func (ctx *Context) ReadResource(
	t, name string, id ID, props map[string]interface{}, opts ...ResourceOpt) (*ResourceState, error) {
	if t == "" {
		return nil, errors.New("resource type argument cannot be empty")
	} else if name == "" {
		return nil, errors.New("resource name argument (for URN creation) cannot be empty")
	} else if id == "" {
		return nil, errors.New("resource ID is required for lookup and cannot be empty")
	}

	// Note that we're about to make an outstanding RPC request, so that we can rendezvous during shutdown.
	if err := ctx.beginRPC(); err != nil {
		return nil, err
	}

	// Create resolvers for the resource's outputs.
	res := makeResourceState(true, props)

	// Kick off the resource read operation.  This will happen asynchronously and resolve the above properties.
	go func() {
		// No matter the outcome, make sure all promises are resolved and that we've signaled completion of this RPC.
		var urn, resID string
		var state *structpb.Struct
		var err error
		defer func() {
			res.resolve(ctx.DryRun(), err, props, urn, resID, state)
			ctx.endRPC()
		}()

		// Prepare the inputs for an impending operation.
		inputs, err := ctx.prepareResourceInputs(props, opts...)
		if err != nil {
			return
		}

		logging.V(9).Infof("ReadResource(%s, %s): Goroutine spawned, RPC call being made", t, name)
		resp, err := ctx.monitor.ReadResource(ctx.ctx, &pulumirpc.ReadResourceRequest{
			Type:       t,
			Name:       name,
			Parent:     inputs.parent,
			Properties: inputs.rpcProps,
			Provider:   inputs.provider,
		})
		if err != nil {
			logging.V(9).Infof("RegisterResource(%s, %s): error: %v", t, name, err)
		} else {
			logging.V(9).Infof("RegisterResource(%s, %s): success: %s %s ...", t, name, resp.Urn, id)
		}
		if resp != nil {
			urn, resID = resp.Urn, string(id)
			state = resp.Properties
		}
	}()

	return res, nil
}

// RegisterResource creates and registers a new resource object.  t is the fully qualified type token and name is
// the "name" part to use in creating a stable and globally unique URN for the object.  state contains the goal state
// for the resource object and opts contains optional settings that govern the way the resource is created.
func (ctx *Context) RegisterResource(
	t, name string, custom bool, props map[string]interface{}, opts ...ResourceOpt) (*ResourceState, error) {
	if t == "" {
		return nil, errors.New("resource type argument cannot be empty")
	} else if name == "" {
		return nil, errors.New("resource name argument (for URN creation) cannot be empty")
	}

	// Note that we're about to make an outstanding RPC request, so that we can rendezvous during shutdown.
	if err := ctx.beginRPC(); err != nil {
		return nil, err
	}

	// Create resolvers for the resource's outputs.
	res := makeResourceState(custom, props)

	// Kick off the resource registration.  If we are actually performing a deployment, the resulting properties
	// will be resolved asynchronously as the RPC operation completes.  If we're just planning, values won't resolve.
	go func() {
		// No matter the outcome, make sure all promises are resolved and that we've signaled completion of this RPC.
		var urn, resID string
		var state *structpb.Struct
		var err error
		defer func() {
			res.resolve(ctx.DryRun(), err, props, urn, resID, state)
			ctx.endRPC()
		}()

		// Prepare the inputs for an impending operation.
		inputs, err := ctx.prepareResourceInputs(props, opts...)
		if err != nil {
			return
		}

		logging.V(9).Infof("RegisterResource(%s, %s): Goroutine spawned, RPC call being made", t, name)
		resp, err := ctx.monitor.RegisterResource(ctx.ctx, &pulumirpc.RegisterResourceRequest{
			Type:                 t,
			Name:                 name,
			Parent:               inputs.parent,
			Object:               inputs.rpcProps,
			Custom:               custom,
			Protect:              inputs.protect,
			Dependencies:         inputs.deps,
			Provider:             inputs.provider,
			PropertyDependencies: inputs.rpcPropertyDeps,
			DeleteBeforeReplace:  inputs.deleteBeforeReplace,
			ImportId:             inputs.importID,
			CustomTimeouts:       inputs.customTimeouts,
		})
		if err != nil {
			logging.V(9).Infof("RegisterResource(%s, %s): error: %v", t, name, err)
		} else {
			logging.V(9).Infof("RegisterResource(%s, %s): success: %s %s ...", t, name, resp.Urn, resp.Id)
		}
		if resp != nil {
			urn, resID = resp.Urn, resp.Id
			state = resp.Object
		}
	}()

	return res, nil
}

// ResourceState contains the results of a resource registration operation.
type ResourceState struct {
	// urn will resolve to the resource's URN after registration has completed.
	urn URNOutput
	// id will resolve to the resource's ID after registration, provided this is for a custom resource.
	id IDOutput
	// State contains the full set of expected output properties and will resolve after completion.
	State Outputs
}

// URN will resolve to the resource's URN after registration has completed.
func (state *ResourceState) URN() URNOutput {
	return state.urn
}

// ID will resolve to the resource's ID after registration, provided this is for a custom resource.
func (state *ResourceState) ID() IDOutput {
	return state.id
}

// makeResourceState creates a set of resolvers that we'll use to finalize state, for URNs, IDs, and output
// properties.
func makeResourceState(custom bool, props map[string]interface{}) *ResourceState {
	state := &ResourceState{}

	state.urn = URNOutput(newOutput(state))

	if custom {
		state.id = IDOutput(newOutput(state))
	}

	state.State = make(map[string]Output)
	for key := range props {
		state.State[key] = newOutput(state)
	}

	return state
}

// resolve resolves the resource outputs using the given error and/or values.
func (state *ResourceState) resolve(dryrun bool, err error, inputs map[string]interface{}, urn, id string,
	result *structpb.Struct) {
	var outprops map[string]interface{}
	if err == nil {
		outprops, err = unmarshalOutputs(result)
	}
	if err != nil {
		// If there was an error, we must reject everything: URN, ID, and state properties.
		state.urn.s.reject(err)
		if state.id.s != nil {
			state.id.s.reject(err)
		}
		for _, o := range state.State {
			o.s.reject(err)
		}
		return
	}

	// Resolve the URN and ID.
	state.urn.s.resolve(URN(urn), true)
	if state.id.s != nil {
		known := id != "" || !dryrun
		state.id.s.resolve(ID(id), known)
	}

	// During previews, it's possible that nils will be returned due to unknown values.  This function
	// determines the known-ness of a given value below.
	isKnown := func(v interface{}) bool {
		return !dryrun || v != nil
	}

	// Now resolve all output properties.
	for k, o := range state.State {
		v, has := outprops[k]
		if !has && !dryrun {
			// If we did not receive a value for a particular property, resolve it to the corresponding input
			// if any exists.
			v = inputs[k]
		}
		o.s.resolve(v, isKnown(v))
	}
}

// resourceInputs reflects all of the inputs necessary to perform core resource RPC operations.
type resourceInputs struct {
	parent              string
	deps                []string
	protect             bool
	provider            string
	rpcProps            *structpb.Struct
	rpcPropertyDeps     map[string]*pulumirpc.RegisterResourceRequest_PropertyDependencies
	deleteBeforeReplace bool
	importID            string
	customTimeouts      *pulumirpc.RegisterResourceRequest_CustomTimeouts
}

// prepareResourceInputs prepares the inputs for a resource operation, shared between read and register.
func (ctx *Context) prepareResourceInputs(props map[string]interface{}, opts ...ResourceOpt) (*resourceInputs, error) {
	// Get the parent and dependency URNs from the options, in addition to the protection bit.  If there wasn't an
	// explicit parent, and a root stack resource exists, we will automatically parent to that.
	parent, optDeps, protect, provider, deleteBeforeReplace, importID, err := ctx.getOpts(opts...)
	if err != nil {
		return nil, errors.Wrap(err, "resolving options")
	}

	timeouts := ctx.getTimeouts(opts...)

	// Serialize all properties, first by awaiting them, and then marshaling them to the requisite gRPC values.
	keepUnknowns := ctx.DryRun()
	rpcProps, propertyDeps, rpcDeps, err := marshalInputs(props, keepUnknowns)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling properties")
	}

	// Convert the property dependencies map for RPC and remove duplicates.
	rpcPropertyDeps := make(map[string]*pulumirpc.RegisterResourceRequest_PropertyDependencies)
	for k, deps := range propertyDeps {
		sort.Slice(deps, func(i, j int) bool { return deps[i] < deps[j] })

		urns := make([]string, 0, len(deps))
		for i, d := range deps {
			if i > 0 && urns[i-1] == string(d) {
				continue
			}
			urns = append(urns, string(d))
		}

		rpcPropertyDeps[k] = &pulumirpc.RegisterResourceRequest_PropertyDependencies{
			Urns: urns,
		}
	}

	// Merge all dependencies with what we got earlier from property marshaling, and remove duplicates.
	var deps []string
	depMap := make(map[URN]bool)
	for _, dep := range append(optDeps, rpcDeps...) {
		if _, has := depMap[dep]; !has {
			deps = append(deps, string(dep))
			depMap[dep] = true
		}
	}
	sort.Strings(deps)

	return &resourceInputs{
		parent:              string(parent),
		deps:                deps,
		protect:             protect,
		provider:            provider,
		rpcProps:            rpcProps,
		rpcPropertyDeps:     rpcPropertyDeps,
		deleteBeforeReplace: deleteBeforeReplace,
		importID:            string(importID),
		customTimeouts:      timeouts,
	}, nil
}

func (ctx *Context) getTimeouts(opts ...ResourceOpt) *pulumirpc.RegisterResourceRequest_CustomTimeouts {
	var timeouts pulumirpc.RegisterResourceRequest_CustomTimeouts
	for _, opt := range opts {
		if opt.CustomTimeouts != nil {
			timeouts.Update = opt.CustomTimeouts.Update
			timeouts.Create = opt.CustomTimeouts.Create
			timeouts.Delete = opt.CustomTimeouts.Delete
		}
	}

	return &timeouts
}

// getOpts returns a set of resource options from an array of them. This includes the parent URN, any dependency URNs,
// a boolean indicating whether the resource is to be protected, and the URN and ID of the resource's provider, if any.
func (ctx *Context) getOpts(opts ...ResourceOpt) (URN, []URN, bool, string, bool, ID, error) {
	var parent Resource
	var deps []Resource
	var protect bool
	var provider ProviderResource
	var deleteBeforeReplace bool
	var importID ID
	for _, opt := range opts {
		if parent == nil && opt.Parent != nil {
			parent = opt.Parent
		}
		if deps == nil && opt.DependsOn != nil {
			deps = opt.DependsOn
		}
		if !protect && opt.Protect {
			protect = true
		}
		if provider == nil && opt.Provider != nil {
			provider = opt.Provider
		}
		if !deleteBeforeReplace && opt.DeleteBeforeReplace {
			deleteBeforeReplace = true
		}
		if importID == "" && opt.Import != "" {
			importID = opt.Import
		}
	}

	var parentURN URN
	if parent == nil {
		parentURN = ctx.stackR
	} else {
		urn, _, err := parent.URN().await(context.TODO())
		if err != nil {
			return "", nil, false, "", false, "", err
		}
		parentURN = urn
	}

	var depURNs []URN
	if deps != nil {
		depURNs = make([]URN, len(deps))
		for i, r := range deps {
			urn, _, err := r.URN().await(context.TODO())
			if err != nil {
				return "", nil, false, "", false, "", err
			}
			depURNs[i] = urn
		}
	}

	var providerRef string
	if provider != nil {
		pr, err := ctx.resolveProviderReference(provider)
		if err != nil {
			return "", nil, false, "", false, "", err
		}
		providerRef = pr
	}

	return parentURN, depURNs, protect, providerRef, false, importID, nil
}

func (ctx *Context) resolveProviderReference(provider ProviderResource) (string, error) {
	urn, _, err := provider.URN().await(context.TODO())
	if err != nil {
		return "", err
	}
	id, known, err := provider.ID().await(context.TODO())
	if err != nil {
		return "", err
	}
	if !known {
		id = rpcTokenUnknownValue
	}
	return string(urn) + "::" + string(id), nil
}

// noMoreRPCs is a sentinel value used to stop subsequent RPCs from occurring.
const noMoreRPCs = -1

// beginRPC attempts to start a new RPC request, returning a non-nil error if no more RPCs are permitted
// (usually because the program is shutting down).
func (ctx *Context) beginRPC() error {
	ctx.rpcsLock.Lock()
	defer ctx.rpcsLock.Unlock()

	// If we're done with RPCs, return an error.
	if ctx.rpcs == noMoreRPCs {
		return errors.New("attempted illegal RPC after program completion")
	}

	ctx.rpcs++
	return nil
}

// endRPC signals the completion of an RPC and notifies any potential awaiters when outstanding RPCs hit zero.
func (ctx *Context) endRPC() {
	ctx.rpcsLock.Lock()
	defer ctx.rpcsLock.Unlock()

	ctx.rpcs--
	if ctx.rpcs == 0 {
		ctx.rpcsDone.Broadcast()
	}
}

// waitForRPCs awaits the completion of any outstanding RPCs and then leaves behind a sentinel to prevent
// any subsequent ones from starting.  This is often used during the shutdown of a program to ensure no RPCs
// go missing due to the program exiting prior to their completion.
func (ctx *Context) waitForRPCs() {
	ctx.rpcsLock.Lock()
	defer ctx.rpcsLock.Unlock()

	// Wait until the RPC count hits zero.
	for ctx.rpcs > 0 {
		ctx.rpcsDone.Wait()
	}

	// Mark the RPCs flag so that no more RPCs are permitted.
	ctx.rpcs = noMoreRPCs
}

var _ Resource = (*ResourceState)(nil)
var _ CustomResource = (*ResourceState)(nil)
var _ ComponentResource = (*ResourceState)(nil)
var _ ProviderResource = (*ResourceState)(nil)

// RegisterResourceOutputs completes the resource registration, attaching an optional set of computed outputs.
func (ctx *Context) RegisterResourceOutputs(urn URN, outs map[string]interface{}) error {
	keepUnknowns := ctx.DryRun()
	outsMarshalled, _, _, err := marshalInputs(outs, keepUnknowns)
	if err != nil {
		return errors.Wrap(err, "marshaling outputs")
	}

	// Note that we're about to make an outstanding RPC request, so that we can rendezvous during shutdown.
	if err = ctx.beginRPC(); err != nil {
		return err
	}

	// Register the outputs
	logging.V(9).Infof("RegisterResourceOutputs(%s): RPC call being made", urn)
	_, err = ctx.monitor.RegisterResourceOutputs(ctx.ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn:     string(urn),
		Outputs: outsMarshalled,
	})
	if err != nil {
		return errors.Wrap(err, "registering outputs")
	}

	logging.V(9).Infof("RegisterResourceOutputs(%s): success", urn)

	// Signal the completion of this RPC and notify any potential awaiters.
	ctx.endRPC()
	return nil
}

// Export registers a key and value pair with the current context's stack.
func (ctx *Context) Export(name string, value interface{}) {
	ctx.exports[name] = value
}
