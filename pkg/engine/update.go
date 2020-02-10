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

package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/util/result"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// RequiredPolicy represents a set of policies to apply during an update.
type RequiredPolicy interface {
	// Name provides the user-specified name of the PolicyPack.
	Name() string
	// Version of the PolicyPack.
	Version() string
	// Install will install the PolicyPack locally, returning the path it was installed to.
	Install(ctx context.Context) (string, error)
}

// LocalPolicyPack represents a set of local Policy Packs to apply during an update.
type LocalPolicyPack struct {
	// Name provides the user-specified name of the Policy Pack.
	Name string
	// Path of the local Policy Pack.
	Path string
	// Path of the local Policy Pack's JSON config file.
	Config string
}

// MakeLocalPolicyPacks is a helper function for converting the list of local Policy
// Pack paths to list of LocalPolicyPack. The name of the Local Policy Pack is not set
// since we must load up the Policy Pack plugin to determine its name.
func MakeLocalPolicyPacks(localPaths []string, configPaths []string) []LocalPolicyPack {
	// If we have any configPaths, we should have already validated that the length of
	// the localPaths and configPaths are the same.
	if len(configPaths) > 0 {
		contract.Assertf(len(localPaths) == len(configPaths), "len(localPaths) != len(configPaths)")
	}

	r := make([]LocalPolicyPack, len(localPaths))
	for i, p := range localPaths {
		var config string
		if len(configPaths) > 0 {
			config = configPaths[i]
		}
		r[i] = LocalPolicyPack{
			Path:   p,
			Config: config,
		}
	}
	return r
}

// ConvertLocalPolicyPacksToPaths is a helper function for converting the list of LocalPolicyPacks
// to a list of paths.
func ConvertLocalPolicyPacksToPaths(localPolicyPack []LocalPolicyPack) []string {
	r := make([]string, len(localPolicyPack))
	for i, p := range localPolicyPack {
		r[i] = p.Name
	}
	return r
}

// UpdateOptions contains all the settings for customizing how an update (deploy, preview, or destroy) is performed.
//
// This structure is embedded in another which uses some of the unexported fields, which trips up the `structcheck`
// linter.
// nolint: structcheck
type UpdateOptions struct {
	// LocalPolicyPacks contains an optional set of policy packs to run as part of this deployment.
	LocalPolicyPacks []LocalPolicyPack

	// RequiredPolicies is the set of policies that are required to run as part of the update.
	RequiredPolicies []RequiredPolicy

	// the degree of parallelism for resource operations (<=1 for serial).
	Parallel int

	// true if debugging output it enabled
	Debug bool

	// true if the plan should refresh before executing.
	Refresh bool

	// Specific resources to refresh during a refresh operation.
	RefreshTargets []resource.URN

	// Specific resources to replace during an update operation.
	ReplaceTargets []resource.URN

	// Specific resources to destroy during a destroy operation.
	DestroyTargets []resource.URN

	// Specific resources to update during an update operation.
	UpdateTargets []resource.URN

	// true if we're allowing dependent targets to change, even if not specified in one of the above
	// XXXTargets lists.
	TargetDependents bool

	// true if the engine should use legacy diffing behavior during an update.
	UseLegacyDiff bool

	// true if we should report events for steps that involve default providers.
	reportDefaultProviderSteps bool

	// the plugin host to use for this update
	host plugin.Host
}

// ResourceChanges contains the aggregate resource changes by operation type.
type ResourceChanges map[deploy.StepOp]int

// HasChanges returns true if there are any non-same changes in the resulting summary.
func (changes ResourceChanges) HasChanges() bool {
	var c int
	for op, count := range changes {
		if op != deploy.OpSame &&
			op != deploy.OpRead &&
			op != deploy.OpReadDiscard &&
			op != deploy.OpReadReplacement {
			c += count
		}
	}
	return c > 0
}

func Update(u UpdateInfo, ctx *Context, opts UpdateOptions, dryRun bool) (ResourceChanges, result.Result) {
	contract.Require(u != nil, "update")
	contract.Require(ctx != nil, "ctx")

	defer func() { ctx.Events <- cancelEvent() }()

	info, err := newPlanContext(u, "update", ctx.ParentSpan)
	if err != nil {
		return nil, result.FromError(err)
	}
	defer info.Close()

	emitter, err := makeEventEmitter(ctx.Events, u)
	if err != nil {
		return nil, result.FromError(err)
	}
	defer emitter.Close()

	return update(ctx, info, planOptions{
		UpdateOptions: opts,
		SourceFunc:    newUpdateSource,
		Events:        emitter,
		Diag:          newEventSink(emitter, false),
		StatusDiag:    newEventSink(emitter, true),
	}, dryRun)
}

// RunInstallPlugins calls installPlugins and just returns the error (avoids having to export pluginSet).
func RunInstallPlugins(
	proj *workspace.Project, pwd, main string, target *deploy.Target, plugctx *plugin.Context) error {
	_, _, err := installPlugins(proj, pwd, main, target, plugctx)
	return err
}

func installPlugins(
	proj *workspace.Project, pwd, main string, target *deploy.Target,
	plugctx *plugin.Context) (pluginSet, map[tokens.Package]*semver.Version, error) {

	// Before launching the source, ensure that we have all of the plugins that we need in order to proceed.
	//
	// There are two places that we need to look for plugins:
	//   1. The language host, which reports to us the set of plugins that the program that's about to execute
	//      needs in order to create new resources. This is purely advisory by the language host and not all
	//      languages implement this (notably Python).
	//   2. The snapshot. The snapshot contains plugins in two locations: first, in the manifest, all plugins
	//      that were loaded are recorded. Second, all first class providers record the version of the plugin
	//      to which they are bound.
	//
	// In order to get a complete view of the set of plugins that we need for an update or query, we must
	// consult both sources and merge their results into a list of plugins.
	languagePlugins, err := gatherPluginsFromProgram(plugctx, plugin.ProgInfo{
		Proj:    proj,
		Pwd:     pwd,
		Program: main,
	})
	if err != nil {
		return nil, nil, err
	}
	snapshotPlugins, err := gatherPluginsFromSnapshot(plugctx, target)
	if err != nil {
		return nil, nil, err
	}

	allPlugins := languagePlugins.Union(snapshotPlugins)

	// If there are any plugins that are not available, we can attempt to install them here.
	//
	// Note that this is purely a best-effort thing. If we can't install missing plugins, just proceed; we'll fail later
	// with an error message indicating exactly what plugins are missing.
	if err := ensurePluginsAreInstalled(allPlugins); err != nil {
		logging.V(7).Infof("newUpdateSource(): failed to install missing plugins: %v", err)
	}

	// Collect the version information for default providers.
	defaultProviderVersions := computeDefaultProviderPlugins(languagePlugins, allPlugins)

	return allPlugins, defaultProviderVersions, nil
}

func installAndLoadPolicyPlugins(plugctx *plugin.Context, policies []RequiredPolicy, localPolicyPacks []LocalPolicyPack,
	opts *plugin.PolicyAnalyzerOptions) error {

	// Install and load required policy packs.
	for _, policy := range policies {
		policyPath, err := policy.Install(context.Background())
		if err != nil {
			return err
		}

		_, err = plugctx.Host.PolicyAnalyzer(tokens.QName(policy.Name()), policyPath, opts)
		if err != nil {
			return err
		}
	}

	// Load local policy packs.
	for i, pack := range localPolicyPacks {
		abs, err := filepath.Abs(pack.Path)
		if err != nil {
			return err
		}

		analyzer, err := plugctx.Host.PolicyAnalyzer(tokens.QName(abs), pack.Path, opts)
		if err != nil {
			return err
		} else if analyzer == nil {
			return errors.Errorf("analyzer could not be loaded from path %q", pack.Path)
		}

		// Update the Policy Pack names now that we have loaded the plugins and can access the name.
		analyzerInfo, err := analyzer.GetAnalyzerInfo()
		if err != nil {
			return err
		}
		localPolicyPacks[i].Name = analyzerInfo.Name
	}
	return nil
}

func newUpdateSource(
	client deploy.BackendClient, opts planOptions, proj *workspace.Project, pwd, main string,
	target *deploy.Target, plugctx *plugin.Context, dryRun bool) (deploy.Source, error) {

	//
	// Step 1: Install and load plugins.
	//

	allPlugins, defaultProviderVersions, err := installPlugins(proj, pwd, main, target,
		plugctx)
	if err != nil {
		return nil, err
	}

	// Once we've installed all of the plugins we need, make sure that all analyzers and language plugins are
	// loaded up and ready to go. Provider plugins are loaded lazily by the provider registry and thus don't
	// need to be loaded here.
	const kinds = plugin.AnalyzerPlugins | plugin.LanguagePlugins
	if err := ensurePluginsAreLoaded(plugctx, allPlugins, kinds); err != nil {
		return nil, err
	}

	//
	// Step 2: Install and load policy plugins.
	//

	// Decrypt the configuration.
	config, err := target.Config.Decrypt(target.Decrypter)
	if err != nil {
		return nil, err
	}
	analyzerOpts := plugin.PolicyAnalyzerOptions{
		Project: proj.Name.String(),
		Stack:   target.Name.String(),
		Config:  config,
		DryRun:  dryRun,
	}
	if err := installAndLoadPolicyPlugins(plugctx, opts.RequiredPolicies, opts.LocalPolicyPacks,
		&analyzerOpts); err != nil {
		return nil, err
	}

	// If that succeeded, create a new source that will perform interpretation of the compiled program.
	// TODO[pulumi/pulumi#88]: we are passing `nil` as the arguments map; we need to allow a way to pass these.
	return deploy.NewEvalSource(plugctx, &deploy.EvalRunInfo{
		Proj:    proj,
		Pwd:     pwd,
		Program: main,
		Target:  target,
	}, defaultProviderVersions, dryRun), nil
}

func update(ctx *Context, info *planContext, opts planOptions, dryRun bool) (ResourceChanges, result.Result) {
	planResult, err := plan(ctx, info, opts, dryRun)
	if err != nil {
		return nil, result.FromError(err)
	}

	policies := map[string]string{}
	for _, p := range opts.RequiredPolicies {
		policies[p.Name()] = p.Version()
	}
	for _, pack := range opts.LocalPolicyPacks {
		path := abbreviateFilePath(pack.Path)
		packName := fmt.Sprintf("%s (%s)", pack.Name, path)
		policies[packName] = "(local)"
	}

	var resourceChanges ResourceChanges
	var res result.Result
	if planResult != nil {
		defer contract.IgnoreClose(planResult)

		// Make the current working directory the same as the program's, and restore it upon exit.
		done, chErr := planResult.Chdir()
		if chErr != nil {
			return nil, result.FromError(chErr)
		}
		defer done()

		if dryRun {
			// If a dry run, just print the plan, don't actually carry out the deployment.
			resourceChanges, res = printPlan(ctx, planResult, dryRun, policies)
		} else {
			// Otherwise, we will actually deploy the latest bits.
			opts.Events.preludeEvent(dryRun, planResult.Ctx.Update.GetTarget().Config)

			// Walk the plan, reporting progress and executing the actual operations as we go.
			start := time.Now()
			actions := newUpdateActions(ctx, info.Update, opts)

			res = planResult.Walk(ctx, actions, false)
			resourceChanges = ResourceChanges(actions.Ops)

			if len(resourceChanges) != 0 {

				// Print out the total number of steps performed (and their kinds), the duration, and any summary info.
				opts.Events.updateSummaryEvent(actions.MaybeCorrupt, time.Since(start),
					resourceChanges, policies)
			}
		}
	}
	return resourceChanges, res
}

// abbreviateFilePath is a helper function that cleans up and shortens a provided file path.
// If the path is long, it will keep the first two and last two directories and then replace the
// middle directories with `...`.
func abbreviateFilePath(path string) string {
	path = filepath.Clean(path)
	if len(path) > 75 {
		// Do some shortening.
		separator := "/"
		dirs := strings.Split(path, separator)

		// If we get no splits, we will try to use the backslashes in support of a Windows path.
		if len(dirs) == 1 {
			separator = `\`
			dirs = strings.Split(path, separator)
		}

		if len(dirs) > 4 {
			back := dirs[len(dirs)-2:]
			dirs = append(dirs[:2], "...")
			dirs = append(dirs, back...)
		}
		path = strings.Join(dirs, separator)
	}
	return path
}

// updateActions pretty-prints the plan application process as it goes.
type updateActions struct {
	Context      *Context
	Steps        int
	Ops          map[deploy.StepOp]int
	Seen         map[resource.URN]deploy.Step
	MapLock      sync.Mutex
	MaybeCorrupt bool
	Update       UpdateInfo
	Opts         planOptions
}

func newUpdateActions(context *Context, u UpdateInfo, opts planOptions) *updateActions {
	return &updateActions{
		Context: context,
		Ops:     make(map[deploy.StepOp]int),
		Seen:    make(map[resource.URN]deploy.Step),
		Update:  u,
		Opts:    opts,
	}
}

func (acts *updateActions) OnResourceStepPre(step deploy.Step) (interface{}, error) {
	// Ensure we've marked this step as observed.
	acts.MapLock.Lock()
	acts.Seen[step.URN()] = step
	acts.MapLock.Unlock()

	// Skip reporting if necessary.
	if shouldReportStep(step, acts.Opts) {
		acts.Opts.Events.resourcePreEvent(step, false /*planning*/, acts.Opts.Debug)
	}

	// Inform the snapshot service that we are about to perform a step.
	return acts.Context.SnapshotManager.BeginMutation(step)
}

func (acts *updateActions) OnResourceStepPost(
	ctx interface{}, step deploy.Step,
	status resource.Status, err error) error {

	acts.MapLock.Lock()
	assertSeen(acts.Seen, step)
	acts.MapLock.Unlock()

	// If we've already been terminated, exit without writing the checkpoint. We explicitly want to leave the
	// checkpoint in an inconsistent state in this event.
	if acts.Context.Cancel.TerminateErr() != nil {
		return nil
	}

	reportStep := shouldReportStep(step, acts.Opts)

	// Report the result of the step.
	if err != nil {
		if status == resource.StatusUnknown {
			acts.MaybeCorrupt = true
		}

		errorURN := resource.URN("")
		if reportStep {
			errorURN = step.URN()
		}

		// Issue a true, bonafide error.
		acts.Opts.Diag.Errorf(diag.GetResourceOperationFailedError(errorURN), err)
		if reportStep {
			acts.Opts.Events.resourceOperationFailedEvent(step, status, acts.Steps, acts.Opts.Debug)
		}
	} else if reportStep {
		op, record := step.Op(), step.Logical()
		if acts.Opts.isRefresh && op == deploy.OpRefresh {
			// Refreshes are handled specially.
			op, record = step.(*deploy.RefreshStep).ResultOp(), true
		}

		if step.Op() == deploy.OpRead {
			record = ShouldRecordReadStep(step)
		}

		if record {
			// Increment the counters.
			acts.MapLock.Lock()
			acts.Steps++
			acts.Ops[op]++
			acts.MapLock.Unlock()
		}

		// Also show outputs here for custom resources, since there might be some from the initial registration. We do
		// not show outputs for component resources at this point: any that exist must be from a previous execution of
		// the Pulumi program, as component resources only report outputs via calls to RegisterResourceOutputs.
		if step.Res().Custom || acts.Opts.Refresh && step.Op() == deploy.OpRefresh {
			acts.Opts.Events.resourceOutputsEvent(op, step, false /*planning*/, acts.Opts.Debug)
		}
	}

	// See pulumi/pulumi#2011 for details. Terraform always returns the existing state with the diff applied to it in
	// the event of an update failure. It's appropriate that we save this new state in the output of the resource, but
	// it is not appropriate to save the inputs, because the resource that exists was not created or updated
	// successfully with those inputs.
	//
	// If we were doing an update and got a `StatusPartialFailure`, the resource that ultimately gets persisted in the
	// snapshot should be old inputs and new outputs. We accomplish that here by clobbering the new resource's inputs
	// with the old inputs.
	//
	// This is a little kludgy given that these resources are global state. However, given the way that we have
	// implemented the snapshot manager and engine today, it's the easiest way to accomplish what we are trying to do.
	if status == resource.StatusPartialFailure && step.Op() == deploy.OpUpdate {
		logging.V(7).Infof(
			"OnResourceStepPost(%s): Step is partially-failed update, saving old inputs instead of new inputs",
			step.URN())
		new := step.New()
		old := step.Old()
		contract.Assert(new != nil)
		contract.Assert(old != nil)
		new.Inputs = make(resource.PropertyMap)
		for key, value := range old.Inputs {
			new.Inputs[key] = value
		}
	}

	// Write out the current snapshot. Note that even if a failure has occurred, we should still have a
	// safe checkpoint.  Note that any error that occurs when writing the checkpoint trumps the error
	// reported above.
	return ctx.(SnapshotMutation).End(step, err == nil || status == resource.StatusPartialFailure)
}

func (acts *updateActions) OnResourceOutputs(step deploy.Step) error {
	acts.MapLock.Lock()
	assertSeen(acts.Seen, step)
	acts.MapLock.Unlock()

	// Skip reporting if necessary.
	if shouldReportStep(step, acts.Opts) {
		acts.Opts.Events.resourceOutputsEvent(step.Op(), step, false /*planning*/, acts.Opts.Debug)
	}

	// There's a chance there are new outputs that weren't written out last time.
	// We need to perform another snapshot write to ensure they get written out.
	return acts.Context.SnapshotManager.RegisterResourceOutputs(step)
}

func (acts *updateActions) OnPolicyViolation(urn resource.URN, d plugin.AnalyzeDiagnostic) {
	acts.Opts.Events.policyViolationEvent(urn, d)
}
