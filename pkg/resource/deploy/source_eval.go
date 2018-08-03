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
	"fmt"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/util/rpcutil"
	"github.com/pulumi/pulumi/pkg/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/pkg/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

// EvalRunInfo provides information required to execute and deploy resources within a package.
type EvalRunInfo struct {
	Proj    *workspace.Project `json:"proj" yaml:"proj"`                         // the package metadata.
	Pwd     string             `json:"pwd" yaml:"pwd"`                           // the package's working directory.
	Program string             `json:"program" yaml:"program"`                   // the path to the program.
	Args    []string           `json:"args,omitempty" yaml:"args,omitempty"`     // any arguments to pass to the package.
	Target  *Target            `json:"target,omitempty" yaml:"target,omitempty"` // the target being deployed into.
}

// NewEvalSource returns a planning source that fetches resources by evaluating a package with a set of args and
// a confgiuration map.  This evaluation is performed using the given plugin context and may optionally use the
// given plugin host (or the default, if this is nil).  Note that closing the eval source also closes the host.
func NewEvalSource(plugctx *plugin.Context, runinfo *EvalRunInfo, dryRun bool) Source {
	return &evalSource{
		plugctx: plugctx,
		runinfo: runinfo,
		dryRun:  dryRun,
	}
}

type evalSource struct {
	plugctx *plugin.Context // the plugin context.
	runinfo *EvalRunInfo    // the directives to use when running the program.
	dryRun  bool            // true if this is a dry-run operation only.
}

func (src *evalSource) Close() error {
	return nil
}

// Project is the name of the project being run by this evaluation source.
func (src *evalSource) Project() tokens.PackageName {
	return src.runinfo.Proj.Name
}

// Stack is the name of the stack being targeted by this evaluation source.
func (src *evalSource) Stack() tokens.QName {
	return src.runinfo.Target.Name
}

func (src *evalSource) Info() interface{} { return src.runinfo }
func (src *evalSource) IsRefresh() bool   { return false }

// Iterate will spawn an evaluator coroutine and prepare to interact with it on subsequent calls to Next.
func (src *evalSource) Iterate(opts Options) (SourceIterator, error) {
	// First, fire up a resource monitor that will watch for and record resource creation.
	regChan := make(chan *registerResourceEvent)
	regOutChan := make(chan *registerResourceOutputsEvent)
	regReadChan := make(chan *readResourceEvent)
	mon, err := newResourceMonitor(src, regChan, regOutChan, regReadChan)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start resource monitor")
	}

	// Create a new iterator with appropriate channels, and gear up to go!
	iter := &evalSourceIterator{
		mon:         mon,
		src:         src,
		regChan:     regChan,
		regOutChan:  regOutChan,
		regReadChan: regReadChan,
		finChan:     make(chan error),
	}

	// Now invoke Run in a goroutine.  All subsequent resource creation events will come in over the gRPC channel,
	// and we will pump them through the channel.  If the Run call ultimately fails, we need to propagate the error.
	iter.forkRun(opts)

	// Finally, return the fresh iterator that the caller can use to take things from here.
	return iter, nil
}

type evalSourceIterator struct {
	mon         *resmon                            // the resource monitor, per iterator.
	src         *evalSource                        // the owning eval source object.
	regChan     chan *registerResourceEvent        // the channel that contains resource registrations.
	regOutChan  chan *registerResourceOutputsEvent // the channel that contains resource completions.
	regReadChan chan *readResourceEvent            // the channel that contains read resource requests.
	finChan     chan error                         // the channel that communicates completion.
	done        bool                               // set to true when the evaluation is done.
}

func (iter *evalSourceIterator) Close() error {
	// Cancel the monitor and reclaim any associated resources.
	return iter.mon.Cancel()
}

func (iter *evalSourceIterator) Next() (SourceEvent, error) {
	// If we are done, quit.
	if iter.done {
		return nil, nil
	}

	// Await the program to compute some more state and then inspect what it has to say.
	select {
	case reg := <-iter.regChan:
		contract.Assert(reg != nil)
		goal := reg.Goal()
		logging.V(5).Infof("EvalSourceIterator produced a registration: t=%v,name=%v,#props=%v",
			goal.Type, goal.Name, len(goal.Properties))
		return reg, nil
	case regOut := <-iter.regOutChan:
		contract.Assert(regOut != nil)
		logging.V(5).Infof("EvalSourceIterator produced a completion: urn=%v,#outs=%v",
			regOut.URN(), len(regOut.Outputs()))
		return regOut, nil
	case read := <-iter.regReadChan:
		contract.Assert(read != nil)
		logging.V(5).Infoln("EvalSourceIterator produced a read")
		return read, nil
	case err := <-iter.finChan:
		// If we are finished, we can safely exit.  The contract with the language provider is that this implies
		// that the language runtime has exited and so calling Close on the plugin is fine.
		iter.done = true
		if err != nil {
			logging.V(5).Infof("EvalSourceIterator ended with an error: %v", err)
		}
		return nil, err
	}
}

// forkRun performs the evaluation from a distinct goroutine.  This function blocks until it's our turn to go.
func (iter *evalSourceIterator) forkRun(opts Options) {
	// Fire up the goroutine to make the RPC invocation against the language runtime.  As this executes, calls
	// to queue things up in the resource channel will occur, and we will serve them concurrently.
	go func() {
		// Next, launch the language plugin.
		run := func() error {
			rt := iter.src.runinfo.Proj.Runtime
			langhost, err := iter.src.plugctx.Host.LanguageRuntime(rt)
			if err != nil {
				return errors.Wrapf(err, "failed to launch language host %s", rt)
			}
			contract.Assertf(langhost != nil, "expected non-nil language host %s", rt)

			// Make sure to clean up before exiting.
			defer contract.IgnoreClose(langhost)

			// Decrypt the configuration.
			config, err := iter.src.runinfo.Target.Config.Decrypt(iter.src.runinfo.Target.Decrypter)
			if err != nil {
				return err
			}

			// Now run the actual program.
			var progerr string
			progerr, err = langhost.Run(plugin.RunInfo{
				MonitorAddress: iter.mon.Address(),
				Stack:          string(iter.src.runinfo.Target.Name),
				Project:        string(iter.src.runinfo.Proj.Name),
				Pwd:            iter.src.runinfo.Pwd,
				Program:        iter.src.runinfo.Program,
				Args:           iter.src.runinfo.Args,
				Config:         config,
				DryRun:         iter.src.dryRun,
				Parallel:       opts.Parallel,
			})
			if err == nil && progerr != "" {
				// If the program had an unhandled error; propagate it to the caller.
				err = errors.Errorf("an unhandled error occurred: %v", progerr)
			}
			return err
		}

		// Communicate the error, if it exists, or nil if the program exited cleanly.
		iter.finChan <- run()
	}()
}

// resmon implements the pulumirpc.ResourceMonitor interface and acts as the gateway between a language runtime's
// evaluation of a program and the internal resource planning and deployment logic.
type resmon struct {
	src         *evalSource                        // the evaluation source.
	regChan     chan *registerResourceEvent        // the channel to send resource registrations to.
	regOutChan  chan *registerResourceOutputsEvent // the channel to send resource output registrations to.
	regReadChan chan *readResourceEvent            // the channel to send resource reads to.
	addr        string                             // the address the host is listening on.
	cancel      chan bool                          // a channel that can cancel the server.
	done        chan error                         // a channel that resolves when the server completes.
}

// newResourceMonitor creates a new resource monitor RPC server.
func newResourceMonitor(src *evalSource, regChan chan *registerResourceEvent,
	regOutChan chan *registerResourceOutputsEvent, regReadChan chan *readResourceEvent) (*resmon, error) {
	// New up an engine RPC server.
	resmon := &resmon{
		src:         src,
		regChan:     regChan,
		regOutChan:  regOutChan,
		regReadChan: regReadChan,
		cancel:      make(chan bool),
	}

	// Fire up a gRPC server and start listening for incomings.
	port, done, err := rpcutil.Serve(0, resmon.cancel, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceMonitorServer(srv, resmon)
			return nil
		},
	})
	if err != nil {
		return nil, err
	}

	resmon.addr = fmt.Sprintf("127.0.0.1:%d", port)
	resmon.done = done

	return resmon, nil
}

// Address returns the address at which the monitor's RPC server may be reached.
func (rm *resmon) Address() string {
	return rm.addr
}

// Cancel signals that the engine should be terminated, awaits its termination, and returns any errors that result.
func (rm *resmon) Cancel() error {
	close(rm.cancel)
	return <-rm.done
}

// Invoke performs an invocation of a member located in a resource provider.
func (rm *resmon) Invoke(ctx context.Context, req *pulumirpc.InvokeRequest) (*pulumirpc.InvokeResponse, error) {
	// Fetch the token and load up the resource provider.
	// TODO: we should be flowing version information about this request, but instead, we'll bind to the latest.
	tok := tokens.ModuleMember(req.GetTok())
	prov, err := rm.src.plugctx.Host.Provider(tok.Package(), nil)
	if err != nil {
		return nil, err
	} else if prov == nil {
		return nil, errors.Errorf("could not load resource provider for package '%v' from $PATH", tok.Package())
	}

	// Now unpack all of the arguments and prepare to perform the invocation.
	label := fmt.Sprintf("ResourceMonitor.Invoke(%s)", tok)
	args, err := plugin.UnmarshalProperties(
		req.GetArgs(), plugin.MarshalOptions{Label: label, KeepUnknowns: true})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal %v args", tok)
	}

	// Do the invoke and then return the arguments.
	logging.V(5).Infof("ResourceMonitor.Invoke received: tok=%v #args=%v", tok, len(args))
	ret, failures, err := prov.Invoke(tok, args)
	if err != nil {
		return nil, errors.Wrapf(err, "invocation of %v returned an error", tok)
	}
	mret, err := plugin.MarshalProperties(ret, plugin.MarshalOptions{Label: label, KeepUnknowns: true})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal %v return", tok)
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
func (rm *resmon) ReadResource(ctx context.Context,
	req *pulumirpc.ReadResourceRequest) (*pulumirpc.ReadResourceResponse, error) {
	// Read the basic inputs necessary to identify the plugin.
	t := tokens.Type(req.GetType())
	name := tokens.QName(req.GetName())
	parent := resource.URN(req.GetParent())
	prov, err := rm.src.plugctx.Host.Provider(t.Package(), nil)
	if err != nil {
		return nil, err
	} else if prov == nil {
		return nil, errors.Errorf("could not load resource provider for package '%v' from $PATH", t.Package())
	}

	id := resource.ID(req.GetId())
	label := fmt.Sprintf("ResourceMonitor.ReadResource(%s, %s, %s)", id, t, name)
	var deps []resource.URN
	for _, depURN := range req.GetDependencies() {
		deps = append(deps, resource.URN(depURN))
	}

	props, err := plugin.UnmarshalProperties(req.GetProperties(), plugin.MarshalOptions{
		Label:        label,
		KeepUnknowns: true,
	})
	if err != nil {
		return nil, err
	}

	event := &readResourceEvent{
		id:           id,
		name:         name,
		baseType:     t,
		parent:       parent,
		props:        props,
		dependencies: deps,
		done:         make(chan *ReadResult),
	}
	select {
	case rm.regReadChan <- event:
	case <-rm.cancel:
		logging.V(5).Infof("ResourceMonitor.ReadResource operation canceled, name=%s", name)
		return nil, rpcerror.New(codes.Unavailable, "resource monitor shut down while sending resource registration")
	}

	// Now block waiting for the operation to finish.
	var result *ReadResult
	select {
	case result = <-event.done:
	case <-rm.cancel:
		logging.V(5).Infof("ResourceMonitor.ReadResource operation canceled, name=%s", name)
		return nil, rpcerror.New(codes.Unavailable, "resource monitor shut down while waiting on step's done channel")
	}

	contract.Assert(result != nil)
	marshaled, err := plugin.MarshalProperties(result.State.Outputs, plugin.MarshalOptions{
		Label:        label,
		KeepUnknowns: true,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal %s return state", result.State.URN)
	}

	return &pulumirpc.ReadResourceResponse{
		Urn:        string(result.State.URN),
		Properties: marshaled,
	}, nil
}

// RegisterResource is invoked by a language process when a new resource has been allocated.
func (rm *resmon) RegisterResource(ctx context.Context,
	req *pulumirpc.RegisterResourceRequest) (*pulumirpc.RegisterResourceResponse, error) {

	// Communicate the type, name, and object information to the iterator that is awaiting us.
	t := tokens.Type(req.GetType())
	name := tokens.QName(req.GetName())
	label := fmt.Sprintf("ResourceMonitor.RegisterResource(%s,%s)", t, name)
	custom := req.GetCustom()
	parent := resource.URN(req.GetParent())
	protect := req.GetProtect()

	dependencies := []resource.URN{}
	for _, dependingURN := range req.GetDependencies() {
		dependencies = append(dependencies, resource.URN(dependingURN))
	}

	props, err := plugin.UnmarshalProperties(
		req.GetObject(), plugin.MarshalOptions{Label: label, KeepUnknowns: true, ComputeAssetHashes: true})
	if err != nil {
		return nil, err
	}

	logging.V(5).Infof(
		"ResourceMonitor.RegisterResource received: t=%v, name=%v, custom=%v, #props=%v, parent=%v, protect=%v, deps=%v",
		t, name, custom, len(props), parent, protect, dependencies)

	// Send the goal state to the engine.
	step := &registerResourceEvent{
		goal: resource.NewGoal(t, name, custom, props, parent, protect, dependencies),
		done: make(chan *RegisterResult),
	}

	select {
	case rm.regChan <- step:
	case <-rm.cancel:
		logging.V(5).Infof("ResourceMonitor.RegisterResource operation canceled, name=%s", name)
		return nil, rpcerror.New(codes.Unavailable, "resource monitor shut down while sending resource registration")
	}

	// Now block waiting for the operation to finish.
	var result *RegisterResult
	select {
	case result = <-step.done:
	case <-rm.cancel:
		logging.V(5).Infof("ResourceMonitor.RegisterResource operation canceled, name=%s", name)
		return nil, rpcerror.New(codes.Unavailable, "resource monitor shut down while waiting on step's done channel")
	}

	state := result.State
	props = state.All()
	stable := result.Stable
	var stables []string
	for _, sta := range result.Stables {
		stables = append(stables, string(sta))
	}
	logging.V(5).Infof(
		"ResourceMonitor.RegisterResource operation finished: t=%v, urn=%v, stable=%v, #stables=%v #outs=%v",
		state.Type, state.URN, stable, len(stables), len(props))

	// Finally, unpack the response into properties that we can return to the language runtime.  This mostly includes
	// an ID, URN, and defaults and output properties that will all be blitted back onto the runtime object.
	obj, err := plugin.MarshalProperties(props, plugin.MarshalOptions{Label: label, KeepUnknowns: true})
	if err != nil {
		return nil, err
	}
	return &pulumirpc.RegisterResourceResponse{
		Urn:     string(state.URN),
		Id:      string(state.ID),
		Object:  obj,
		Stable:  stable,
		Stables: stables,
	}, nil
}

// RegisterResourceOutputs records some new output properties for a resource that have arrived after its initial
// provisioning.  These will make their way into the eventual checkpoint state file for that resource.
func (rm *resmon) RegisterResourceOutputs(ctx context.Context,
	req *pulumirpc.RegisterResourceOutputsRequest) (*pbempty.Empty, error) {

	// Obtain and validate the message's inputs (a URN plus the output property map).
	urn := resource.URN(req.GetUrn())
	if urn == "" {
		return nil, errors.New("missing required URN")
	}
	label := fmt.Sprintf("ResourceMonitor.RegisterResourceOutputs(%s)", urn)
	outs, err := plugin.UnmarshalProperties(
		req.GetOutputs(), plugin.MarshalOptions{Label: label, KeepUnknowns: true, ComputeAssetHashes: true})
	if err != nil {
		return nil, errors.Wrapf(err, "cannot unmarshal output properties")
	}
	logging.V(5).Infof("ResourceMonitor.RegisterResourceOutputs received: urn=%v, #outs=%v", urn, len(outs))

	// Now send the step over to the engine to perform.
	step := &registerResourceOutputsEvent{
		urn:     urn,
		outputs: outs,
		done:    make(chan bool),
	}

	select {
	case rm.regOutChan <- step:
	case <-rm.cancel:
		logging.V(5).Infof("ResourceMonitor.RegisterResourceOutputs operation canceled, urn=%s", urn)
		return nil, rpcerror.New(codes.Unavailable, "resource monitor shut down while sending resource outputs")
	}

	// Now block waiting for the operation to finish.
	select {
	case <-step.done:
	case <-rm.cancel:
		logging.V(5).Infof("ResourceMonitor.RegisterResourceOutputs operation canceled, urn=%s", urn)
		return nil, rpcerror.New(codes.Unavailable, "resource monitor shut down while waiting on output step's done channel")
	}

	logging.V(5).Infof(
		"ResourceMonitor.RegisterResourceOutputs operation finished: urn=%v, #outs=%v", urn, len(outs))
	return &pbempty.Empty{}, nil
}

type registerResourceEvent struct {
	goal *resource.Goal       // the resource goal state produced by the iterator.
	done chan *RegisterResult // the channel to communicate with after the resource state is available.
}

var _ RegisterResourceEvent = (*registerResourceEvent)(nil)

func (g *registerResourceEvent) event() {}

func (g *registerResourceEvent) Goal() *resource.Goal {
	return g.goal
}

func (g *registerResourceEvent) Done(result *RegisterResult) {
	// Communicate the resulting state back to the RPC thread, which is parked awaiting our reply.
	g.done <- result
}

type registerResourceOutputsEvent struct {
	urn     resource.URN         // the URN to which this completion applies.
	outputs resource.PropertyMap // an optional property bag for output properties.
	done    chan bool            // the channel to communicate with after the operation completes.
}

var _ RegisterResourceOutputsEvent = (*registerResourceOutputsEvent)(nil)

func (g *registerResourceOutputsEvent) event() {}

func (g *registerResourceOutputsEvent) URN() resource.URN {
	return g.urn
}

func (g *registerResourceOutputsEvent) Outputs() resource.PropertyMap {
	return g.outputs
}

func (g *registerResourceOutputsEvent) Done() {
	// Communicate the resulting state back to the RPC thread, which is parked awaiting our reply.
	g.done <- true
}

type readResourceEvent struct {
	id           resource.ID
	name         tokens.QName
	baseType     tokens.Type
	parent       resource.URN
	props        resource.PropertyMap
	dependencies []resource.URN
	done         chan *ReadResult
}

var _ ReadResourceEvent = (*readResourceEvent)(nil)

func (g *readResourceEvent) event() {}

func (g *readResourceEvent) ID() resource.ID                  { return g.id }
func (g *readResourceEvent) Name() tokens.QName               { return g.name }
func (g *readResourceEvent) Type() tokens.Type                { return g.baseType }
func (g *readResourceEvent) Parent() resource.URN             { return g.parent }
func (g *readResourceEvent) Properties() resource.PropertyMap { return g.props }
func (g *readResourceEvent) Dependencies() []resource.URN     { return g.dependencies }
func (g *readResourceEvent) Done(result *ReadResult) {
	g.done <- result
}
