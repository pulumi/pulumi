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

package deploy

import (
	"context"
	"fmt"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// OutputWaiterStore manages stack outputs for co-deployed stacks, enabling
// dynamic StackReference resolution. When a stack references another co-deployed
// stack's outputs, it can block until those outputs become available.
type OutputWaiterStore struct {
	mu      sync.Mutex
	stacks  map[string]bool         // set of co-deployed stack names (fully qualified)
	outputs map[string]property.Map // stack name -> outputs (set when stack completes its root resource outputs)
	errors  map[string]error        // stack name -> error (set when stack fails)
	ready   map[string]chan struct{} // stack name -> channel closed when outputs are ready

	// For cycle detection: tracks which stacks are waiting on which.
	waitGraph map[string]string // waiter stack -> waited-on stack
}

// NewOutputWaiterStore creates a new store for the given set of co-deployed stack names.
func NewOutputWaiterStore(coDeployedStacks []string) *OutputWaiterStore {
	stacks := make(map[string]bool, len(coDeployedStacks))
	ready := make(map[string]chan struct{}, len(coDeployedStacks))
	for _, s := range coDeployedStacks {
		stacks[s] = true
		ready[s] = make(chan struct{})
	}
	return &OutputWaiterStore{
		stacks:    stacks,
		outputs:   make(map[string]property.Map),
		errors:    make(map[string]error),
		ready:     ready,
		waitGraph: make(map[string]string),
	}
}

// IsCoDeployed returns true if the given stack name is part of this co-deployment.
func (s *OutputWaiterStore) IsCoDeployed(stackName string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stacks[stackName]
}

// SetOutputs records the outputs for a co-deployed stack, unblocking any waiters.
// This should be called when the stack's root resource calls RegisterResourceOutputs.
func (s *OutputWaiterStore) SetOutputs(stackName string, outputs property.Map) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.outputs[stackName] = outputs
	if ch, ok := s.ready[stackName]; ok {
		close(ch) // Signal all waiters
	}
}

// FailStack records that a co-deployed stack has failed, unblocking any waiters with an error.
// This should be called when a stack's operation fails so that dependent stacks don't hang.
func (s *OutputWaiterStore) FailStack(stackName string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.errors[stackName] = err
	if ch, ok := s.ready[stackName]; ok {
		// Close idempotently — only close if not already closed (outputs not already set).
		select {
		case <-ch:
			// Already closed (outputs were set before failure).
		default:
			close(ch)
		}
	}
}

// WaitForOutputs blocks until the given co-deployed stack's outputs are available.
// Returns an error if the context is cancelled, a cycle is detected, or the target stack failed.
// The waiterStack parameter identifies who is waiting (for cycle detection).
func (s *OutputWaiterStore) WaitForOutputs(
	ctx context.Context, waiterStack, targetStack string,
) (property.Map, error) {
	s.mu.Lock()

	// Check if outputs are already available.
	if outputs, ok := s.outputs[targetStack]; ok {
		s.mu.Unlock()
		return outputs, nil
	}

	// Check if the target stack has already failed.
	if err, ok := s.errors[targetStack]; ok {
		s.mu.Unlock()
		return property.Map{}, fmt.Errorf("co-deployed stack %q failed: %w", targetStack, err)
	}

	// Check for cycles: would this create a cycle in the wait graph?
	if err := s.checkCycle(waiterStack, targetStack); err != nil {
		s.mu.Unlock()
		return property.Map{}, err
	}

	// Record the wait edge for cycle detection.
	s.waitGraph[waiterStack] = targetStack
	ch := s.ready[targetStack]
	s.mu.Unlock()

	// Block until outputs are ready or context is cancelled.
	select {
	case <-ch:
		s.mu.Lock()
		// Check for error first (FailStack closes the channel too).
		if err, ok := s.errors[targetStack]; ok {
			delete(s.waitGraph, waiterStack)
			s.mu.Unlock()
			return property.Map{}, fmt.Errorf("co-deployed stack %q failed: %w", targetStack, err)
		}
		outputs := s.outputs[targetStack]
		delete(s.waitGraph, waiterStack) // Clean up wait edge
		s.mu.Unlock()
		return outputs, nil
	case <-ctx.Done():
		s.mu.Lock()
		delete(s.waitGraph, waiterStack)
		s.mu.Unlock()
		return property.Map{}, fmt.Errorf(
			"timed out waiting for outputs from co-deployed stack %q: %w", targetStack, ctx.Err(),
		)
	}
}

// checkCycle checks if adding waiter->target edge would create a cycle.
// Must be called with s.mu held.
func (s *OutputWaiterStore) checkCycle(waiter, target string) error {
	// Walk the wait graph from target to see if we reach waiter.
	visited := make(map[string]bool)
	current := target
	for {
		if current == waiter {
			return fmt.Errorf(
				"circular dependency detected: stack %q and stack %q are waiting on each other's outputs",
				waiter, target,
			)
		}
		if visited[current] {
			break
		}
		visited[current] = true
		next, ok := s.waitGraph[current]
		if !ok {
			break
		}
		current = next
	}
	return nil
}
