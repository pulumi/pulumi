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
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/pulumi/pulumi/pkg/v3/display"
	resourceanalyzer "github.com/pulumi/pulumi/pkg/v3/resource/analyzer"
	"github.com/pulumi/pulumi/pkg/v3/resource/autonaming"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// RequiredPolicy represents a set of policies to apply during an update.
type RequiredPolicy interface {
	// Name provides the user-specified name of the PolicyPack.
	Name() string
	// Version of the PolicyPack.
	Version() string
	// Install will install the PolicyPack locally, returning the path it was installed to.
	Install(ctx context.Context) (string, error)
	// Config returns the PolicyPack's configuration.
	Config() map[string]*json.RawMessage
}

// LocalPolicyPack represents a set of local Policy Packs to apply during an update.
type LocalPolicyPack struct {
	// Name provides the user-specified name of the Policy Pack.
	Name string
	// Version of the local Policy Pack.
	Version string
	// Path of the local Policy Pack.
	Path string
	// Path of the local Policy Pack's JSON config file.
	Config string
}

// NameForEvents encodes a local policy pack's information in a single string which can
// be used for engine events. It is done this way so we don't lose path information.
func (pack LocalPolicyPack) NameForEvents() string {
	path := abbreviateFilePath(pack.Path)
	return fmt.Sprintf("%s|local|%s", pack.Name, path)
}

// GetLocalPolicyPackInfoFromEventName round trips the NameForEvents back into a name/path pair.
func GetLocalPolicyPackInfoFromEventName(name string) (string, string) {
	parts := strings.Split(name, "|")
	if len(parts) != 3 {
		return "", ""
	}
	return parts[0], parts[2]
}

// MakeLocalPolicyPacks is a helper function for converting the list of local Policy
// Pack paths to list of LocalPolicyPack. The name of the Local Policy Pack is not set
// since we must load up the Policy Pack plugin to determine its name.
func MakeLocalPolicyPacks(localPaths []string, configPaths []string) []LocalPolicyPack {
	// If we have any configPaths, we should have already validated that the length of
	// the localPaths and configPaths are the same.
	contract.Assertf(len(configPaths) == 0 || len(configPaths) == len(localPaths),
		"configPaths must be empty or match localPaths count (%d), got %d", len(localPaths), len(configPaths))

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
//
//nolint:structcheck
type UpdateOptions struct {
	// true if the step generator should use parallel diff.
	ParallelDiff bool

	// LocalPolicyPacks contains an optional set of policy packs to run as part of this deployment.
	LocalPolicyPacks []LocalPolicyPack

	// RequiredPolicies is the set of policies that are required to run as part of the update.
	RequiredPolicies []RequiredPolicy

	// the degree of parallelism for resource operations (<=1 for serial).
	Parallel int32

	// true if debugging output it enabled
	Debug bool

	// true if the plan should refresh before executing.
	Refresh bool

	// true if the plan should run the program as part of destroy.
	DestroyProgram bool

	// Specific resources to replace during an update operation.
	ReplaceTargets deploy.UrnTargets

	// Specific resources to update during a deployment.
	Targets deploy.UrnTargets

	// true if we're allowing dependent targets to change, even if not specified in one of the above
	// XXXTargets lists.
	TargetDependents bool

	// true if the engine should use legacy diffing behavior during an update.
	UseLegacyDiff bool

	// true if the engine should use legacy refresh diffing behavior and report
	// only output changes, as opposed to computing diffs against desired state.
	UseLegacyRefreshDiff bool

	// true if the engine should disable provider previews.
	DisableProviderPreview bool

	// true if the engine should disable resource reference support.
	DisableResourceReferences bool

	// true if the engine should disable output value support.
	DisableOutputValues bool

	// the plugin host to use for this update
	Host plugin.Host

	// The plan to use for the update, if any.
	Plan *deploy.Plan

	// GeneratePlan when true cause plans to be generated, we skip this if we know their not needed (e.g. during up)
	GeneratePlan bool

	// Experimental is true if the engine is in experimental mode (i.e. PULUMI_EXPERIMENTAL was set)
	Experimental bool

	// ContinueOnError is true if the engine should continue processing resources after an error is encountered.
	ContinueOnError bool

	// AttachDebugger to launch the language host in debug mode.
	AttachDebugger bool

	// Autonamer can resolve user's preference for custom autonaming options for a given resource.
	Autonamer autonaming.Autonamer

	// The execution kind of the operation.
	ExecKind string

	// ShowSecrets is true if the engine should display secrets in the CLI.
	ShowSecrets bool
}

// HasChanges returns true if there are any non-same changes in the resulting summary.
func HasChanges(changes display.ResourceChanges) bool {
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

func Update(u UpdateInfo, ctx *Context, opts UpdateOptions, dryRun bool) (
	*deploy.Plan, display.ResourceChanges, error,
) {
	contract.Requiref(u != nil, "update", "cannot be nil")
	contract.Requiref(ctx != nil, "ctx", "cannot be nil")
	defer func() { ctx.Events <- NewCancelEvent() }()

	info, err := newDeploymentContext(u, "update", ctx.ParentSpan)
	if err != nil {
		return nil, nil, err
	}
	defer info.Close()

	emitter, err := makeEventEmitter(ctx.Events, u)
	if err != nil {
		return nil, nil, err
	}
	defer emitter.Close()

	logging.V(7).Infof("*** Starting Update(preview=%v) ***", dryRun)
	defer logging.V(7).Infof("*** Update(preview=%v) complete ***", dryRun)

	// We skip the target check here because the targeted resource may not exist yet.

	return update(ctx, info, &deploymentOptions{
		UpdateOptions: opts,
		SourceFunc:    newUpdateSource,
		Events:        emitter,
		Diag:          newEventSink(emitter, false),
		StatusDiag:    newEventSink(emitter, true),
		DryRun:        dryRun,
	})
}

func installPlugins(
	ctx context.Context,
	proj *workspace.Project, pwd, main string, target *deploy.Target, opts *deploymentOptions,
	plugctx *plugin.Context, returnInstallErrors bool,
) (PluginSet, map[tokens.Package]workspace.PackageDescriptor, error) {
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
	runtime := proj.Runtime.Name()
	programInfo := plugin.NewProgramInfo(
		/* rootDirectory */ plugctx.Root,
		/* programDirectory */ pwd,
		/* entryPoint */ main,
		/* options */ proj.Runtime.Options(),
	)
	languagePackages, err := gatherPackagesFromProgram(plugctx, runtime, programInfo)
	if err != nil {
		return nil, nil, err
	}
	snapshotPackages, err := gatherPackagesFromSnapshot(plugctx, target)
	if err != nil {
		return nil, nil, err
	}

	allPackages := languagePackages.Union(snapshotPackages)
	allPlugins := allPackages.ToPluginSet().Deduplicate()

	// If there are any plugins that are not available, we can attempt to install them here.
	//
	// Note that this is purely a best-effort thing. If we can't install missing plugins, just proceed; we'll fail later
	// with an error message indicating exactly what plugins are missing. If `returnInstallErrors` is set, then return
	// the error.
	if err := EnsurePluginsAreInstalled(ctx, opts, plugctx.Diag, allPlugins,
		plugctx.Host.GetProjectPlugins(), false /*reinstall*/, false /*explicitInstall*/); err != nil {
		if returnInstallErrors {
			return nil, nil, err
		}
		logging.V(7).Infof("newUpdateSource(): failed to install missing plugins: %v", err)
	}

	// Collect the version information for default providers.
	defaultProviderVersions := computeDefaultProviderPackages(languagePackages, allPackages)

	return allPlugins, defaultProviderVersions, nil
}

// installAndLoadPolicyPlugins loads and installs all requird policy plugins and packages as well as any
// local policy packs. It returns fully populated metadata about those policy plugins.
func installAndLoadPolicyPlugins(ctx context.Context, plugctx *plugin.Context,
	deployOpts *deploymentOptions, analyzerOpts *plugin.PolicyAnalyzerOptions,
) error {
	var allValidationErrors []string
	appendValidationErrors := func(policyPackName, policyPackVersion string, validationErrors []string) {
		for _, validationError := range validationErrors {
			allValidationErrors = append(allValidationErrors,
				fmt.Sprintf("validating policy config: %s %s  %s",
					policyPackName, policyPackVersion, validationError))
		}
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(deployOpts.RequiredPolicies)+len(deployOpts.LocalPolicyPacks))
	// Install and load required policy packs.
	for _, policy := range deployOpts.RequiredPolicies {
		deployOpts.Events.PolicyLoadEvent()
		policyPath, err := policy.Install(ctx)
		if err != nil {
			return err
		}

		wg.Add(1)
		go func(policy RequiredPolicy, policyPath string) {
			defer wg.Done()
			analyzer, err := plugctx.Host.PolicyAnalyzer(tokens.QName(policy.Name()), policyPath, analyzerOpts)
			if err != nil {
				errs <- err
				return
			}

			analyzerInfo, err := analyzer.GetAnalyzerInfo()
			if err != nil {
				errs <- err
				return
			}

			// Parse the config, reconcile & validate it, and pass it to the policy pack.
			if !analyzerInfo.SupportsConfig {
				if len(policy.Config()) > 0 {
					logging.V(7).Infof("policy pack %q does not support config; skipping configure", analyzerInfo.Name)
				}
				return
			}
			configFromAPI, err := resourceanalyzer.ParsePolicyPackConfigFromAPI(policy.Config())
			if err != nil {
				errs <- err
				return
			}
			config, validationErrors, err := resourceanalyzer.ReconcilePolicyPackConfig(
				analyzerInfo.Policies, analyzerInfo.InitialConfig, configFromAPI)
			if err != nil {
				errs <- fmt.Errorf("reconciling config for %q: %w", analyzerInfo.Name, err)
				return
			}
			appendValidationErrors(analyzerInfo.Name, analyzerInfo.Version, validationErrors)
			if err = analyzer.Configure(config); err != nil {
				errs <- fmt.Errorf("configuring policy pack %q: %w", analyzerInfo.Name, err)
				return
			}
		}(policy, policyPath)
	}

	// Load local policy packs.
	for i, pack := range deployOpts.LocalPolicyPacks {
		wg.Add(1)
		go func(i int, pack LocalPolicyPack) {
			defer wg.Done()
			deployOpts.Events.PolicyLoadEvent()
			abs, err := filepath.Abs(pack.Path)
			if err != nil {
				errs <- err
				return
			}

			analyzer, err := plugctx.Host.PolicyAnalyzer(tokens.QName(abs), pack.Path, analyzerOpts)
			if err != nil {
				errs <- err
				return
			} else if analyzer == nil {
				errs <- fmt.Errorf("policy analyzer could not be loaded from path %q", pack.Path)
				return
			}

			// Update the Policy Pack names now that we have loaded the plugins and can access the name.
			analyzerInfo, err := analyzer.GetAnalyzerInfo()
			if err != nil {
				errs <- err
				return
			}

			// Read and store the name and version since it won't have been supplied by anyone else yet.
			deployOpts.LocalPolicyPacks[i].Name = analyzerInfo.Name
			deployOpts.LocalPolicyPacks[i].Version = analyzerInfo.Version

			// Load config, reconcile & validate it, and pass it to the policy pack.
			if !analyzerInfo.SupportsConfig {
				if pack.Config != "" {
					errs <- fmt.Errorf("policy pack %q at %q does not support config", analyzerInfo.Name, pack.Path)
					return
				}
				return
			}
			var configFromFile map[string]plugin.AnalyzerPolicyConfig
			if pack.Config != "" {
				configFromFile, err = resourceanalyzer.LoadPolicyPackConfigFromFile(pack.Config)
				if err != nil {
					errs <- err
					return
				}
			}
			config, validationErrors, err := resourceanalyzer.ReconcilePolicyPackConfig(
				analyzerInfo.Policies, analyzerInfo.InitialConfig, configFromFile)
			if err != nil {
				errs <- fmt.Errorf("reconciling policy config for %q at %q: %w", analyzerInfo.Name, pack.Path, err)
				return
			}
			appendValidationErrors(analyzerInfo.Name, analyzerInfo.Version, validationErrors)
			if err = analyzer.Configure(config); err != nil {
				errs <- fmt.Errorf("configuring policy pack %q at %q: %w", analyzerInfo.Name, pack.Path, err)
				return
			}
		}(i, pack)
	}

	wg.Wait()
	if len(errs) > 0 {
		// If we have any errors return the first one.  Even
		// if we have more than one error, we only return the
		// first to not overwhelm the user.
		return <-errs
	}

	// Report any policy config validation errors and return an error.
	if len(allValidationErrors) > 0 {
		sort.Strings(allValidationErrors)
		for _, validationError := range allValidationErrors {
			plugctx.Diag.Errorf(diag.Message("", validationError))
		}
		return errors.New("validating policy config")
	}

	return nil
}

func newUpdateSource(ctx context.Context,
	client deploy.BackendClient, opts *deploymentOptions, proj *workspace.Project, pwd, main, projectRoot string,
	target *deploy.Target, plugctx *plugin.Context,
) (deploy.Source, error) {
	//
	// Step 1: Install and load plugins.
	//

	allPlugins, defaultProviderVersions, err := installPlugins(
		ctx,
		proj,
		pwd,
		main,
		target,
		opts,
		plugctx,
		false, /*returnInstallErrors*/
	)
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
	analyzerOpts := &plugin.PolicyAnalyzerOptions{
		Organization: target.Organization.String(),
		Project:      proj.Name.String(),
		Stack:        target.Name.String(),
		Config:       config,
		DryRun:       opts.DryRun,
	}
	if err := installAndLoadPolicyPlugins(ctx, plugctx, opts, analyzerOpts); err != nil {
		return nil, err
	}

	// If we are connecting to an existing client, stash the address of the engine in its arguments.
	var args []string
	if proj.Runtime.Name() == clientRuntimeName {
		args = []string{plugctx.Host.ServerAddr()}
	}

	// If that succeeded, create a new source that will perform interpretation of the compiled program.
	return deploy.NewEvalSource(plugctx, &deploy.EvalRunInfo{
		Proj:        proj,
		Pwd:         pwd,
		Program:     main,
		ProjectRoot: projectRoot,
		Args:        args,
		Target:      target,
	}, defaultProviderVersions, deploy.EvalSourceOptions{
		DryRun:                    opts.DryRun,
		Parallel:                  opts.Parallel,
		DisableResourceReferences: opts.DisableResourceReferences,
		DisableOutputValues:       opts.DisableOutputValues,
		AttachDebugger:            opts.AttachDebugger,
	}), nil
}

func update(
	ctx *Context,
	info *deploymentContext,
	opts *deploymentOptions,
) (*deploy.Plan, display.ResourceChanges, error) {
	// Create an appropriate set of event listeners.
	var actions runActions
	if opts.DryRun {
		actions = newPreviewActions(opts)
	} else {
		actions = newUpdateActions(ctx, info.Update, opts)
	}

	// Initialize our deployment object with the context and options.
	deployment, err := newDeployment(ctx, info, actions, opts)
	if err != nil {
		return nil, nil, err
	}
	defer contract.IgnoreClose(deployment)

	// Execute the deployment.
	return deployment.run(ctx)
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
	Context *Context
	Steps   int32
	Ops     map[display.StepOp]int
	Seen    map[resource.URN]deploy.Step
	MapLock sync.Mutex
	Update  UpdateInfo
	Opts    *deploymentOptions

	maybeCorrupt bool
}

func newUpdateActions(context *Context, u UpdateInfo, opts *deploymentOptions) *updateActions {
	return &updateActions{
		Context: context,
		Ops:     make(map[display.StepOp]int),
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
	acts.Opts.Events.resourcePreEvent(step,
		false, /*planning*/
		acts.Opts.Debug,
		isInternalStep(step),
		acts.Opts.ShowSecrets,
	)

	// Inform the snapshot service that we are about to perform a step.
	return acts.Context.SnapshotManager.BeginMutation(step)
}

func (acts *updateActions) OnResourceStepPost(
	ctx interface{}, step deploy.Step,
	status resource.Status, err error,
) error {
	acts.MapLock.Lock()
	assertSeen(acts.Seen, step)
	acts.MapLock.Unlock()

	// If we've already been terminated, exit without writing the checkpoint. We explicitly want to leave the
	// checkpoint in an inconsistent state in this event.
	if acts.Context.Cancel.TerminateErr() != nil {
		return nil
	}

	isInternalStep := isInternalStep(step)

	// Report the result of the step.
	if err != nil {
		if status == resource.StatusUnknown {
			acts.maybeCorrupt = true
		}

		errorURN := resource.URN("")
		if !isInternalStep {
			errorURN = step.URN()
		}

		// Issue a true, bonafide error.
		acts.Opts.Diag.Errorf(diag.GetResourceOperationFailedError(errorURN), err)
		acts.Opts.Events.resourceOperationFailedEvent(step, status, acts.Steps, acts.Opts.Debug, acts.Opts.ShowSecrets)
	} else {
		op, record := step.Op(), step.Logical()
		if acts.Opts.isRefresh && op == deploy.OpRefresh {
			// Refreshes are handled specially.
			op, record = step.(*deploy.RefreshStep).ResultOp(), true
		}

		if step.Op() == deploy.OpRead {
			record = ShouldRecordReadStep(step)
		}

		if record && !isInternalStep {
			// Increment the counters.
			acts.MapLock.Lock()
			atomic.AddInt32(&acts.Steps, 1)
			acts.Ops[op]++
			acts.MapLock.Unlock()
		}

		// Also show outputs here for custom resources, since there might be some from the initial registration. We do
		// not show outputs for component resources at this point: any that exist must be from a previous execution of
		// the Pulumi program, as component resources only report outputs via calls to RegisterResourceOutputs.
		// Deletions emit the resourceOutputEvent so the display knows when to stop the time elapsed counter.
		if step.Res().Custom || acts.Opts.Refresh && step.Op() == deploy.OpRefresh || step.Op() == deploy.OpDelete {
			acts.Opts.Events.resourceOutputsEvent(
				op,
				step,
				false, /*planning*/
				acts.Opts.Debug,
				isInternalStep,
				acts.Opts.ShowSecrets,
			)
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
		contract.Assertf(new != nil, "new state should not be nil for partially-failed update")
		contract.Assertf(old != nil, "old state should not be nil for partially-failed update")
		new.Inputs = make(resource.PropertyMap)
		for key, value := range old.Inputs {
			new.Inputs[key] = value
		}
	}

	// Write out the current snapshot. Note that even if a failure has occurred, we should still have a
	// safe checkpoint.  Note that any error that occurs when writing the checkpoint trumps the error
	// reported above.
	return ctx.(SnapshotMutation).End(step, err == nil ||
		status == resource.StatusPartialFailure)
}

func (acts *updateActions) OnResourceOutputs(step deploy.Step) error {
	acts.MapLock.Lock()
	assertSeen(acts.Seen, step)
	acts.MapLock.Unlock()

	acts.Opts.Events.resourceOutputsEvent(
		step.Op(),
		step,
		false, /*planning*/
		acts.Opts.Debug,
		isInternalStep(step),
		acts.Opts.ShowSecrets,
	)

	// There's a chance there are new outputs that weren't written out last time.
	// We need to perform another snapshot write to ensure they get written out.
	return acts.Context.SnapshotManager.RegisterResourceOutputs(step)
}

func (acts *updateActions) OnPolicyViolation(urn resource.URN, d plugin.AnalyzeDiagnostic) {
	acts.Opts.Events.policyViolationEvent(urn, d)
}

func (acts *updateActions) OnPolicyRemediation(urn resource.URN, t plugin.Remediation,
	before resource.PropertyMap, after resource.PropertyMap,
) {
	acts.Opts.Events.policyRemediationEvent(urn, t, before, after)
}

func (acts *updateActions) MaybeCorrupt() bool {
	return acts.maybeCorrupt
}

func (acts *updateActions) Changes() display.ResourceChanges {
	return display.ResourceChanges(acts.Ops)
}

type previewActions struct {
	Ops     map[display.StepOp]int
	Opts    *deploymentOptions
	Seen    map[resource.URN]deploy.Step
	MapLock sync.Mutex
}

func isInternalStep(step deploy.Step) bool {
	return step.Op() == deploy.OpRemovePendingReplace || isDefaultProviderStep(step)
}

func ShouldRecordReadStep(step deploy.Step) bool {
	contract.Assertf(step.Op() == deploy.OpRead, "Only call this on a Read step")

	// If reading a resource didn't result in any change to the resource, we then want to
	// record this as a 'same'.  That way, when things haven't actually changed, but a user
	// app did any 'reads' these don't show up in the resource summary at the end.
	return step.Old() != nil &&
		step.New() != nil &&
		step.Old().Outputs != nil &&
		step.New().Outputs != nil &&
		step.Old().Outputs.Diff(step.New().Outputs) != nil
}

func newPreviewActions(opts *deploymentOptions) *previewActions {
	return &previewActions{
		Ops:  make(map[display.StepOp]int),
		Opts: opts,
		Seen: make(map[resource.URN]deploy.Step),
	}
}

func (acts *previewActions) OnResourceStepPre(step deploy.Step) (interface{}, error) {
	acts.MapLock.Lock()
	acts.Seen[step.URN()] = step
	acts.MapLock.Unlock()

	acts.Opts.Events.resourcePreEvent(
		step, true, /*planning*/
		acts.Opts.Debug,
		isInternalStep(step),
		acts.Opts.ShowSecrets,
	)

	return nil, nil
}

func (acts *previewActions) OnResourceStepPost(ctx interface{},
	step deploy.Step, status resource.Status, err error,
) error {
	acts.MapLock.Lock()
	assertSeen(acts.Seen, step)
	acts.MapLock.Unlock()

	isInternalStep := isInternalStep(step)

	if err != nil {
		// We always want to report a failure. If we intend to elide this step overall, though, we report it as a
		// global message.
		reportedURN := resource.URN("")
		if !isInternalStep {
			reportedURN = step.URN()
		}

		acts.Opts.Diag.Errorf(diag.GetPreviewFailedError(reportedURN), err)
	} else {
		op, record := step.Op(), step.Logical()
		if acts.Opts.isRefresh && op == deploy.OpRefresh {
			// Refreshes are handled specially.
			op, record = step.(*deploy.RefreshStep).ResultOp(), true
		}

		if step.Op() == deploy.OpRead {
			record = ShouldRecordReadStep(step)
		}

		// Track the operation if shown and/or if it is a logically meaningful operation.
		if record && !isInternalStep {
			acts.MapLock.Lock()
			acts.Ops[op]++
			acts.MapLock.Unlock()
		}

		acts.Opts.Events.resourceOutputsEvent(
			op,
			step,
			true, /*planning*/
			acts.Opts.Debug,
			isInternalStep,
			acts.Opts.ShowSecrets,
		)
	}

	return nil
}

func (acts *previewActions) OnResourceOutputs(step deploy.Step) error {
	acts.MapLock.Lock()
	assertSeen(acts.Seen, step)
	acts.MapLock.Unlock()

	// Print the resource outputs separately.
	acts.Opts.Events.resourceOutputsEvent(
		step.Op(),
		step,
		true, /*planning*/
		acts.Opts.Debug,
		isInternalStep(step),
		acts.Opts.ShowSecrets,
	)

	return nil
}

func (acts *previewActions) OnPolicyViolation(urn resource.URN, d plugin.AnalyzeDiagnostic) {
	acts.Opts.Events.policyViolationEvent(urn, d)
}

func (acts *previewActions) OnPolicyRemediation(urn resource.URN, t plugin.Remediation,
	before resource.PropertyMap, after resource.PropertyMap,
) {
	acts.Opts.Events.policyRemediationEvent(urn, t, before, after)
}

func (acts *previewActions) MaybeCorrupt() bool {
	return false
}

func (acts *previewActions) Changes() display.ResourceChanges {
	return display.ResourceChanges(acts.Ops)
}
