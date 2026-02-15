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
	"sort"
	"strings"
	"sync"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	sdkDisplay "github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
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
}

// MultistackResult holds the results for a single stack in a multistack operation.
type MultistackResult struct {
	// Changes is the set of resource changes for this stack.
	Changes sdkDisplay.ResourceChanges
	// Plan is the deployment plan (for preview operations).
	Plan *deploy.Plan
	// Error is any error that occurred during the operation.
	Error error
}

// MultistackPreview runs a preview across multiple stacks in dependency order.
// Stacks are ordered by StackReference dependencies discovered from their previous snapshots.
// Independent stacks at the same level run in parallel.
func MultistackPreview(
	ctx context.Context,
	entries []MultistackEntry,
	opts MultistackOptions,
) (map[string]*MultistackResult, error) {
	return runMultistackOperation(ctx, entries, opts, operationPreview)
}

// MultistackUpdate runs an update across multiple stacks in dependency order.
// Stacks are ordered by StackReference dependencies discovered from their previous snapshots.
// Independent stacks at the same level run in parallel.
func MultistackUpdate(
	ctx context.Context,
	entries []MultistackEntry,
	opts MultistackOptions,
) (map[string]*MultistackResult, error) {
	return runMultistackOperation(ctx, entries, opts, operationUpdate)
}

// MultistackDestroyPreview runs a destroy preview across multiple stacks, showing what would
// be destroyed without actually destroying anything. Uses the same dependency ordering as destroy.
func MultistackDestroyPreview(
	ctx context.Context,
	entries []MultistackEntry,
	opts MultistackOptions,
) (map[string]*MultistackResult, error) {
	return runMultistackOperation(ctx, entries, opts, operationDestroyPreview)
}

// MultistackDestroy destroys multiple stacks in reverse dependency order.
// Dependencies are automatically reversed so downstream stacks are destroyed first.
func MultistackDestroy(
	ctx context.Context,
	entries []MultistackEntry,
	opts MultistackOptions,
) (map[string]*MultistackResult, error) {
	return runMultistackOperation(ctx, entries, opts, operationDestroy)
}

type operationType int

const (
	operationPreview operationType = iota
	operationUpdate
	operationDestroy
	operationDestroyPreview
)

// runMultistackOperation orchestrates a multistack operation by:
// 1. Building a dependency graph from StackReference resources in previous snapshots
// 2. Topologically sorting stacks
// 3. Running each level of the topological sort in parallel
// 4. Routing all events to a unified display
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

	// For destroy operations, reverse the order (destroy downstream first).
	if opType == operationDestroy || opType == operationDestroyPreview {
		for i, j := 0, len(levels)-1; i < j; i, j = i+1, j-1 {
			levels[i], levels[j] = levels[j], levels[i]
		}
	}

	// Lock all stacks upfront in deterministic order to avoid deadlocks.
	// NOTE: For the MVP, we rely on the backend's per-stack locking during
	// individual operations. True upfront locking will be added in a follow-up.

	// Create an OutputWaiterStore for co-deployed stack output resolution.
	// This allows StackReferences between co-deployed stacks to resolve lazily,
	// even when they run in parallel (e.g., new stacks with no prior snapshots).
	coDeployedNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		coDeployedNames = append(coDeployedNames, string(entry.Stack.Ref().FullyQualifiedName()))
	}
	outputWaiters := deploy.NewOutputWaiterStore(coDeployedNames)
	logging.V(4).Infof("multistack: created OutputWaiterStore with co-deployed stacks: %v", coDeployedNames)

	// Set the output waiter store on each entry's engine options so it flows
	// through to the builtinProvider in each deployment.
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
	// All stacks' events are forwarded here for a single tree display.
	unifiedEvents := make(chan engine.Event)
	displayDone := make(chan bool)

	displayOpts := opts.DisplayOpts
	// Suppress permalink in unified display — each stack has its own permalink in the cloud.
	displayOpts.SuppressPermalink = true

	go display.ShowEvents(
		strings.ToLower(label), action,
		tokens.StackName{} /* no single stack */, "" /* no single project */,
		"" /* permalink */, unifiedEvents, displayDone, displayOpts, isPreview)

	// Execute stacks level by level.
	results := make(map[string]*MultistackResult, len(entries))
	failed := make(map[string]bool) // tracks stacks that failed

	for levelIdx, level := range levels {
		logging.V(4).Infof("multistack: executing level %d with %d stacks", levelIdx, len(level))

		// Check if any dependency of stacks in this level has failed.
		var skippedInLevel []string
		var runnableInLevel []string
		for _, key := range level {
			if shouldSkip(key, deps, failed) {
				skippedInLevel = append(skippedInLevel, key)
			} else {
				runnableInLevel = append(runnableInLevel, key)
			}
		}

		// Mark skipped stacks.
		for _, key := range skippedInLevel {
			results[key] = &MultistackResult{
				Error: fmt.Errorf("skipped: dependency failed"),
			}
			failed[key] = true
		}

		// If FailFast and we've already had failures, skip remaining.
		if opts.FailFast && len(failed) > 0 {
			for _, key := range runnableInLevel {
				results[key] = &MultistackResult{
					Error: fmt.Errorf("skipped: fail-fast mode and a prior stack failed"),
				}
			}
			continue
		}

		// Run stacks in this level in parallel.
		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, key := range runnableInLevel {
			wg.Add(1)
			go func(key string) {
				defer wg.Done()
				entry := entryMap[key]
				result := executeStackOperation(ctx, entry, opType, unifiedEvents)

				// If the stack failed, notify the output waiter store so
				// co-deployed stacks waiting on this stack's outputs unblock with an error.
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
	sendAggregatedSummary(results, unifiedEvents)

	close(unifiedEvents)
	<-displayDone

	return results, nil
}

// sendAggregatedSummary sends a single SummaryEvent to the unified display channel
// that aggregates changes across all stacks.
func sendAggregatedSummary(results map[string]*MultistackResult, events chan<- engine.Event) {
	changes := make(sdkDisplay.ResourceChanges)
	for _, result := range results {
		if result.Changes != nil {
			for op, count := range result.Changes {
				changes[op] += count
			}
		}
	}
	events <- engine.NewEvent(engine.SummaryEventPayload{
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

	// Create a per-stack event channel that forwards to the unified channel.
	events := make(chan engine.Event)
	var forwardWg sync.WaitGroup
	forwardWg.Add(1)
	go func() {
		defer forwardWg.Done()
		for e := range events {
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
		changes, err := UpdateStack(ctx, entry.Stack, entry.Op, events)
		result.Changes = changes
		result.Error = err
	case operationDestroy:
		// Skip preview & auto-approve — the multistack orchestrator handles prompting.
		entry.Op.Opts.SkipPreview = true
		entry.Op.Opts.AutoApprove = true
		changes, err := DestroyStack(ctx, entry.Stack, entry.Op, events)
		result.Changes = changes
		result.Error = err
	case operationDestroyPreview:
		// Run destroy in preview-only mode to show what will be destroyed.
		entry.Op.Opts.PreviewOnly = true
		changes, err := DestroyStack(ctx, entry.Stack, entry.Op, events)
		result.Changes = changes
		result.Error = err
	}

	close(events)
	forwardWg.Wait()

	return result
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
		snapshot, err := entry.Stack.Snapshot(ctx, entry.Op.SecretsProvider)
		if err != nil {
			logging.V(4).Infof("multistack: could not load snapshot for %s: %v", key, err)
			continue // No snapshot = new stack = no dependencies
		}
		if snapshot == nil {
			continue
		}

		// Find StackReference resources and extract the referenced stack names.
		for _, res := range snapshot.Resources {
			if isStackReferenceType(res.Type) {
				// The "name" input property contains the fully qualified stack reference.
				if nameVal, ok := res.Inputs["name"]; ok && nameVal.IsString() {
					refName := nameVal.StringValue()
					// Only add dependency if the referenced stack is co-deployed.
					if _, coDeployed := entryMap[refName]; coDeployed {
						deps[key] = append(deps[key], refName)
					}
				}
			}
		}
	}

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

// FormatMultistackSummary formats the multistack deployment plan for display.
func FormatMultistackSummary(entries []MultistackEntry) string {
	var sb strings.Builder
	sb.WriteString("Multistack deployment:\n")
	for _, entry := range entries {
		ref := entry.Stack.Ref().FullyQualifiedName()
		dir := entry.Dir
		if dir == "" {
			dir = "."
		}
		sb.WriteString(fmt.Sprintf("  %-20s → %s\n", dir, ref))
	}
	return sb.String()
}

// FormatMultistackResults formats the results of a multistack operation for display.
func FormatMultistackResults(
	results map[string]*MultistackResult,
	entries []MultistackEntry,
) string {
	var sb strings.Builder
	sb.WriteString("\nMultistack results:\n")

	totalChanges := make(sdkDisplay.ResourceChanges)

	for _, entry := range entries {
		key := string(entry.Stack.Ref().FullyQualifiedName())
		result := results[key]
		if result == nil {
			sb.WriteString(fmt.Sprintf("  %s: no result\n", key))
			continue
		}
		if result.Error != nil {
			sb.WriteString(fmt.Sprintf("  %s: FAILED - %v\n", key, result.Error))
			continue
		}
		// Summarize changes.
		var changeParts []string
		for op, count := range result.Changes {
			if count > 0 {
				changeParts = append(changeParts, fmt.Sprintf("%d %s", count, op))
			}
			totalChanges[op] += count
		}
		if len(changeParts) == 0 {
			sb.WriteString(fmt.Sprintf("  %s: no changes\n", key))
		} else {
			sort.Strings(changeParts)
			sb.WriteString(fmt.Sprintf("  %s: %s\n", key, strings.Join(changeParts, ", ")))
		}
	}

	sb.WriteString("\nSummary:\n")
	sb.WriteString(fmt.Sprintf("  %d stacks", len(entries)))
	var totalParts []string
	for op, count := range totalChanges {
		if count > 0 {
			totalParts = append(totalParts, fmt.Sprintf("%d %s", count, op))
		}
	}
	if len(totalParts) > 0 {
		sort.Strings(totalParts)
		sb.WriteString(fmt.Sprintf(", %s", strings.Join(totalParts, ", ")))
	}
	sb.WriteString("\n")

	return sb.String()
}
