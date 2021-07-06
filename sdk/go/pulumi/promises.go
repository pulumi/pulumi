// Copyright 2016-2021, Pulumi Corporation.
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

// Internals: helper promise types.
//
// Can call `fulfill()` multiple times, the first value wins.

package pulumi

import (
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

type providersPromise struct {
	once      sync.Once
	waitGroup sync.WaitGroup
	value     map[string]ProviderResource
}

type aliasesPromise struct {
	once      sync.Once
	waitGroup sync.WaitGroup
	value     []URNOutput
}

type transformationsPromise struct {
	promiseName string
	once        sync.Once
	waitGroup   sync.WaitGroup
	value       []ResourceTransformation
}

func initProvidersPromise(loc **providersPromise) *providersPromise {
	ptr := (*unsafe.Pointer)(unsafe.Pointer(loc))
	newP := &providersPromise{}
	newP.waitGroup.Add(1)
	atomic.CompareAndSwapPointer(ptr, nil, unsafe.Pointer(newP))
	return *loc
}

func initAliasesPromise(loc **aliasesPromise) *aliasesPromise {
	ptr := (*unsafe.Pointer)(unsafe.Pointer(loc))
	newP := &aliasesPromise{}
	newP.waitGroup.Add(1)
	atomic.CompareAndSwapPointer(ptr, nil, unsafe.Pointer(newP))
	return *loc
}

func initTransformationsPromise(promiseName string, loc **transformationsPromise) *transformationsPromise {
	dbgPrintf("initTransformationsPromise promiseName=%s stack=%s\n", promiseName,
		string(debug.Stack()))
	ptr := (*unsafe.Pointer)(unsafe.Pointer(loc))
	newP := &transformationsPromise{}
	newP.waitGroup.Add(1)
	atomic.CompareAndSwapPointer(ptr, nil, unsafe.Pointer(newP))
	p := *loc
	p.promiseName = promiseName
	return p
}

func (p *providersPromise) fulfill(value map[string]ProviderResource) {
	p.once.Do(func() {
		p.value = value
		p.waitGroup.Done()
	})
}

func (p *aliasesPromise) fulfill(value []URNOutput) {
	p.once.Do(func() {
		p.value = value
		p.waitGroup.Done()
	})
}

func (p *transformationsPromise) fulfill(value []ResourceTransformation) {
	dbgPrintf("Fulfill transformationPromise for %s with %d\n", p.promiseName, len(value))
	p.once.Do(func() {
		p.value = value
		p.waitGroup.Done()
	})
}

func (p *providersPromise) await() map[string]ProviderResource {
	withTimeout("providersPromise", 1*time.Second, func() {
		p.waitGroup.Wait()
	})
	return p.value
}

func (p *aliasesPromise) await() []URNOutput {
	withTimeout("aliasesPromise", 1*time.Second, func() {
		p.waitGroup.Wait()
	})
	return p.value
}

func (p *transformationsPromise) await(whyExactly string) []ResourceTransformation {
	dbgPrintf("await() transformationsPromise with name=%s and whyExactly=%s stack=%s\n", p.promiseName,
		whyExactly,
		string(debug.Stack()))
	withTimeout("transformationsPromise: "+p.promiseName, 3*time.Second, func() {
		p.waitGroup.Wait()
	})
	return p.value
}

func withTimeout(name string, dur time.Duration, work func()) {
	c := make(chan bool, 1)

	go func() {
		work()
		c <- false
	}()

	select {
	case <-c:
		return // ok
	case <-time.After(dur):
		dbgPrintf("Timeout after %v waiting on %s. Stack is %s", dur, name,
			string(debug.Stack()))
		panic(fmt.Sprintf("Timeout after %v waiting on %s. Stack is %s", dur, name,
			string(debug.Stack())))
	}
}
