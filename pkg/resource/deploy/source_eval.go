// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package deploy

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/pkg/pack"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/rpcutil"
	lumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

// EvalRunInfo provides information required to execute and deploy resources within a package.
type EvalRunInfo struct {
	Pkg     *pack.Package `json:"pkg"`              // the package metadata.
	Pwd     string        `json:"pwd"`              // the package's working directory.
	Program string        `json:"program"`          // the path to the program we are executing.
	Args    []string      `json:"args,omitempty"`   // any arguments to pass to the package.
	Target  *Target       `json:"target,omitempty"` // the target being deployed into.
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

func (src *evalSource) Pkg() tokens.PackageName {
	return src.runinfo.Pkg.Name
}

func (src *evalSource) Info() interface{} {
	return src.runinfo
}

// Iterate will spawn an evaluator coroutine and prepare to interact with it on subsequent calls to Next.
func (src *evalSource) Iterate(opts Options) (SourceIterator, error) {
	// First, fire up a resource monitor that will watch for and record resource creation.
	reschan := make(chan *evalSourceGoal)
	mon, err := newResourceMonitor(src, reschan)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start resource monitor")
	}

	// Create a new iterator with appropriate channels, and gear up to go!
	iter := &evalSourceIterator{
		mon:     mon,
		src:     src,
		finchan: make(chan error),
		reschan: reschan,
	}

	// Now invoke Run in a goroutine.  All subsequent resource creation events will come in over the gRPC channel,
	// and we will pump them through the channel.  If the Run call ultimately fails, we need to propagate the error.
	iter.forkRun(opts)

	// Finally, return the fresh iterator that the caller can use to take things from here.
	return iter, nil
}

type evalSourceIterator struct {
	mon     *resmon              // the resource monitor, per iterator.
	src     *evalSource          // the owning eval source object.
	finchan chan error           // the channel that communicates completion.
	reschan chan *evalSourceGoal // the channel that contains resource elements.
	done    bool                 // set to true when the evaluation is done.
}

func (iter *evalSourceIterator) Close() error {
	// Cancel the monitor and reclaim any associated resources.
	return iter.mon.Cancel()
}

func (iter *evalSourceIterator) Next() (SourceGoal, error) {
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
	case err := <-iter.finchan:
		// If we are finished, we can safely exit.  The contract with the language provider is that this implies
		// that the language runtime has exited and so calling Close on the plugin is fine.
		iter.done = true
		if err != nil {
			glog.V(5).Infof("EvalSourceIterator ended with an error: %v", err)
		}
		return nil, err
	case res := <-iter.reschan:
		contract.Assert(res != nil)
		goal := res.Resource()
		glog.V(5).Infof("EvalSourceIterator produced a new object: t=%v,name=%v,#props=%v",
			goal.Type, goal.Name, len(goal.Properties))
		return res, nil
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
			rt := iter.src.runinfo.Pkg.Runtime
			langhost, err := iter.src.plugctx.Host.LanguageRuntime(rt, iter.mon.Address())
			if err != nil {
				err = errors.Wrapf(err, "failed to launch language host for '%v'", rt)
			} else if langhost == nil {
				err = errors.Errorf("could not load language plugin for '%v' from $PATH", rt)
			} else {
				// Make sure to clean up before exiting.
				defer contract.IgnoreClose(langhost)

				// Now run the actual program.
				var progerr string
				progerr, err = langhost.Run(plugin.RunInfo{
					Stack:    string(iter.src.runinfo.Target.Name),
					Pwd:      iter.src.runinfo.Pwd,
					Program:  iter.src.runinfo.Program,
					Args:     iter.src.runinfo.Args,
					Config:   iter.src.runinfo.Target.Config,
					DryRun:   iter.src.dryRun,
					Parallel: opts.Parallel,
				})
				if err == nil && progerr != "" {
					// If the program had an unhandled error; propagate it to the caller.
					err = errors.Errorf("an unhandled error occurred: %v", progerr)
				}
			}

			// Communicate the error, if it exists, or nil if the program exited cleanly.
			iter.finchan <- err
		}()
	}
}

// resmon implements the lumirpc.ResourceMonitor interface and acts as the gateway between a language runtime's
// evaluation of a program and the internal resource planning and deployment logic.
type resmon struct {
	src     *evalSource          // the evaluation source.
	reschan chan *evalSourceGoal // the channel to send resources to.
	addr    string               // the address the host is listening on.
	cancel  chan bool            // a channel that can cancel the server.
	done    chan error           // a channel that resolves when the server completes.
}

// newResourceMonitor creates a new resource monitor RPC server.
func newResourceMonitor(src *evalSource, reschan chan *evalSourceGoal) (*resmon, error) {
	// New up an engine RPC server.
	resmon := &resmon{
		src:     src,
		reschan: reschan,
		cancel:  make(chan bool),
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
	tok := tokens.ModuleMember(req.GetTok())
	prov, err := rm.src.plugctx.Host.Provider(tok.Package())
	if err != nil {
		return nil, err
	} else if prov == nil {
		return nil, errors.Errorf("could not load resource provider for package '%v' from $PATH", tok.Package())
	}

	// Now unpack all of the arguments and prepare to perform the invocation.
	args, err := plugin.UnmarshalProperties(
		req.GetArgs(), plugin.MarshalOptions{AllowUnknowns: true})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal %v args", tok)
	}

	// Do the invoke and then return the arguments.
	glog.V(5).Infof("ResourceMonitor.Invoke received: tok=%v #args=%v", tok, len(args))
	ret, failures, err := prov.Invoke(tok, args)
	if err != nil {
		return nil, errors.Wrapf(err, "invocation of %v returned an error", tok)
	}
	mret, err := plugin.MarshalProperties(ret, plugin.MarshalOptions{AllowUnknowns: true})
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

// NewResource is invoked by a language process when a new resource has been allocated.
func (rm *resmon) NewResource(ctx context.Context,
	req *lumirpc.NewResourceRequest) (*lumirpc.NewResourceResponse, error) {

	// Communicate the type, name, and object information to the iterator that is awaiting us.
	props, err := plugin.UnmarshalProperties(
		req.GetObject(), plugin.MarshalOptions{AllowUnknowns: true})
	if err != nil {
		return nil, err
	}

	var children []resource.URN
	for _, child := range req.GetChildren() {
		children = append(children, resource.URN(child))
	}

	goal := &evalSourceGoal{
		goal: resource.NewGoal(
			tokens.Type(req.GetType()),
			tokens.QName(req.GetName()),
			req.GetCustom(),
			props,
			children,
		),
		done: make(chan *evalState),
	}
	glog.V(5).Infof("ResourceMonitor.NewResource received: t=%v, name=%v, custom=%v, #props=%v, #children=%v",
		goal.goal.Type, goal.goal.Name, goal.goal.Custom, len(goal.goal.Properties), len(goal.goal.Children))
	rm.reschan <- goal

	// Now block waiting for the operation to finish.
	// FIXME: we probably need some way to cancel this in case of catastrophe.
	done := <-goal.done
	state := done.State
	outprops := state.Synthesized()
	stable := done.Stable
	var stables []string
	for _, sta := range done.Stables {
		stables = append(stables, string(sta))
	}
	glog.V(5).Infof(
		"ResourceMonitor.NewResource operation finished: t=%v, urn=%v (name=%v), stable=%v, #stables=%v #outs=%v",
		state.Type, state.URN, goal.goal.Name, stable, len(stables), len(outprops))

	// Finally, unpack the response into properties that we can return to the language runtime.  This mostly includes
	// an ID, URN, and defaults and output properties that will all be blitted back onto the runtime object.
	outs, err := plugin.MarshalProperties(outprops, plugin.MarshalOptions{AllowUnknowns: true})
	if err != nil {
		return nil, err
	}
	return &lumirpc.NewResourceResponse{
		Id:      string(state.ID),
		Urn:     string(state.URN),
		Object:  outs,
		Stable:  stable,
		Stables: stables,
	}, nil
}

type evalState struct {
	State   *resource.State        // the resource state.
	Stable  bool                   // if true, the resource state is stable and may be trusted.
	Stables []resource.PropertyKey // an optional list of specific resource properties that are stable.
}

type evalSourceGoal struct {
	goal *resource.Goal  // the resource goal state produced by the iterator.
	done chan *evalState // the channel to communicate with after the resource state is available.
}

func (g *evalSourceGoal) Resource() *resource.Goal {
	return g.goal
}

func (g *evalSourceGoal) Done(state *resource.State, stable bool, stables []resource.PropertyKey) {
	// Communicate the resulting state back to the RPC thread, which is parked awaiting our reply.
	g.done <- &evalState{
		State:   state,
		Stable:  stable,
		Stables: stables,
	}
}
