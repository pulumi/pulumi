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

	"github.com/golang/glog"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

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
func (ctx *Context) Invoke(tok string, args map[string]interface{}) (map[string]interface{}, error) {
	if tok == "" {
		return nil, errors.New("invoke token must not be empty")
	}

	// Serialize arguments, first by awaiting them, and then marshaling them to the requisite gRPC values.
	// TODO[pulumi/pulumi#1483]: feels like we should be propagating dependencies to the outputs, instead of ignoring.
	_, rpcArgs, _, err := marshalInputs(args)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling arguments")
	}

	// Note that we're about to make an outstanding RPC request, so that we can rendezvous during shutdown.
	if err = ctx.beginRPC(); err != nil {
		return nil, err
	}
	defer ctx.endRPC()

	// Now, invoke the RPC to the provider synchronously.
	glog.V(9).Infof("Invoke(%s, #args=%d): RPC call being made synchronously", tok, len(args))
	resp, err := ctx.monitor.Invoke(ctx.ctx, &pulumirpc.InvokeRequest{
		Tok:  tok,
		Args: rpcArgs,
	})
	if err != nil {
		glog.V(9).Infof("Invoke(%s, ...): error: %v", tok, err)
		return nil, err
	}

	// If there were any failures from the provider, return them.
	if len(resp.Failures) > 0 {
		glog.V(9).Infof("Invoke(%s, ...): success: w/ %d failures", tok, len(resp.Failures))
		var ferr error
		for _, failure := range resp.Failures {
			ferr = multierror.Append(ferr,
				errors.Errorf("%s invoke failed: %s (%s)", tok, failure.Reason, failure.Property))
		}
		return nil, ferr
	}

	// Otherwsie, simply unmarshal the output properties and return the result.
	outs, err := unmarshalOutputs(resp.Return)
	glog.V(9).Infof("Invoke(%s, ...): success: w/ %d outs (err=%v)", tok, len(outs), err)
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

	// Prepare the inputs for an impending operation.
	op, err := ctx.newResourceOperation(true, props, opts...)
	if err != nil {
		return nil, err
	}

	// Note that we're about to make an outstanding RPC request, so that we can rendezvous during shutdown.
	if err = ctx.beginRPC(); err != nil {
		return nil, err
	}

	// Kick off the resource read operation.  This will happen asynchronously and resolve the above properties.
	go func() {
		glog.V(9).Infof("ReadResource(%s, %s): Goroutine spawned, RPC call being made", t, name)
		resp, err := ctx.monitor.ReadResource(ctx.ctx, &pulumirpc.ReadResourceRequest{
			Type:       t,
			Name:       name,
			Parent:     op.parent,
			Properties: op.rpcProps,
		})
		if err != nil {
			glog.V(9).Infof("RegisterResource(%s, %s): error: %v", t, name, err)
		} else {
			glog.V(9).Infof("RegisterResource(%s, %s): success: %s %s ...", t, name, resp.Urn, id)
		}

		// No matter the outcome, make sure all promises are resolved.
		var urn, resID string
		var props *structpb.Struct
		if resp != nil {
			urn, resID = resp.Urn, string(id)
			props = resp.Properties
		}
		op.complete(err, urn, resID, props)

		// Signal the completion of this RPC and notify any potential awaiters.
		ctx.endRPC()
	}()

	outs := make(map[string]*Output)
	for k, s := range op.outState {
		outs[k] = s.out
	}
	return &ResourceState{
		URN:   (*URNOutput)(op.outURN.out),
		ID:    (*IDOutput)(op.outID.out),
		State: outs,
	}, nil
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

	// Prepare the inputs for an impending operation.
	op, err := ctx.newResourceOperation(custom, props, opts...)
	if err != nil {
		return nil, err
	}

	// Note that we're about to make an outstanding RPC request, so that we can rendezvous during shutdown.
	if err = ctx.beginRPC(); err != nil {
		return nil, err
	}

	// Kick off the resource registration.  If we are actually performing a deployment, the resulting properties
	// will be resolved asynchronously as the RPC operation completes.  If we're just planning, values won't resolve.
	go func() {
		glog.V(9).Infof("RegisterResource(%s, %s): Goroutine spawned, RPC call being made", t, name)
		resp, err := ctx.monitor.RegisterResource(ctx.ctx, &pulumirpc.RegisterResourceRequest{
			Type:         t,
			Name:         name,
			Parent:       op.parent,
			Object:       op.rpcProps,
			Custom:       custom,
			Protect:      op.protect,
			Dependencies: op.deps,
		})
		if err != nil {
			glog.V(9).Infof("RegisterResource(%s, %s): error: %v", t, name, err)
		} else {
			glog.V(9).Infof("RegisterResource(%s, %s): success: %s %s ...", t, name, resp.Urn, resp.Id)
		}

		// No matter the outcome, make sure all promises are resolved.
		var urn, resID string
		var props *structpb.Struct
		if resp != nil {
			urn, resID = resp.Urn, resp.Id
			props = resp.Object
		}
		op.complete(err, urn, resID, props)

		// Signal the completion of this RPC and notify any potential awaiters.
		ctx.endRPC()
	}()

	var id *IDOutput
	if op.outID != nil {
		id = (*IDOutput)(op.outID.out)
	}
	outs := make(map[string]*Output)
	for k, s := range op.outState {
		outs[k] = s.out
	}
	return &ResourceState{
		URN:   (*URNOutput)(op.outURN.out),
		ID:    id,
		State: outs,
	}, nil
}

// resourceOperation reflects all of the inputs necessary to perform core resource RPC operations.
type resourceOperation struct {
	ctx      *Context
	parent   string
	deps     []string
	protect  bool
	props    map[string]interface{}
	rpcProps *structpb.Struct
	outURN   *resourceOutput
	outID    *resourceOutput
	outState map[string]*resourceOutput
}

// newResourceOperation prepares the inputs for a resource operation, shared between read and register.
func (ctx *Context) newResourceOperation(custom bool, props map[string]interface{},
	opts ...ResourceOpt) (*resourceOperation, error) {
	// Get the parent and dependency URNs from the options, in addition to the protection bit.  If there wasn't an
	// explicit parent, and a root stack resource exists, we will automatically parent to that.
	parent, optDeps, protect := ctx.getOpts(opts...)

	// Serialize all properties, first by awaiting them, and then marshaling them to the requisite gRPC values.
	keys, rpcProps, rpcDeps, err := marshalInputs(props)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling properties")
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

	// Create a set of resolvers that we'll use to finalize state, for URNs, IDs, and output properties.
	outURN, resolveURN, rejectURN := NewOutput(nil)
	urn := &resourceOutput{out: outURN, resolve: resolveURN, reject: rejectURN}

	var id *resourceOutput
	if custom {
		outID, resolveID, rejectID := NewOutput(nil)
		id = &resourceOutput{out: outID, resolve: resolveID, reject: rejectID}
	}

	state := make(map[string]*resourceOutput)
	for _, key := range keys {
		outState, resolveState, rejectState := NewOutput(nil)
		state[key] = &resourceOutput{
			out:     outState,
			resolve: resolveState,
			reject:  rejectState,
		}
	}

	return &resourceOperation{
		ctx:      ctx,
		parent:   string(parent),
		deps:     deps,
		protect:  protect,
		props:    props,
		rpcProps: rpcProps,
		outURN:   urn,
		outID:    id,
		outState: state,
	}, nil
}

// complete finishes a resource operation given the set of RPC results.
func (op *resourceOperation) complete(err error, urn string, id string, result *structpb.Struct) {
	var outprops map[string]interface{}
	if err == nil {
		outprops, err = unmarshalOutputs(result)
	}
	if err != nil {
		// If there was an error, we must reject everything: URN, ID, and state properties.
		op.outURN.reject(err)
		if op.outID != nil {
			op.outID.reject(err)
		}
		for _, s := range op.outState {
			s.reject(err)
		}
	} else {
		// Resolve the URN and ID.
		op.outURN.resolve(URN(urn), true)
		if op.outID != nil {
			if id == "" && op.ctx.DryRun() {
				op.outID.resolve("", false)
			} else {
				op.outID.resolve(ID(id), true)
			}
		}

		// During previews, it's possible that nils will be returned due to unknown values.  This function
		// determines the known-ed-ness of a given value below.
		isKnown := func(v interface{}) bool {
			return !op.ctx.DryRun() || v != nil
		}

		// Now resolve all output properties.
		seen := make(map[string]bool)
		for k, v := range outprops {
			if s, has := op.outState[k]; has {
				s.resolve(v, isKnown(v))
				seen[k] = true
			}
		}

		// If we didn't get back any inputs as outputs, resolve them to the inputs.
		for k, s := range op.outState {
			if !seen[k] {
				v := op.props[k]
				s.resolve(v, isKnown(v))
			}
		}
	}
}

type resourceOutput struct {
	out     *Output
	resolve func(interface{}, bool)
	reject  func(error)
}

// getOpts returns a set of resource options from an array of them.  This includes the parent URN, any
// dependency URNs, and a boolean indicating whether the resource is to be protected.
func (ctx *Context) getOpts(opts ...ResourceOpt) (URN, []URN, bool) {
	return ctx.getOptsParentURN(opts...),
		ctx.getOptsDepURNs(opts...),
		ctx.getOptsProtect(opts...)
}

// getOptsParentURN returns a URN to use for a resource, given its options, defaulting to the current stack resource.
func (ctx *Context) getOptsParentURN(opts ...ResourceOpt) URN {
	for _, opt := range opts {
		if opt.Parent != nil {
			return opt.Parent.URN()
		}
	}
	return ctx.stackR
}

// getOptsDepURNs returns the set of dependency URNs in a resource's options.
func (ctx *Context) getOptsDepURNs(opts ...ResourceOpt) []URN {
	var urns []URN
	for _, opt := range opts {
		for _, dep := range opt.DependsOn {
			urns = append(urns, dep.URN())
		}
	}
	return urns
}

// getOptsProtect returns true if a resource's options indicate that it is to be protected.
func (ctx *Context) getOptsProtect(opts ...ResourceOpt) bool {
	for _, opt := range opts {
		if opt.Protect {
			return true
		}
	}
	return false
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

// ResourceState contains the results of a resource registration operation.
type ResourceState struct {
	// URN will resolve to the resource's URN after registration has completed.
	URN *URNOutput
	// ID will resolve to the resource's ID after registration, provided this is for a custom resource.
	ID *IDOutput
	// State contains the full set of expected output properties and will resolve after completion.
	State Outputs
}

// RegisterResourceOutputs completes the resource registration, attaching an optional set of computed outputs.
func (ctx *Context) RegisterResourceOutputs(urn URN, outs map[string]interface{}) error {
	_, outsMarshalled, _, err := marshalInputs(outs)
	if err != nil {
		return errors.Wrap(err, "marshaling outputs")
	}

	// Note that we're about to make an outstanding RPC request, so that we can rendezvous during shutdown.
	if err = ctx.beginRPC(); err != nil {
		return err
	}

	// Register the outputs
	glog.V(9).Infof("RegisterResourceOutputs(%s): RPC call being made", urn)
	_, err = ctx.monitor.RegisterResourceOutputs(ctx.ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn:     string(urn),
		Outputs: outsMarshalled,
	})
	if err != nil {
		return errors.Wrap(err, "registering outputs")
	}

	glog.V(9).Infof("RegisterResourceOutputs(%s): success", urn)

	// Signal the completion of this RPC and notify any potential awaiters.
	ctx.endRPC()
	return nil
}

// Export registers a key and value pair with the current context's stack.
func (ctx *Context) Export(name string, value interface{}) {
	ctx.exports[name] = value
}
