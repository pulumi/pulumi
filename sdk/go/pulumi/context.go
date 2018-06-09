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
	// Validate some properties.
	if info.Project == "" {
		return nil, errors.New("missing project name")
	}
	if info.Stack == "" {
		return nil, errors.New("missing stack name")
	}
	if info.MonitorAddr == "" {
		return nil, errors.New("missing resource monitor RPC address")
	}
	if info.EngineAddr == "" {
		return nil, errors.New("missing engine RPC address")
	}

	monitorConn, err := grpc.Dial(info.MonitorAddr, grpc.WithInsecure())
	if err != nil {
		return nil, errors.Wrap(err, "connecting to resource monitor over RPC")
	}

	engineConn, err := grpc.Dial(info.EngineAddr, grpc.WithInsecure())
	if err != nil {
		return nil, errors.Wrap(err, "connecting to engine over RPC")
	}

	mutex := &sync.Mutex{}
	return &Context{
		ctx:         ctx,
		info:        info,
		exports:     make(map[string]interface{}),
		monitorConn: monitorConn,
		monitor:     pulumirpc.NewResourceMonitorClient(monitorConn),
		engineConn:  engineConn,
		engine:      pulumirpc.NewEngineClient(engineConn),
		rpcs:        0,
		rpcsLock:    mutex,
		rpcsDone:    sync.NewCond(mutex),
	}, nil
}

// Close implements io.Closer and relinquishes any outstanding resources held by the context.
func (ctx *Context) Close() error {
	if err := ctx.engineConn.Close(); err != nil {
		return err
	}
	if err := ctx.monitorConn.Close(); err != nil {
		return err
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

// Invoke will invoke a provider's function, identified by its token tok.
func (ctx *Context) Invoke(tok string, args map[string]interface{}) (Outputs, error) {
	// TODO(joe): implement this.
	return nil, errors.New("Invoke not yet implemented")
}

// ReadResource reads an existing custom resource's state from the resource monitor.  Note that resources read in this
// way will not be part of the resulting stack's state, as they are presumed to belong to another.
func (ctx *Context) ReadResource(
	t, name string, id ID, state map[string]interface{}, opts ...ResourceOpt) (*ResourceState, error) {
	if t == "" {
		return nil, errors.New("resource type argument cannot be empty")
	} else if name == "" {
		return nil, errors.New("resource name argument (for URN creation) cannot be empty")
	} else if id == "" {
		return nil, errors.New("resource ID is required for lookup and cannot be empty")
	}

	return nil, errors.New("ReadResource not yet implemented")
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

	// Get the parent and dependency URNs from the options, in addition to the protection bit.  If there wasn't an
	// explicit parent, and a root stack resource exists, we will automatically parent to that.
	parentURN, optDepURNs, protect := ctx.getOpts(opts...)

	// Serialize all properties, first by awaiting them, and then marshaling them to the requisite gRPC values.
	keys, rpcProps, rpcDepURNs, err := marshalInputs(props)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling properties")
	}

	// Merge all dependencies with what we got earlier from property marshaling, and remove duplicates.
	var depURNs []string
	depMap := make(map[URN]bool)
	for _, dep := range append(optDepURNs, rpcDepURNs...) {
		if _, has := depMap[dep]; !has {
			depURNs = append(depURNs, string(dep))
			depMap[dep] = true
		}
	}
	sort.Strings(depURNs)

	// Create a set of resolvers that we'll use to finalize state, for URNs, IDs, and output properties.
	urn, resolveURN, rejectURN := NewOutput(nil)

	var id *Output
	var resolveID func(interface{})
	var rejectID func(error)
	if custom {
		id, resolveID, rejectID = NewOutput(nil)
	}

	state := make(map[string]*Output)
	resolveState := make(map[string]func(interface{}))
	rejectState := make(map[string]func(error))
	for _, key := range keys {
		state[key], resolveState[key], rejectState[key] = NewOutput(nil)
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
			Parent:       string(parentURN),
			Object:       rpcProps,
			Custom:       custom,
			Protect:      protect,
			Dependencies: depURNs,
		})
		var outprops map[string]interface{}
		if err == nil {
			outprops, err = unmarshalOutputs(resp.Object)
		}
		if err != nil {
			glog.V(9).Infof("RegisterResource(%s, %s): error: %v", t, name, err)
			rejectURN(err)
			if rejectID != nil {
				rejectID(err)
			}
			for _, reject := range rejectState {
				reject(err)
			}
		} else {
			glog.V(9).Infof("RegisterResource(%s, %s): success: %s %s %d", t, name, resp.Urn, resp.Id, len(outprops))
			resolveURN(URN(resp.Urn))
			if resolveID != nil {
				resolveID(ID(resp.Id))
			}
			for _, key := range keys {
				out, err := unmarshalOutput(outprops[key])
				if err != nil {
					rejectState[key](err)
				} else {
					resolveState[key](out)
				}
			}
		}

		// Signal the completion of this RPC and notify any potential awaiters.
		ctx.endRPC()
	}()

	return &ResourceState{
		URN:   urn,
		ID:    id,
		State: state,
	}, nil
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
	URN *Output
	// ID will resolve to the resource's ID after registration, provided this is for a custom resource.
	ID *Output
	// State contains the full set of expected output properties and will resolve after completion.
	State Outputs
}

// RegisterResourceOutputs completes the resource registration, attaching an optional set of computed outputs.
func (ctx *Context) RegisterResourceOutputs(urn URN, outs map[string]interface{}) error {
	return nil
}

// Export registers a key and value pair with the current context's stack.
func (ctx *Context) Export(name string, value interface{}) {
	ctx.exports[name] = value
}
