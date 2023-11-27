// Copyright 2016-2023, Pulumi Corporation.
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

package promise

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

const (
	statusUninitialized int32 = iota
	statusPending
	statusFulfilled
	statusRejected
)

// Promise is a promise that can be resolved with a value of type T or rejected with an error. It is safe to call Result
// on it multiple times from multiple goroutines. This is much more permissive than channels.
type Promise[T any] struct {
	done   chan struct{}
	mutex  sync.Mutex
	status atomic.Int32
	result T
	err    error
}

// Result waits for the promise to be resolved and returns the result.
func (p *Promise[T]) Result(ctx context.Context) (T, error) {
	if p.status.Load() == statusUninitialized {
		panic("Promise must be initialized")
	}

	// Wait for either the promise or context to be done, if the context is done just exit with it's error
	select {
	case <-p.done:
		break
	case <-ctx.Done():
		var t T
		return t, ctx.Err()
	}

	contract.Assertf(p.status.Load() != statusPending, "Promise must be resolved")
	// Only one of result or err will be set, the other will be the zero value so we can just return both.
	return p.result, p.err
}

// TryResult returns the result and true if the promise has been resolved, otherwise it returns false.
//
//nolint:revive // This error _isn't_ an error from the function, so ignore the "error should be last" rule.
func (p *Promise[T]) TryResult() (T, error, bool) {
	// We don't need to lock here because we're just reading the status and the result and err are immutable
	// once set.
	status := p.status.Load()

	if status == statusUninitialized {
		panic("Promise must be initialized")
	}

	if status == statusPending {
		var t T
		return t, nil, false
	}
	// If the status is not pending then the promise is resolved and we can return the result and err. There
	// is no race between status being set to fulfilled or rejected and result and err being changed.
	return p.result, p.err, true
}

// CompletionSource is a source for a promise that can be resolved or rejected. It is safe to call Resolve or
// Reject multiple times concurrently, the first will apply and all others will return that they couldn't set the
// promise.
type CompletionSource[T any] struct {
	init    sync.Once
	promise *Promise[T]
}

func (ps *CompletionSource[T]) Promise() *Promise[T] {
	ps.init.Do(func() {
		p := &Promise[T]{}
		p.status.Store(statusPending)
		p.done = make(chan struct{})
		ps.promise = p
	})
	return ps.promise
}

func (ps *CompletionSource[T]) Fulfill(value T) bool {
	promise := ps.Promise()
	promise.mutex.Lock()
	defer promise.mutex.Unlock()

	contract.Assertf(promise.status.Load() != statusUninitialized, "Promise must be initialized")
	if promise.status.Load() != statusPending {
		return false
	}
	promise.result = value
	promise.status.Store(statusFulfilled)
	close(promise.done)
	return true
}

func (ps *CompletionSource[T]) MustFulfill(value T) {
	if !ps.Fulfill(value) {
		panic("CompletionSource already resolved")
	}
}

func (ps *CompletionSource[T]) Reject(err error) bool {
	contract.Requiref(err != nil, "err", "err must not be nil")

	promise := ps.Promise()
	promise.mutex.Lock()
	defer promise.mutex.Unlock()

	contract.Assertf(promise.status.Load() != statusUninitialized, "Promise must be initialized")
	if promise.status.Load() != statusPending {
		return false
	}
	promise.err = err
	promise.status.Store(statusRejected)
	close(promise.done)
	return true
}

func (ps *CompletionSource[T]) MustReject(err error) {
	if !ps.Reject(err) {
		panic("CompletionSource already resolved")
	}
}

// Run runs the given function in a goroutine and returns a promise that will be resolved with the result of the
// function.
func Run[T any](f func() (T, error)) *Promise[T] {
	ps := &CompletionSource[T]{}
	go func() {
		value, err := f()
		if err != nil {
			ps.Reject(err)
		} else {
			ps.Fulfill(value)
		}
	}()
	return ps.Promise()
}
