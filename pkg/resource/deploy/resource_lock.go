// Copyright 2016-2024, Pulumi Corporation.
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
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type resourceLock struct {
	mu    sync.Mutex
	locks map[resource.URN]struct{}
	cond  sync.Cond
}

func newResourceLock() *resourceLock {
	rl := &resourceLock{
		locks: make(map[resource.URN]struct{}),
	}
	rl.cond.L = &rl.mu
	return rl
}

// Locks the controlling mutex
func (rl *resourceLock) lock() {
	rl.mu.Lock()
}

// Tries to lock the controlling mutex and reports when it succeeds.
func (rl *resourceLock) tryLock() bool {
	return rl.mu.TryLock()
}

// Unlocks the controlling mutex
func (rl *resourceLock) unlock() {
	rl.mu.Unlock()
}

// Locks a single resource, This waits until it can lock the URN
//
// The mutex MUST be locked when this is called
func (rl *resourceLock) LockResource(urn resource.URN) {
	contract.Assertf(!rl.mu.TryLock(), "the mutex must be locked")

	for _, ok := rl.locks[urn]; ok; _, ok = rl.locks[urn] {
		rl.cond.Wait()
	}
	rl.locks[urn] = struct{}{}
}

// Unlocks a single resource. This does not verify that the URN was actually
// locked.
//
// The mutex MUST be locked when this is called
func (rl *resourceLock) UnlockResource(urn resource.URN) {
	contract.Assertf(!rl.mu.TryLock(), "the mutex must be locked")

	delete(rl.locks, urn)
	// Note: We always use broadcast rather than signal, because we sometimes
	// lock URNS in groups and then unlock them singly, there is no way of knowing if
	// signal will wake the correct thread
	rl.cond.Broadcast()
}

// Locks a set of resources. This will
//
//  1. call the provided function to get a list of resources
//
//  2. IF any of the resources is currently locked
//
//     THEN this will wait and retry
//
//     ELSE this will lock each of the resources and return the list of resources locked
//
// Note: Each time this is unable to lock the list and retries, it will call the
// function to get the list of resources
//
// The mutex MUST be locked when this is called
func (rl *resourceLock) LockResources(fn func() []*resource.State) []*resource.State {
	contract.Assertf(!rl.mu.TryLock(), "the mutex must be locked")

Retry:
	for {
		resources := fn()

		for _, r := range resources {
			if _, ok := rl.locks[r.URN]; ok {
				rl.cond.Wait()
				continue Retry
			}
		}

		for _, r := range resources {
			rl.locks[r.URN] = struct{}{}
		}
		return resources
	}
}

// Unlocks a list of previously locked resources.
//
// The condition variable is signalled to all (Broadcast) when these are unlocked
// because we have a singular condition variable for many URNs so we cannot know
// which waiting goroutine will need to be awakened.
//
// The mutex MUST be locked when this is called
func (rl *resourceLock) UnlockResources(resources []*resource.State) {
	contract.Assertf(!rl.mu.TryLock(), "the mutex must be locked")

	for _, r := range resources {
		delete(rl.locks, r.URN)
	}
	rl.cond.Broadcast()
}

// Unlocks a list of previously locked resources provided as a list of
// dependentReplace structs
//
// For each resource that is unlocked, the condition variable is signalled. So
// if many threads are waiting on individual resources they have the potential
// to continue
//
// The mutex MUST be locked when this is called
func (rl *resourceLock) UnlockDependentReplaces(toReplace []dependentReplace) {
	contract.Assertf(!rl.mu.TryLock(), "the mutex must be locked")

	for _, r := range toReplace {
		delete(rl.locks, r.res.URN)
	}
	rl.cond.Broadcast()
}

// Inverts the mutex lock and runs the supplied function outside the lock
func (rl *resourceLock) InvertLock(fn func() error) error {
	contract.Assertf(!rl.mu.TryLock(), "the mutex must be locked")
	rl.mu.Unlock()
	defer rl.mu.Lock()

	return fn()
}
