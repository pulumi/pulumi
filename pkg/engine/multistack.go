// Copyright 2016-2025, Pulumi Corporation.
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
	"time"

	"github.com/opentracing/opentracing-go"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// MultistackEntry represents a single stack participating in a multistack engine operation.
type MultistackEntry struct {
	Project *workspace.Project // per-stack project
	Target  *deploy.Target     // per-stack target (has snapshot, config, decrypter)
	Root    string             // project root directory
	FQN     string             // fully qualified stack name (org/project/stack)
}

// MultistackContext provides the execution context for a multistack engine operation.
type MultistackContext struct {
	Cancel           *context.Context
	Events           chan<- Event          // unified events channel
	SnapshotManagers map[string]SnapshotManager // stack FQN → per-stack manager (nil for preview)
	BackendClient    deploy.BackendClient
	ParentSpan       opentracing.SpanContext
	PluginManager    PluginManager
}

// MultistackUpdate runs a single unified deployment across N stacks.
// It creates N evalSources, wraps them in a MultiSource, merges snapshots,
// and executes one Deployment that processes all stacks' resources together.
func MultistackUpdate(
	entries []MultistackEntry,
	mctx *MultistackContext,
	opts UpdateOptions,
	dryRun bool,
) (display.ResourceChanges, error) {
	contract.Requiref(len(entries) > 0, "entries", "must have at least one entry")

	logging.V(4).Infof("MultistackUpdate: starting with %d entries, dryRun=%v", len(entries), dryRun)

	// Build the project-to-FQN map and co-deployed set for cross-stack resolution.
	projectToFQN := make(map[tokens.PackageName]string, len(entries))
	coDeployedFQNs := make([]string, 0, len(entries))
	coDeployedProjects := make(map[string]bool, len(entries))
	for _, entry := range entries {
		projectToFQN[entry.Project.Name] = entry.FQN
		coDeployedFQNs = append(coDeployedFQNs, entry.FQN)
		coDeployedProjects[entry.FQN] = true
	}

	// Create OutputWaiterStore for co-deployed stack output resolution.
	outputWaiters := deploy.NewOutputWaiterStore(coDeployedFQNs)

	// Create a tracing span.
	spanOpts := []opentracing.StartSpanOption{
		opentracing.Tag{Key: "operation", Value: "multistack-update"},
	}
	if mctx.ParentSpan != nil {
		spanOpts = append(spanOpts, opentracing.ChildOf(mctx.ParentSpan))
	}
	tracingSpan := opentracing.StartSpan("pulumi-multistack", spanOpts...)
	defer tracingSpan.Finish()

	// Create the event emitter for the unified display.
	emitter, err := makeEventEmitter(mctx.Events, UpdateInfo{
		Project: entries[0].Project,
		Target:  entries[0].Target,
		Root:    entries[0].Root,
	})
	if err != nil {
		return nil, err
	}
	defer emitter.Close()

	deployOpts := &deploymentOptions{
		UpdateOptions: opts,
		Events:        emitter,
		Diag:          newEventSink(emitter, false),
		StatusDiag:    newEventSink(emitter, true),
		DryRun:        dryRun,
		SourceFunc:    newUpdateSource,
		pluginManager: mctx.PluginManager,
	}

	// Create N sources.
	// For destroy operations, use NullSources (no programs to run — deletes come from old snapshot).
	// For update/preview, use evalSources (each runs its own program).
	sources := make([]deploy.Source, len(entries))
	plugctxs := make([]*plugin.Context, len(entries))

	if opts.DestroyProgram {
		logging.V(4).Infof("MultistackUpdate: creating null sources for destroy")
		for i, entry := range entries {
			sources[i] = deploy.NewNullSource(entry.Project.Name)
		}

		// Create a plugin context for the deployment (needed for provider operations during destroy).
		projinfo := &Projinfo{Proj: entries[0].Project, Root: entries[0].Root}
		_, _, plugctx, err := ProjectInfoContext(
			projinfo, opts.Host,
			deployOpts.Diag, deployOpts.StatusDiag,
			nil, /* debugContext */
			opts.DisableProviderPreview, tracingSpan, nil, /* config */
		)
		if err != nil {
			return nil, fmt.Errorf("creating plugin context for destroy: %w", err)
		}
		plugctxs[0] = plugctx
	} else {
		logging.V(4).Infof("MultistackUpdate: creating eval sources")
		panicErrs := make(chan error)

		for i, entry := range entries {
			projinfo := &Projinfo{Proj: entry.Project, Root: entry.Root}

			// Decrypt config for the plugin context.
			decConfig, err := entry.Target.Config.Decrypt(entry.Target.Decrypter)
			if err != nil {
				return nil, fmt.Errorf("decrypting config for %s: %w", entry.FQN, err)
			}

			pwd, main, plugctx, err := ProjectInfoContext(
				projinfo, opts.Host,
				deployOpts.Diag, deployOpts.StatusDiag,
				nil, /* debugContext */
				opts.DisableProviderPreview, tracingSpan, decConfig,
			)
			if err != nil {
				return nil, fmt.Errorf("creating plugin context for %s: %w", entry.FQN, err)
			}
			plugctxs[i] = plugctx

			// Gather default provider versions for this stack.
			_, defaultProviderVersions, err := installPlugins(
				context.Background(), entry.Project, pwd, main,
				entry.Target, deployOpts, plugctx,
				false, nil,
			)
			if err != nil {
				return nil, fmt.Errorf("gathering plugins for %s: %w", entry.FQN, err)
			}

			resourceHooks := deploy.NewResourceHooks(plugctx.DialOptions)

			source := deploy.NewEvalSource(plugctx, &deploy.EvalRunInfo{
				Proj:        entry.Project,
				Pwd:         pwd,
				Program:     main,
				ProjectRoot: entry.Root,
				Target:      entry.Target,
			}, defaultProviderVersions, resourceHooks, deploy.EvalSourceOptions{
				DryRun:                    dryRun,
				Parallel:                  opts.Parallel,
				DisableResourceReferences: opts.DisableResourceReferences,
				DisableOutputValues:       opts.DisableOutputValues,
			}, panicErrs)

			sources[i] = source
		}
	}

	// Clean up plugin contexts when done.
	defer func() {
		for _, ctx := range plugctxs {
			if ctx != nil {
				contract.IgnoreClose(ctx)
			}
		}
	}()

	// Step 3: Wrap in MultiSource.
	multiSource := deploy.NewMultiSource(sources)

	// Step 4: Merge snapshots.
	logging.V(4).Infof("MultistackUpdate: merging snapshots")
	snapshots := make([]*deploy.Snapshot, len(entries))
	perStackSnapshots := make(map[string]*deploy.Snapshot, len(entries))
	for i, entry := range entries {
		snapshots[i] = entry.Target.Snapshot
		perStackSnapshots[entry.FQN] = entry.Target.Snapshot
	}
	mergedSnapshot := deploy.MergeSnapshots(snapshots, coDeployedProjects)

	// Step 5: Create synthetic target with merged snapshot.
	syntheticTarget := &deploy.Target{
		Name:      entries[0].Target.Name,
		Config:    config.Map{}, // each source has its own config
		Decrypter: config.NopDecrypter,
		Snapshot:  mergedSnapshot,
	}

	// Step 6: Set up deploy.Options with OutputWaiters.
	deplOpts := &deploy.Options{
		DryRun:                    dryRun,
		Parallel:                  opts.Parallel,
		Refresh:                   opts.Refresh,
		UseLegacyDiff:             opts.UseLegacyDiff,
		UseLegacyRefreshDiff:      opts.UseLegacyRefreshDiff,
		DisableResourceReferences: opts.DisableResourceReferences,
		DisableOutputValues:       opts.DisableOutputValues,
		ContinueOnError:           opts.ContinueOnError,
		OutputWaiters:             outputWaiters,
		OutputWaitersStackFQNs:    projectToFQN,
	}

	// Step 7: Create SnapshotManager.
	var snapshotMgr SnapshotManager
	if mctx.SnapshotManagers != nil && len(mctx.SnapshotManagers) > 0 {
		snapshotMgr = NewRoutingSnapshotManager(
			mctx.SnapshotManagers,
			projectToFQN,
			perStackSnapshots,
		)
	}

	// Step 8: Create actions (preview or update).
	var actions runActions
	if dryRun {
		actions = newMultistackPreviewActions(mctx.Events, deployOpts, snapshotMgr)
	} else {
		actions = newMultistackUpdateActions(mctx.Events, deployOpts, snapshotMgr)
	}

	// Step 9: Create ONE Deployment.
	logging.V(4).Infof("MultistackUpdate: creating unified deployment")

	// Use the first entry's plugin context as the "master" context for the deployment.
	depl, err := deploy.NewDeployment(
		plugctxs[0], deplOpts, actions, syntheticTarget, mergedSnapshot,
		nil, /* plan */
		multiSource,
		nil, /* localPolicyPackPaths */
		mctx.BackendClient,
		nil, /* resourceHooks - each source has its own */
	)
	if err != nil {
		return nil, fmt.Errorf("creating deployment: %w", err)
	}
	defer contract.IgnoreClose(depl)

	// Step 10: Execute the deployment.
	logging.V(4).Infof("MultistackUpdate: executing deployment")

	// Emit prelude event.
	emitter.preludeEvent(dryRun, syntheticTarget.Config)

	start := time.Now()
	_, walkErr := depl.Execute(context.Background())
	duration := time.Since(start)

	changes := actions.Changes()

	// Emit summary event.
	emitter.summaryEvent(dryRun, actions.MaybeCorrupt(), duration, changes, nil)

	// Close the snapshot manager.
	if snapshotMgr != nil {
		if err := snapshotMgr.Close(); err != nil {
			logging.V(4).Infof("MultistackUpdate: error closing snapshot manager: %v", err)
		}
	}

	return changes, walkErr
}

// MultistackDestroy runs a unified destroy across N stacks.
// For destroy, the sources are N NullSources. The deployment runs in destroy mode,
// generating delete steps from the merged old snapshot.
func MultistackDestroy(
	entries []MultistackEntry,
	mctx *MultistackContext,
	opts UpdateOptions,
	dryRun bool,
) (display.ResourceChanges, error) {
	// For destroy, use NullSources and set DestroyProgram.
	opts.DestroyProgram = true
	return MultistackUpdate(entries, mctx, opts, dryRun)
}

// multistackPreviewActions implements runActions for multistack preview operations.
type multistackPreviewActions struct {
	events      chan<- Event
	opts        *deploymentOptions
	snapshotMgr SnapshotManager
	ops         map[display.StepOp]int
}

func newMultistackPreviewActions(
	events chan<- Event,
	opts *deploymentOptions,
	snapshotMgr SnapshotManager,
) *multistackPreviewActions {
	return &multistackPreviewActions{
		events:      events,
		opts:        opts,
		snapshotMgr: snapshotMgr,
		ops:         make(map[display.StepOp]int),
	}
}

func (a *multistackPreviewActions) OnSnapshotWrite(snap *deploy.Snapshot) error {
	if a.snapshotMgr != nil {
		return a.snapshotMgr.Write(snap)
	}
	return nil
}

func (a *multistackPreviewActions) OnRebuiltBaseState() error {
	if a.snapshotMgr != nil {
		return a.snapshotMgr.RebuiltBaseState()
	}
	return nil
}

func (a *multistackPreviewActions) OnResourceStepPre(step deploy.Step) (any, error) {
	a.opts.Events.resourcePreEvent(step, true /*planning*/, a.opts.Debug, isInternalStep(step), a.opts.ShowSecrets)
	if a.snapshotMgr != nil {
		return a.snapshotMgr.BeginMutation(step)
	}
	return nil, nil
}

func (a *multistackPreviewActions) OnResourceStepPost(
	ctx any, step deploy.Step, status resource.Status, err error,
) error {
	a.opts.Events.resourceOutputsEvent(
		step.Op(), step, true /*planning*/, a.opts.Debug, isInternalStep(step), a.opts.ShowSecrets)
	if step.Op() != deploy.OpSame && step.Logical() && !isInternalStep(step) {
		a.ops[step.Op()]++
	}
	if ctx != nil {
		return ctx.(SnapshotMutation).End(step, err == nil)
	}
	return nil
}

func (a *multistackPreviewActions) OnResourceOutputs(step deploy.Step) error {
	a.opts.Events.resourceOutputsEvent(
		step.Op(), step, true /*planning*/, a.opts.Debug, isInternalStep(step), a.opts.ShowSecrets)
	if a.snapshotMgr != nil {
		return a.snapshotMgr.RegisterResourceOutputs(step)
	}
	return nil
}

func (a *multistackPreviewActions) OnPolicyViolation(urn resource.URN, d plugin.AnalyzeDiagnostic) {
	a.opts.Events.policyViolationEvent(urn, d)
}
func (a *multistackPreviewActions) OnPolicyRemediation(
	urn resource.URN, r plugin.Remediation, before, after resource.PropertyMap,
) {
	a.opts.Events.policyRemediationEvent(urn, r, before, after)
}
func (a *multistackPreviewActions) OnPolicyAnalyzeSummary(summary plugin.PolicySummary) {}
func (a *multistackPreviewActions) OnPolicyRemediateSummary(summary plugin.PolicySummary) {}
func (a *multistackPreviewActions) OnPolicyAnalyzeStackSummary(summary plugin.PolicySummary) {}

func (a *multistackPreviewActions) Changes() display.ResourceChanges {
	return a.ops
}

func (a *multistackPreviewActions) MaybeCorrupt() bool {
	return false
}

// multistackUpdateActions implements runActions for multistack update operations.
type multistackUpdateActions struct {
	events       chan<- Event
	opts         *deploymentOptions
	snapshotMgr  SnapshotManager
	ops          map[display.StepOp]int
	maybeCorrupt bool
}

func newMultistackUpdateActions(
	events chan<- Event,
	opts *deploymentOptions,
	snapshotMgr SnapshotManager,
) *multistackUpdateActions {
	return &multistackUpdateActions{
		events:      events,
		opts:        opts,
		snapshotMgr: snapshotMgr,
		ops:         make(map[display.StepOp]int),
	}
}

func (a *multistackUpdateActions) OnSnapshotWrite(snap *deploy.Snapshot) error {
	if a.snapshotMgr != nil {
		return a.snapshotMgr.Write(snap)
	}
	return nil
}

func (a *multistackUpdateActions) OnRebuiltBaseState() error {
	if a.snapshotMgr != nil {
		return a.snapshotMgr.RebuiltBaseState()
	}
	return nil
}

func (a *multistackUpdateActions) OnResourceStepPre(step deploy.Step) (any, error) {
	a.opts.Events.resourcePreEvent(step, false /*planning*/, a.opts.Debug, isInternalStep(step), a.opts.ShowSecrets)
	if a.snapshotMgr != nil {
		return a.snapshotMgr.BeginMutation(step)
	}
	return nil, nil
}

func (a *multistackUpdateActions) OnResourceStepPost(
	ctx any, step deploy.Step, status resource.Status, err error,
) error {
	if err != nil && status == resource.StatusUnknown {
		a.maybeCorrupt = true
	}

	if err == nil {
		op, record := step.Op(), step.Logical()
		if record && !isInternalStep(step) {
			a.ops[op]++
		}
		if step.Res().Custom || step.Op() == deploy.OpDelete {
			a.opts.Events.resourceOutputsEvent(
				op, step, false, a.opts.Debug, isInternalStep(step), a.opts.ShowSecrets)
		}
	}

	if ctx != nil {
		return ctx.(SnapshotMutation).End(step, err == nil || status == resource.StatusPartialFailure)
	}
	return nil
}

func (a *multistackUpdateActions) OnResourceOutputs(step deploy.Step) error {
	a.opts.Events.resourceOutputsEvent(
		step.Op(), step, false, a.opts.Debug, isInternalStep(step), a.opts.ShowSecrets)
	if a.snapshotMgr != nil {
		return a.snapshotMgr.RegisterResourceOutputs(step)
	}
	return nil
}

func (a *multistackUpdateActions) OnPolicyViolation(urn resource.URN, d plugin.AnalyzeDiagnostic) {
	a.opts.Events.policyViolationEvent(urn, d)
}
func (a *multistackUpdateActions) OnPolicyRemediation(
	urn resource.URN, r plugin.Remediation, before, after resource.PropertyMap,
) {
	a.opts.Events.policyRemediationEvent(urn, r, before, after)
}
func (a *multistackUpdateActions) OnPolicyAnalyzeSummary(summary plugin.PolicySummary) {}
func (a *multistackUpdateActions) OnPolicyRemediateSummary(summary plugin.PolicySummary) {}
func (a *multistackUpdateActions) OnPolicyAnalyzeStackSummary(summary plugin.PolicySummary) {}

func (a *multistackUpdateActions) Changes() display.ResourceChanges {
	return a.ops
}

func (a *multistackUpdateActions) MaybeCorrupt() bool {
	return a.maybeCorrupt
}
