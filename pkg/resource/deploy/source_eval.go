// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package deploy

import (
	"fmt"

	"github.com/golang/glog"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/rpcutil"
	"github.com/pulumi/pulumi/pkg/workspace"
	lumirpc "github.com/pulumi/pulumi/sdk/proto/go"
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
//
// If destroy is true, then all of the usual initialization will take place, but the state will be presented to the
// planning engine as if no new resources exist.  This will cause it to forcibly remove them.
func NewEvalSource(plugctx *plugin.Context, runinfo *EvalRunInfo, destroy bool, dryRun bool) Source {
	return &evalSource{
		plugctx: plugctx,
		runinfo: runinfo,
		destroy: destroy,
		dryRun:  dryRun,
	}
}

type evalSource struct {
	plugctx *plugin.Context // the plugin context.
	runinfo *EvalRunInfo    // the directives to use when running the program.
	destroy bool            // true if this source will trigger total destruction.
	dryRun  bool            // true if this is a dry-run operation only.
}

func (src *evalSource) Close() error {
	return nil
}

func (src *evalSource) Project() tokens.PackageName {
	return src.runinfo.Proj.Name
}

func (src *evalSource) Info() interface{} {
	return src.runinfo
}

// Iterate will spawn an evaluator coroutine and prepare to interact with it on subsequent calls to Next.
func (src *evalSource) Iterate(opts Options) (SourceIterator, error) {
	// First, fire up a resource monitor that will watch for and record resource creation.
	regChan := make(chan *registerResourceEvent)
	regOutChan := make(chan *registerResourceOutputsEvent)
	mon, err := newResourceMonitor(src, regChan, regOutChan)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start resource monitor")
	}

	// Create a new iterator with appropriate channels, and gear up to go!
	iter := &evalSourceIterator{
		mon:        mon,
		src:        src,
		regChan:    regChan,
		regOutChan: regOutChan,
		finChan:    make(chan error),
	}

	// Now invoke Run in a goroutine.  All subsequent resource creation events will come in over the gRPC channel,
	// and we will pump them through the channel.  If the Run call ultimately fails, we need to propagate the error.
	iter.forkRun(opts)

	// Finally, return the fresh iterator that the caller can use to take things from here.
	return iter, nil
}

type evalSourceIterator struct {
	mon        *resmon                            // the resource monitor, per iterator.
	src        *evalSource                        // the owning eval source object.
	regChan    chan *registerResourceEvent        // the channel that contains resource registrations.
	regOutChan chan *registerResourceOutputsEvent // the channel that contains resource completions.
	finChan    chan error                         // the channel that communicates completion.
	done       bool                               // set to true when the evaluation is done.
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

	// If we are destroying, we simply return nothing.
	if iter.src.destroy {
		return nil, nil
	}

	// Await the program to compute some more state and then inspect what it has to say.
	select {
	case reg := <-iter.regChan:
		contract.Assert(reg != nil)
		goal := reg.Goal()
		glog.V(5).Infof("EvalSourceIterator produced a registration: t=%v,name=%v,#props=%v",
			goal.Type, goal.Name, len(goal.Properties))
		return reg, nil
	case regOut := <-iter.regOutChan:
		contract.Assert(regOut != nil)
		glog.V(5).Infof("EvalSourceIterator produced a completion: urn=%v,#outs=%v",
			regOut.URN(), len(regOut.Outputs()))
		return regOut, nil
	case err := <-iter.finChan:
		// If we are finished, we can safely exit.  The contract with the language provider is that this implies
		// that the language runtime has exited and so calling Close on the plugin is fine.
		iter.done = true
		if err != nil {
			glog.V(5).Infof("EvalSourceIterator ended with an error: %v", err)
		}
		return nil, err
	}
}

// forkRun performs the evaluation from a distinct goroutine.  This function blocks until it's our turn to go.
func (iter *evalSourceIterator) forkRun(opts Options) {
	// If we are destroying, no need to perform any evaluation beyond the config initialization.
	if !iter.src.destroy {
		// Fire up the goroutine to make the RPC invocation against the language runtime.  As this executes, calls
		// to queue things up in the resource channel will occur, and we will serve them concurrently.
		// FIXME: we need to ensure that out of order calls won't deadlock us.  In particular, we need to ensure: 1)
		//    gRPC won't block the dispatching of calls, and 2) that the channel's fixed size won't cause troubles.
		go func() {
			// Next, launch the language plugin.
			// IDEA: cache these so we reuse the same language plugin instance; if we do this, monitors must be per-run.
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
}

// resmon implements the lumirpc.ResourceMonitor interface and acts as the gateway between a language runtime's
// evaluation of a program and the internal resource planning and deployment logic.
type resmon struct {
	src        *evalSource                        // the evaluation source.
	regChan    chan *registerResourceEvent        // the channel to send resource registrations to.
	regOutChan chan *registerResourceOutputsEvent // the channel to send resource output registrations to.
	addr       string                             // the address the host is listening on.
	cancel     chan bool                          // a channel that can cancel the server.
	done       chan error                         // a channel that resolves when the server completes.
}

// newResourceMonitor creates a new resource monitor RPC server.
func newResourceMonitor(src *evalSource, regChan chan *registerResourceEvent,
	regOutChan chan *registerResourceOutputsEvent) (*resmon, error) {
	// New up an engine RPC server.
	resmon := &resmon{
		src:        src,
		regChan:    regChan,
		regOutChan: regOutChan,
		cancel:     make(chan bool),
	}

	// Fire up a gRPC server and start listening for incomings.
	port, done, err := rpcutil.Serve(0, resmon.cancel, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			lumirpc.RegisterResourceMonitorServer(srv, resmon)
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
	rm.cancel <- true
	return <-rm.done
}

// Invoke performs an invocation of a member located in a resource provider.
func (rm *resmon) Invoke(ctx context.Context, req *lumirpc.InvokeRequest) (*lumirpc.InvokeResponse, error) {
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
	glog.V(5).Infof("ResourceMonitor.Invoke received: tok=%v #args=%v", tok, len(args))
	ret, failures, err := prov.Invoke(tok, args)
	if err != nil {
		return nil, errors.Wrapf(err, "invocation of %v returned an error", tok)
	}
	mret, err := plugin.MarshalProperties(ret, plugin.MarshalOptions{Label: label, KeepUnknowns: true})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal %v return", tok)
	}
	var chkfails []*lumirpc.CheckFailure
	for _, failure := range failures {
		chkfails = append(chkfails, &lumirpc.CheckFailure{
			Property: string(failure.Property),
			Reason:   failure.Reason,
		})
	}
	return &lumirpc.InvokeResponse{Return: mret, Failures: chkfails}, nil
}

// RegisterResource is invoked by a language process when a new resource has been allocated.
func (rm *resmon) RegisterResource(ctx context.Context,
	req *lumirpc.RegisterResourceRequest) (*lumirpc.RegisterResourceResponse, error) {

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

	glog.V(5).Infof(
		"ResourceMonitor.RegisterResource received: t=%v, name=%v, custom=%v, #props=%v, parent=%v, protect=%v, deps=%v",
		t, name, custom, len(props), parent, protect, dependencies)

	// Send the goal state to the engine.
	step := &registerResourceEvent{
		goal: resource.NewGoal(t, name, custom, props, parent, protect, dependencies),
		done: make(chan *RegisterResult),
	}
	rm.regChan <- step

	// Now block waiting for the operation to finish.
	// IDEA: we probably need some way to cancel this in case of catastrophe.
	result := <-step.done
	state := result.State
	props = state.All()
	stable := result.Stable
	var stables []string
	for _, sta := range result.Stables {
		stables = append(stables, string(sta))
	}
	glog.V(5).Infof(
		"ResourceMonitor.RegisterResource operation finished: t=%v, urn=%v, stable=%v, #stables=%v #outs=%v",
		state.Type, state.URN, stable, len(stables), len(props))

	// Finally, unpack the response into properties that we can return to the language runtime.  This mostly includes
	// an ID, URN, and defaults and output properties that will all be blitted back onto the runtime object.
	obj, err := plugin.MarshalProperties(props, plugin.MarshalOptions{Label: label, KeepUnknowns: true})
	if err != nil {
		return nil, err
	}
	return &lumirpc.RegisterResourceResponse{
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
	req *lumirpc.RegisterResourceOutputsRequest) (*pbempty.Empty, error) {

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
	glog.V(5).Infof("ResourceMonitor.RegisterResourceOutputs received: urn=%v, #outs=%v", urn, len(outs))

	// Now send the step over to the engine to perform.
	step := &registerResourceOutputsEvent{
		urn:     urn,
		outputs: outs,
		done:    make(chan bool),
	}
	rm.regOutChan <- step

	// Now block waiting for the operation to finish.
	// IDEA: we probably need some way to cancel this in case of catastrophe.
	<-step.done
	glog.V(5).Infof(
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
