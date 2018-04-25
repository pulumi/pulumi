// Copyright 2018, Pulumi Corporation.  All rights reserved.

package engine

import (
	"os"

	"github.com/opentracing/opentracing-go"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// ProjectInfoContext returns information about the current project, including its pwd, main, and plugin context.
func ProjectInfoContext(projinfo *Projinfo, config plugin.ConfigSource, pluginEvents plugin.Events,
	diag diag.Sink, tracingSpan opentracing.Span) (string, string, *plugin.Context, error) {
	contract.Require(projinfo != nil, "projinfo")

	// If the package contains an override for the main entrypoint, use it.
	pwd, main, err := projinfo.GetPwdMain()
	if err != nil {
		return "", "", nil, err
	}

	// Create a context for plugins.
	ctx, err := plugin.NewContext(diag, nil, config, pluginEvents, pwd, tracingSpan)
	if err != nil {
		return "", "", nil, err
	}

	return pwd, main, ctx, nil
}

// newPlanContext creates a context for a subsequent planning operation.  Callers must call Close on the
// resulting context object once they have completed the associated planning operation.
func newPlanContext(u UpdateInfo) (*planContext, error) {
	contract.Require(u != nil, "u")

	// Create a root span for the operation
	tracingSpan := opentracing.StartSpan("pulumi-plan")

	return &planContext{
		Update:      u,
		TracingSpan: tracingSpan,
	}, nil
}

type planContext struct {
	Update      UpdateInfo       // The update being processed.
	TracingSpan opentracing.Span // An OpenTracing span to parent plan operations within.
}

func (ctx *planContext) Close() {
	ctx.TracingSpan.Finish()
}

// planOptions includes a full suite of options for performing a plan and/or deploy operation.
type planOptions struct {
	UpdateOptions

	// SourceFunc is a factory that returns an EvalSource to use during planning.  This is the thing that
	// creates resources to compare against the current checkpoint state (e.g., by evaluating a program, etc).
	SourceFunc planSourceFunc

	SkipOutputs  bool          // true if we we should skip printing outputs separately.
	DOT          bool          // true if we should print the DOT file for this plan.
	Events       eventEmitter  // the channel to write events from the engine to.
	Diag         diag.Sink     // the sink to use for diag'ing.
	PluginEvents plugin.Events // an optional listener for plugin events
}

// planSourceFunc is a callback that will be used to prepare for, and evaluate, the "new" state for a stack.
type planSourceFunc func(
	opts planOptions, proj *workspace.Project, pwd, main string,
	target *deploy.Target, plugctx *plugin.Context, dryRun bool) (deploy.Source, error)

// plan just uses the standard logic to parse arguments, options, and to create a snapshot and plan.
func plan(ctx *planContext, opts planOptions, dryRun bool) (*planResult, error) {
	contract.Assert(ctx != nil)
	contract.Assert(ctx.Update != nil)
	contract.Assert(opts.SourceFunc != nil)

	// First, load the package metadata and the deployment target in preparation for executing the package's program
	// and creating resources.  This includes fetching its pwd and main overrides.
	proj, target := ctx.Update.GetProject(), ctx.Update.GetTarget()
	contract.Assert(proj != nil)
	contract.Assert(target != nil)
	projinfo := &Projinfo{Proj: proj, Root: ctx.Update.GetRoot()}
	pwd, main, plugctx, err := ProjectInfoContext(projinfo, target, opts.PluginEvents, opts.Diag, ctx.TracingSpan)
	if err != nil {
		return nil, err
	}

	// Now create the state source.  This may issue an error if it can't create the source.  This entails,
	// for example, loading any plugins which will be required to execute a program, among other things.
	source, err := opts.SourceFunc(opts, proj, pwd, main, target, plugctx, dryRun)
	if err != nil {
		return nil, err
	}

	// If there are any analyzers in the project file, add them.
	var analyzers []tokens.QName
	if as := projinfo.Proj.Analyzers; as != nil {
		for _, a := range *as {
			analyzers = append(analyzers, a)
		}
	}

	// Append any analyzers from the command line.
	for _, a := range opts.Analyzers {
		analyzers = append(analyzers, tokens.QName(a))
	}

	// Generate a plan; this API handles all interesting cases (create, update, delete).
	plan := deploy.NewPlan(plugctx, target, target.Snapshot, source, analyzers, dryRun)
	return &planResult{
		Ctx:     ctx,
		Plugctx: plugctx,
		Plan:    plan,
		Options: opts,
	}, nil
}

type planResult struct {
	Ctx     *planContext    // plan context information.
	Plugctx *plugin.Context // the context containing plugins and their state.
	Plan    *deploy.Plan    // the plan created by this command.
	Options planOptions     // the options used during planning.
}

// Chdir changes the directory so that all operations from now on are relative to the project we are working with.
// It returns a function that, when run, restores the old working directory.
func (res *planResult) Chdir() (func(), error) {
	pwd := res.Plugctx.Pwd
	if pwd == "" {
		return func() {}, nil
	}
	oldpwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if err = os.Chdir(pwd); err != nil {
		return nil, errors.Wrapf(err, "could not change to the project working directory")
	}
	return func() {
		// Restore the working directory after planning completes.
		cderr := os.Chdir(oldpwd)
		contract.IgnoreError(cderr)
	}, nil
}

// Walk enumerates all steps in the plan, calling out to the provided action at each step.  It returns four things: the
// resulting Snapshot, no matter whether an error occurs or not; an error, if something went wrong; the step that
// failed, if the error is non-nil; and finally the state of the resource modified in the failing step.
func (res *planResult) Walk(ctx *Context, events deploy.Events, preview bool) (deploy.PlanSummary,
	deploy.Step, resource.Status, error) {
	opts := deploy.Options{
		Events:   events,
		Parallel: res.Options.Parallel,
	}

	// Fetch a plan iterator and keep walking it until we are done.
	iter, err := res.Plan.Start(opts)
	if err != nil {
		return nil, nil, resource.StatusOK, err
	}

	step, err := iter.Next()
	if err != nil {
		closeErr := iter.Close() // ignore close errors; the Next error trumps
		contract.IgnoreError(closeErr)
		return nil, nil, resource.StatusOK, err
	}

	// Iterate the plan in a goroutine while listening for termination.
	var rst resource.Status
	done := make(chan bool)
	go func() {
		defer func() {
			// Close the iterator. If we have already observed another error, that error trumps the close error.
			closeErr := iter.Close()
			if err == nil {
				err = closeErr
			}
			close(done)
		}()

		for step != nil {
			// Check for cancellation and termination.
			if cancelErr := ctx.Cancel.CancelErr(); cancelErr != nil {
				rst, err = resource.StatusOK, cancelErr
				return
			}

			// Perform any per-step actions.
			rst, err = iter.Apply(step, preview)

			// If an error occurred, exit early.
			if err != nil {
				return
			}
			contract.Assert(rst == resource.StatusOK)

			step, err = iter.Next()
			if err != nil {
				return
			}
		}

		// Finally, return a summary and the resulting plan information.
		rst, err = resource.StatusOK, nil
	}()

	select {
	case <-ctx.Cancel.Terminated():
		return iter, step, rst, ctx.Cancel.TerminateErr()

	case <-done:
		return iter, step, rst, err
	}
}

func (res *planResult) Close() error {
	return res.Plugctx.Close()
}

// printPlan prints the plan's result to the plan's Options.Events stream.
func printPlan(ctx *Context, result *planResult, dryRun bool) (ResourceChanges, error) {
	result.Options.Events.preludeEvent(dryRun, result.Ctx.Update.GetTarget().Config)

	// Walk the plan's steps and and pretty-print them out.
	actions := newPreviewActions(result.Options)
	_, _, _, err := result.Walk(ctx, actions, true)
	if err != nil {
		return nil, errors.New("an error occurred while advancing the preview")
	}

	// Emit an event with a summary of operation counts.
	changes := ResourceChanges(actions.Ops)
	result.Options.Events.previewSummaryEvent(changes)
	return changes, nil
}

func assertSeen(seen map[resource.URN]deploy.Step, step deploy.Step) {
	_, has := seen[step.URN()]
	contract.Assertf(has, "URN '%v' had not been marked as seen", step.URN())
}
