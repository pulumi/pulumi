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

package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	sdkDisplay "github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// MultistackEntry represents a single stack participating in a multistack operation.
type MultistackEntry struct {
	// Stack is the resolved stack object.
	Stack Stack
	// Op is the update operation configuration for this stack.
	Op UpdateOperation
	// Dir is the project directory path (for display purposes).
	Dir string
}

// MultistackOptions configures a multistack operation.
type MultistackOptions struct {
	// FailFast halts all stacks immediately on any failure.
	FailFast bool
	// DisplayOpts controls how the unified display renders events.
	DisplayOpts display.Options
	// Engine contains engine-level options (parallelism, debug, etc.).
	Engine engine.UpdateOptions
}

// PerStackSnapshotProvider can set up per-stack snapshot managers for single-engine
// multistack deployments. This is implemented by the Pulumi Cloud backend to enable
// resource-level cross-stack parallelism (vs. the legacy stack-level approach).
type PerStackSnapshotProvider interface {
	// SetupPerStackSnapshots creates update records and snapshot managers for each stack.
	// Returns a map from stack FQN to SnapshotManager, a map of per-stack snapshots (loaded
	// without integrity checking for multistack compatibility), and a completion function that
	// must be called with the final update status to finalize update records and release leases.
	SetupPerStackSnapshots(
		ctx context.Context,
		kind apitype.UpdateKind,
		entries []MultistackEntry,
		dryRun bool,
	) (managers map[string]engine.SnapshotManager, snapshots map[string]*deploy.Snapshot,
		complete func(apitype.UpdateStatus) error, err error)
}

// MultistackResult holds the results for a single stack in a multistack operation.
type MultistackResult struct {
	// Changes is the set of resource changes for this stack.
	Changes sdkDisplay.ResourceChanges
	// Plan is the deployment plan (for preview operations).
	Plan *deploy.Plan
	// Error is any error that occurred during the operation.
	Error error
	// Events collected during the operation (for "details" display in confirmation prompt).
	Events []engine.Event
}

// MultistackPreview runs a unified preview across multiple stacks using a single engine deployment.
// All stacks' programs run concurrently with resource-level interleaving.
func MultistackPreview(
	ctx context.Context,
	entries []MultistackEntry,
	opts MultistackOptions,
) (map[string]*MultistackResult, error) {
	return runMultistackPreviewViaEngine(ctx, entries, opts, false /* isDestroy */)
}

// MultistackUpdate runs an update across multiple stacks.
// If the backend supports per-stack snapshots, uses the single-engine path for
// resource-level cross-stack parallelism. Otherwise falls back to per-stack operations.
func MultistackUpdate(
	ctx context.Context,
	entries []MultistackEntry,
	opts MultistackOptions,
) (map[string]*MultistackResult, error) {
	if provider, ok := entries[0].Stack.Backend().(PerStackSnapshotProvider); ok {
		return runMultistackUpdateViaEngine(ctx, entries, opts, provider, false /* isDestroy */)
	}
	return runMultistackOperation(ctx, entries, opts, operationUpdate)
}

// MultistackDestroyPreview runs a unified destroy preview across multiple stacks.
func MultistackDestroyPreview(
	ctx context.Context,
	entries []MultistackEntry,
	opts MultistackOptions,
) (map[string]*MultistackResult, error) {
	return runMultistackPreviewViaEngine(ctx, entries, opts, true /* isDestroy */)
}

// MultistackDestroy destroys multiple stacks.
// If the backend supports per-stack snapshots, uses the single-engine path.
// Otherwise falls back to per-stack operations.
func MultistackDestroy(
	ctx context.Context,
	entries []MultistackEntry,
	opts MultistackOptions,
) (map[string]*MultistackResult, error) {
	if provider, ok := entries[0].Stack.Backend().(PerStackSnapshotProvider); ok {
		return runMultistackUpdateViaEngine(ctx, entries, opts, provider, true /* isDestroy */)
	}
	return runMultistackOperation(ctx, entries, opts, operationDestroy)
}

// runMultistackPreviewViaEngine runs a unified preview through a single engine deployment.
// This is the new single-engine path: N programs run concurrently in one Deployment with
// a single step generator and step executor. Resources from all stacks are interleaved.
func runMultistackPreviewViaEngine(
	ctx context.Context,
	entries []MultistackEntry,
	opts MultistackOptions,
	isDestroy bool,
) (map[string]*MultistackResult, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("no stacks specified for multistack operation")
	}

	// Build engine-level multistack entries.
	engineEntries := make([]engine.MultistackEntry, len(entries))
	for i, entry := range entries {
		fqn := string(entry.Stack.Ref().FullyQualifiedName())

		// Load the snapshot without integrity checking. Per-stack snapshots may
		// contain cross-stack dependency references from previous multistack runs.
		snapshot := loadMultistackSnapshot(ctx, entry.Stack, entry.Op.SecretsProvider)

		// Build the deploy target.
		target := &deploy.Target{
			Name:      entry.Stack.Ref().Name(),
			Config:    entry.Op.StackConfiguration.Config,
			Decrypter: entry.Op.StackConfiguration.Decrypter,
			Snapshot:  snapshot,
		}

		engineEntries[i] = engine.MultistackEntry{
			Project: entry.Op.Proj,
			Target:  target,
			Root:    entry.Op.Root,
			FQN:     fqn,
		}
	}

	// Determine display settings.
	var action apitype.UpdateKind
	if isDestroy {
		action = apitype.DestroyUpdate
	} else {
		action = apitype.PreviewUpdate
	}
	label := ActionLabel(action, true /* isPreview */)

	// Create unified event channel and display.
	unifiedEvents := make(chan engine.Event)
	displayDone := make(chan bool)

	displayOpts := opts.DisplayOpts
	displayOpts.SuppressPermalink = true
	displayOpts.StackLabels = buildStackLabels(entries)
	displayOpts.ExpectedStackURNs = buildExpectedStackURNs(entries)

	go display.ShowEvents(
		strings.ToLower(label), action,
		tokens.StackName{}, "",
		"", unifiedEvents, displayDone, displayOpts, true /* isPreview */)

	// Collect events for each stack result.
	engineEvents := make(chan engine.Event)
	var collectedEvents []engine.Event
	var forwardWg sync.WaitGroup
	forwardWg.Add(1)
	go func() {
		defer forwardWg.Done()
		for e := range engineEvents {
			if !e.Internal() {
				if e.Type == engine.ResourcePreEvent ||
					e.Type == engine.ResourceOutputsEvent ||
					e.Type == engine.PolicyRemediationEvent {
					collectedEvents = append(collectedEvents, e)
				}
			}
			unifiedEvents <- e
		}
	}()

	// Use engine options from the multistack options.
	engineOpts := opts.Engine
	if isDestroy {
		engineOpts.DestroyProgram = true
	}

	// Build the multistack context.
	mctx := &engine.MultistackContext{
		Events:        engineEvents,
		BackendClient: NewBackendClient(entries[0].Stack.Backend(), entries[0].Op.SecretsProvider),
	}

	// Call the unified engine.
	changes, err := engine.MultistackUpdate(engineEntries, mctx, engineOpts, true /* dryRun */)

	close(engineEvents)
	forwardWg.Wait()
	close(unifiedEvents)
	<-displayDone

	// For the unified engine path, errors are not per-stack — return directly.
	if err != nil {
		return nil, err
	}

	// Build per-stack results for the confirmation prompt's "details" view.
	results := make(map[string]*MultistackResult, len(entries))
	for _, entry := range entries {
		fqn := string(entry.Stack.Ref().FullyQualifiedName())
		results[fqn] = &MultistackResult{
			Changes: changes,
			Events:  collectedEvents,
		}
	}

	return results, nil
}

// runMultistackUpdateViaEngine runs a real update or destroy through a single engine deployment.
// Unlike the preview path, this sets up per-stack snapshot managers via the PerStackSnapshotProvider
// so that resource state is persisted back to each stack independently.
func runMultistackUpdateViaEngine(
	ctx context.Context,
	entries []MultistackEntry,
	opts MultistackOptions,
	provider PerStackSnapshotProvider,
	isDestroy bool,
) (map[string]*MultistackResult, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("no stacks specified for multistack operation")
	}

	// Set up per-stack snapshot managers via the cloud backend.
	var action apitype.UpdateKind
	if isDestroy {
		action = apitype.DestroyUpdate
	} else {
		action = apitype.UpdateUpdate
	}

	managers, perStackSnaps, complete, err := provider.SetupPerStackSnapshots(ctx, action, entries, false /* dryRun */)
	if err != nil {
		return nil, fmt.Errorf("setting up per-stack snapshots: %w", err)
	}
	// Ensure we always complete the update records, even on error.
	var updateErr error
	defer func() {
		status := apitype.UpdateStatusSucceeded
		if updateErr != nil {
			status = apitype.UpdateStatusFailed
		}
		if completeErr := complete(status); completeErr != nil {
			logging.V(4).Infof("multistack: error completing updates: %v", completeErr)
		}
	}()

	// Build engine-level multistack entries using snapshots from SetupPerStackSnapshots
	// (which loads them without integrity checking for multistack compatibility).
	engineEntries := make([]engine.MultistackEntry, len(entries))
	for i, entry := range entries {
		fqn := string(entry.Stack.Ref().FullyQualifiedName())
		target := &deploy.Target{
			Name:      entry.Stack.Ref().Name(),
			Config:    entry.Op.StackConfiguration.Config,
			Decrypter: entry.Op.StackConfiguration.Decrypter,
			Snapshot:  perStackSnaps[fqn],
		}
		engineEntries[i] = engine.MultistackEntry{
			Project: entry.Op.Proj,
			Target:  target,
			Root:    entry.Op.Root,
			FQN:     fqn,
		}
	}

	// Set up display.
	label := ActionLabel(action, false /* isPreview */)
	unifiedEvents := make(chan engine.Event)
	displayDone := make(chan bool)

	displayOpts := opts.DisplayOpts
	displayOpts.SuppressPermalink = true
	displayOpts.StackLabels = buildStackLabels(entries)
	displayOpts.ExpectedStackURNs = buildExpectedStackURNs(entries)

	go display.ShowEvents(
		strings.ToLower(label), action,
		tokens.StackName{}, "",
		"", unifiedEvents, displayDone, displayOpts, false /* isPreview */)

	// Forward engine events to the unified display.
	engineEvents := make(chan engine.Event)
	var collectedEvents []engine.Event
	var forwardWg sync.WaitGroup
	forwardWg.Add(1)
	go func() {
		defer forwardWg.Done()
		for e := range engineEvents {
			if !e.Internal() {
				if e.Type == engine.ResourcePreEvent ||
					e.Type == engine.ResourceOutputsEvent ||
					e.Type == engine.PolicyRemediationEvent {
					collectedEvents = append(collectedEvents, e)
				}
			}
			unifiedEvents <- e
		}
	}()

	// Build engine options.
	engineOpts := opts.Engine
	if isDestroy {
		engineOpts.DestroyProgram = true
	}

	// Build the multistack context with per-stack snapshot managers.
	mctx := &engine.MultistackContext{
		Events:           engineEvents,
		SnapshotManagers: managers,
		BackendClient:    NewBackendClient(entries[0].Stack.Backend(), entries[0].Op.SecretsProvider),
	}

	// Run the single-engine deployment.
	changes, updateErr := engine.MultistackUpdate(engineEntries, mctx, engineOpts, false /* dryRun */)

	close(engineEvents)
	forwardWg.Wait()
	close(unifiedEvents)
	<-displayDone

	if updateErr != nil {
		return nil, updateErr
	}

	// Build per-stack results.
	results := make(map[string]*MultistackResult, len(entries))
	for _, entry := range entries {
		fqn := string(entry.Stack.Ref().FullyQualifiedName())
		results[fqn] = &MultistackResult{
			Changes: changes,
			Events:  collectedEvents,
		}
	}

	return results, nil
}

type operationType int

const (
	operationPreview operationType = iota
	operationUpdate
	operationDestroy
	operationDestroyPreview
)

// runMultistackOperation orchestrates a multistack operation using per-stack backend operations.
// This is the legacy path used for update and destroy until MultistackBackend support is added.
func runMultistackOperation(
	ctx context.Context,
	entries []MultistackEntry,
	opts MultistackOptions,
	opType operationType,
) (map[string]*MultistackResult, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("no stacks specified for multistack operation")
	}

	// Build a map from fully qualified stack name to entry for quick lookup.
	entryMap := make(map[string]*MultistackEntry, len(entries))
	for i := range entries {
		ref := entries[i].Stack.Ref().FullyQualifiedName()
		key := string(ref)
		if _, exists := entryMap[key]; exists {
			return nil, fmt.Errorf("duplicate stack %q in multistack operation", key)
		}
		entryMap[key] = &entries[i]
	}

	// Build dependency graph from StackReference resources in previous snapshots.
	deps, err := buildMultistackDependencyGraph(ctx, entries, entryMap)
	if err != nil {
		return nil, fmt.Errorf("building dependency graph: %w", err)
	}

	// Topologically sort the stacks.
	levels, err := topologicalSort(entries, deps)
	if err != nil {
		return nil, err
	}
	logging.V(4).Infof("multistack: topological levels (before reversal): %v", levels)

	// For destroy operations, reverse the order (destroy downstream first).
	if opType == operationDestroy || opType == operationDestroyPreview {
		for i, j := 0, len(levels)-1; i < j; i, j = i+1, j-1 {
			levels[i], levels[j] = levels[j], levels[i]
		}
		logging.V(4).Infof("multistack: topological levels (after destroy reversal): %v", levels)
	}

	// Create an OutputWaiterStore for co-deployed stack output resolution.
	coDeployedNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		coDeployedNames = append(coDeployedNames, string(entry.Stack.Ref().FullyQualifiedName()))
	}
	outputWaiters := deploy.NewOutputWaiterStore(coDeployedNames)
	logging.V(4).Infof("multistack: created OutputWaiterStore with co-deployed stacks: %v", coDeployedNames)

	// Set the output waiter store on each entry's engine options.
	for i := range entries {
		key := string(entries[i].Stack.Ref().FullyQualifiedName())
		entries[i].Op.Opts.Engine.OutputWaiters = outputWaiters
		entries[i].Op.Opts.Engine.OutputWaitersStackName = key
		logging.V(4).Infof("multistack: set OutputWaiters on entry %q (ptr=%p)", key, outputWaiters)
	}

	// Determine the action kind and whether this is a preview for display purposes.
	isPreview := opType == operationPreview || opType == operationDestroyPreview
	var action apitype.UpdateKind
	switch opType {
	case operationPreview:
		action = apitype.PreviewUpdate
	case operationUpdate:
		action = apitype.UpdateUpdate
	case operationDestroy:
		action = apitype.DestroyUpdate
	case operationDestroyPreview:
		action = apitype.DestroyUpdate
	}
	label := ActionLabel(action, isPreview)

	// Create unified event channel and display goroutine.
	unifiedEvents := make(chan engine.Event)
	displayDone := make(chan bool)

	displayOpts := opts.DisplayOpts
	displayOpts.SuppressPermalink = true
	displayOpts.StackLabels = buildStackLabels(entries)
	displayOpts.ExpectedStackURNs = buildExpectedStackURNs(entries)

	go display.ShowEvents(
		strings.ToLower(label), action,
		tokens.StackName{}, "",
		"", unifiedEvents, displayDone, displayOpts, isPreview)

	// Execute stacks level by level.
	startTime := time.Now()
	results := make(map[string]*MultistackResult, len(entries))
	failed := make(map[string]bool)

	isDestroyOp := opType == operationDestroy || opType == operationDestroyPreview

	if isDestroyOp {
		// For destroy operations, we must execute level-by-level in reverse topological order
		// so that dependents are destroyed before their dependencies. The OutputWaiterStore
		// cannot help with ordering here since there are no outputs to wait for during destroy.
		for levelIdx, level := range levels {
			logging.V(4).Infof("multistack: executing destroy level %d with %d stacks", levelIdx, len(level))

			var skippedInLevel []string
			var runnableInLevel []string
			for _, key := range level {
				if shouldSkip(key, deps, failed) {
					skippedInLevel = append(skippedInLevel, key)
				} else {
					runnableInLevel = append(runnableInLevel, key)
				}
			}

			for _, key := range skippedInLevel {
				results[key] = &MultistackResult{
					Error: fmt.Errorf("skipped: dependency failed"),
				}
				failed[key] = true
			}

			if opts.FailFast && len(failed) > 0 {
				for _, key := range runnableInLevel {
					results[key] = &MultistackResult{
						Error: fmt.Errorf("skipped: fail-fast mode and a prior stack failed"),
					}
				}
				continue
			}

			var wg sync.WaitGroup
			var mu sync.Mutex
			for _, key := range runnableInLevel {
				wg.Add(1)
				go func(key string) {
					defer wg.Done()
					entry := entryMap[key]
					result := executeStackOperation(ctx, entry, opType, unifiedEvents)
					mu.Lock()
					results[key] = result
					if result.Error != nil {
						failed[key] = true
					}
					mu.Unlock()
				}(key)
			}
			wg.Wait()
		}
	} else {
		// For update/preview, launch all stacks concurrently. Cross-stack dependencies are
		// handled by the OutputWaiterStore: when stack A publishes its outputs, any stack B
		// that reads a StackReference to A unblocks immediately — giving us resource-level
		// parallelism across stacks rather than waiting for entire stacks to complete.
		var wg sync.WaitGroup
		var mu sync.Mutex
		allKeys := make([]string, 0, len(entryMap))
		for key := range entryMap {
			allKeys = append(allKeys, key)
		}
		for _, key := range allKeys {
			wg.Add(1)
			go func(key string) {
				defer wg.Done()
				entry := entryMap[key]
				result := executeStackOperation(ctx, entry, opType, unifiedEvents)

				if result.Error != nil {
					outputWaiters.FailStack(key, result.Error)
				}

				mu.Lock()
				results[key] = result
				if result.Error != nil {
					failed[key] = true
				}
				mu.Unlock()
			}(key)
		}
		wg.Wait()
	}

	// Send an aggregated SummaryEvent before closing the unified channel.
	sendAggregatedSummary(results, unifiedEvents, isPreview, time.Since(startTime))

	close(unifiedEvents)
	<-displayDone

	return results, nil
}

// sendAggregatedSummary sends a single SummaryEvent to the unified display channel
// that aggregates changes across all stacks.
func sendAggregatedSummary(
	results map[string]*MultistackResult, events chan<- engine.Event,
	isPreview bool, duration time.Duration,
) {
	changes := make(sdkDisplay.ResourceChanges)
	for _, result := range results {
		if result.Changes != nil {
			for op, count := range result.Changes {
				changes[op] += count
			}
		}
	}
	events <- engine.NewEvent(engine.SummaryEventPayload{
		IsPreview:       isPreview,
		Duration:        duration,
		ResourceChanges: changes,
	})
}

// executeStackOperation runs a single stack's operation (preview, update, or destroy),
// forwarding events to the unified display channel.
func executeStackOperation(
	ctx context.Context,
	entry *MultistackEntry,
	opType operationType,
	unifiedEvents chan<- engine.Event,
) *MultistackResult {
	// Suppress per-stack display; we use the unified display.
	entry.Op.Opts.Display.SuppressDisplay = true
	entry.Op.Opts.Display.SuppressPermalink = true

	// Create a per-stack event channel that forwards to the unified channel
	// and collects events for the confirmation prompt's "details" view.
	events := make(chan engine.Event)
	var collectedEvents []engine.Event
	var forwardWg sync.WaitGroup
	forwardWg.Add(1)
	go func() {
		defer forwardWg.Done()
		for e := range events {
			// Collect relevant events for the "details" diff display.
			if !e.Internal() {
				if e.Type == engine.ResourcePreEvent ||
					e.Type == engine.ResourceOutputsEvent ||
					e.Type == engine.PolicyRemediationEvent ||
					e.Type == engine.SummaryEvent {
					collectedEvents = append(collectedEvents, e)
				}
			}
			// Filter per-stack SummaryEvent and CancelEvent — we send our own aggregated one.
			if e.Type == engine.SummaryEvent || e.Type == engine.CancelEvent {
				continue
			}
			unifiedEvents <- e
		}
	}()

	result := &MultistackResult{}
	switch opType {
	case operationPreview:
		plan, changes, err := PreviewStack(ctx, entry.Stack, entry.Op, events)
		result.Plan = plan
		result.Changes = changes
		result.Error = err
	case operationUpdate:
		// Skip preview & auto-approve — the multistack orchestrator handles prompting.
		entry.Op.Opts.SkipPreview = true
		entry.Op.Opts.AutoApprove = true
		entry.Op.Opts.PreviewOnly = false
		changes, err := UpdateStack(ctx, entry.Stack, entry.Op, events)
		result.Changes = changes
		result.Error = err
	case operationDestroy:
		// Skip preview & auto-approve — the multistack orchestrator handles prompting.
		entry.Op.Opts.SkipPreview = true
		entry.Op.Opts.AutoApprove = true
		entry.Op.Opts.PreviewOnly = false
		changes, err := DestroyStack(ctx, entry.Stack, entry.Op, events)
		result.Changes = changes
		result.Error = err
	case operationDestroyPreview:
		entry.Op.Opts.Engine.DestroyProgram = true
		plan, changes, err := PreviewStack(ctx, entry.Stack, entry.Op, events)
		result.Plan = plan
		result.Changes = changes
		result.Error = err
	}

	close(events)
	forwardWg.Wait()

	result.Events = collectedEvents

	return result
}

// loadMultistackSnapshot loads a stack's snapshot with integrity checking temporarily disabled.
// Per-stack snapshots from previous multistack runs may contain cross-stack dependency references
// that would fail normal integrity verification. Returns nil if the snapshot can't be loaded.
func loadMultistackSnapshot(ctx context.Context, stack Stack, secretsProvider secrets.Provider) *deploy.Snapshot {
	origDisable := DisableIntegrityChecking
	DisableIntegrityChecking = true
	defer func() { DisableIntegrityChecking = origDisable }()

	snap, err := stack.Snapshot(ctx, secretsProvider)
	if err != nil {
		fqn := string(stack.Ref().FullyQualifiedName())
		logging.V(4).Infof("multistack: could not load snapshot for %s: %v", fqn, err)
		return nil
	}
	return snap
}

// buildMultistackDependencyGraph analyzes StackReference resources in previous snapshots
// to determine inter-stack dependencies. Only dependencies between co-deployed stacks are tracked.
func buildMultistackDependencyGraph(
	ctx context.Context,
	entries []MultistackEntry,
	entryMap map[string]*MultistackEntry,
) (map[string][]string, error) {
	deps := make(map[string][]string, len(entries))

	for _, entry := range entries {
		key := string(entry.Stack.Ref().FullyQualifiedName())
		deps[key] = nil // Initialize even if no deps

		// Get the stack's snapshot to find StackReference resources.
		snapshot := loadMultistackSnapshot(ctx, entry.Stack, entry.Op.SecretsProvider)
		if snapshot == nil {
			continue
		}

		// Find StackReference resources and extract the referenced stack names.
		for _, res := range snapshot.Resources {
			if isStackReferenceType(res.Type) {
				// The "name" input property contains the fully qualified stack reference.
				if nameVal, ok := res.Inputs["name"]; ok && nameVal.IsString() {
					refName := nameVal.StringValue()
					logging.V(4).Infof("multistack: stack %q has StackReference to %q", key, refName)
					// Only add dependency if the referenced stack is co-deployed.
					if _, coDeployed := entryMap[refName]; coDeployed {
						deps[key] = append(deps[key], refName)
						logging.V(4).Infof("multistack: added dependency %q -> %q", key, refName)
					} else {
						logging.V(4).Infof("multistack: %q not co-deployed (keys: %v)", refName, func() []string {
							keys := make([]string, 0, len(entryMap))
							for k := range entryMap {
								keys = append(keys, k)
							}
							return keys
						}())
					}
				}
			}
		}
	}

	logging.V(4).Infof("multistack: dependency graph: %v", deps)
	return deps, nil
}

// stackReferenceType is the type token for StackReference resources.
const stackReferenceType tokens.Type = "pulumi:pulumi:StackReference"

// isStackReferenceType checks if a resource type is a stack reference.
func isStackReferenceType(t tokens.Type) bool {
	return t == stackReferenceType
}

// topologicalSort performs a topological sort of stacks based on their dependencies.
// Returns a list of levels, where each level contains stacks that can run in parallel.
func topologicalSort(entries []MultistackEntry, deps map[string][]string) ([][]string, error) {
	// Build in-degree map.
	inDegree := make(map[string]int, len(entries))
	for _, entry := range entries {
		key := string(entry.Stack.Ref().FullyQualifiedName())
		if _, ok := inDegree[key]; !ok {
			inDegree[key] = 0
		}
	}

	for key, depList := range deps {
		_ = key
		for _, dep := range depList {
			inDegree[dep] = inDegree[dep] // ensure dep exists in map
		}
		// The "key" depends on items in depList, so the items in depList should come first.
		// inDegree counts how many stacks depend on this one being done first.
	}

	// Recalculate: inDegree[x] = number of deps x has (things x depends on).
	for k := range inDegree {
		inDegree[k] = 0
	}
	for key, depList := range deps {
		inDegree[key] = len(depList)
	}

	// Kahn's algorithm for topological sort with level tracking.
	var levels [][]string
	remaining := make(map[string]bool, len(entries))
	for _, entry := range entries {
		key := string(entry.Stack.Ref().FullyQualifiedName())
		remaining[key] = true
	}

	for len(remaining) > 0 {
		// Find all stacks with in-degree 0 (no remaining unresolved dependencies).
		var level []string
		for key := range remaining {
			if inDegree[key] == 0 {
				level = append(level, key)
			}
		}

		if len(level) == 0 {
			// Cycle detected — build error message.
			var cycleStacks []string
			for key := range remaining {
				cycleStacks = append(cycleStacks, key)
			}
			sort.Strings(cycleStacks)
			return nil, fmt.Errorf(
				"circular dependency detected among stacks: %s",
				strings.Join(cycleStacks, ", "),
			)
		}

		// Sort level for deterministic ordering.
		sort.Strings(level)
		levels = append(levels, level)

		// Remove stacks in this level and update in-degrees.
		for _, key := range level {
			delete(remaining, key)
			// For all stacks that depend on this one, decrement their in-degree.
			for other, depList := range deps {
				for _, dep := range depList {
					if dep == key {
						inDegree[other]--
					}
				}
			}
		}
	}

	return levels, nil
}

// shouldSkip checks if a stack should be skipped because one of its dependencies failed.
func shouldSkip(key string, deps map[string][]string, failed map[string]bool) bool {
	for _, dep := range deps[key] {
		if failed[dep] {
			return true
		}
	}
	return false
}

// buildStackLabels creates a mapping from project name to logical display label
// for multistack operations. The label is the directory basename (e.g., "vpc" for
// a project in the "vpc/" directory), matching what PrintMultistackConfirmation shows.
func buildStackLabels(entries []MultistackEntry) map[string]string {
	labels := make(map[string]string, len(entries))
	for _, entry := range entries {
		projectName := string(entry.Op.Proj.Name)
		dir := entry.Dir
		// Use relative path from cwd if possible, otherwise just basename.
		if cwd, err := os.Getwd(); err == nil {
			if rel, err := filepath.Rel(cwd, dir); err == nil {
				dir = rel
			}
		}
		labels[projectName] = filepath.Base(dir)
	}
	return labels
}

// buildExpectedStackURNs constructs the expected stack root URNs for each entry
// so the display can pre-populate stack rows immediately.
func buildExpectedStackURNs(entries []MultistackEntry) []resource.URN {
	urns := make([]resource.URN, 0, len(entries))
	for _, entry := range entries {
		stackName := entry.Stack.Ref().Name().Q()
		projName := entry.Op.Proj.Name
		urns = append(urns, resource.DefaultRootStackURN(stackName, projName))
	}
	return urns
}
