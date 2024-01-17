package deploy

import (
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type resourceLock struct {
	mu    *sync.Mutex
	locks map[resource.URN]struct{}
	cond  sync.Cond
}

func newResourceLock(mu *sync.Mutex) *resourceLock {
	rl := &resourceLock{
		mu:    mu,
		locks: make(map[resource.URN]struct{}),
	}
	rl.cond.L = mu
	return rl
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
	rl.cond.Signal()
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
// For each resource that is unlocked, the condition variable is signalled. So
// if many threads are waiting on individual resources they have the potential
// to continue
//
// The mutex MUST be locked when this is called
func (rl *resourceLock) UnlockResources(resources []*resource.State) {
	contract.Assertf(!rl.mu.TryLock(), "the mutex must be locked")

	for _, r := range resources {
		if _, ok := rl.locks[r.URN]; ok {
			delete(rl.locks, r.URN)
			rl.cond.Signal()
		}
	}
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
		if _, ok := rl.locks[r.res.URN]; ok {
			delete(rl.locks, r.res.URN)
			rl.cond.Signal()
		}
	}
}

// Inverts the mutex lock and runs the supplied function outside the lock
func (rl *resourceLock) InvertLock(fn func() error) error {
	contract.Assertf(!rl.mu.TryLock(), "the mutex must be locked")
	rl.mu.Unlock()
	defer rl.mu.Lock()

	return fn()
}
