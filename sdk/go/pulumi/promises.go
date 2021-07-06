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
	once      sync.Once
	waitGroup sync.WaitGroup
	value     []ResourceTransformation
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

func initTransformationsPromise(loc **transformationsPromise) *transformationsPromise {
	ptr := (*unsafe.Pointer)(unsafe.Pointer(loc))
	newP := &transformationsPromise{}
	newP.waitGroup.Add(1)
	atomic.CompareAndSwapPointer(ptr, nil, unsafe.Pointer(newP))
	return *loc
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
	p.once.Do(func() {
		p.value = value
		p.waitGroup.Done()
	})
}

func (p *providersPromise) await() map[string]ProviderResource {
	withTimeout(1*time.Second, func() {
		p.waitGroup.Wait()
	})
	return p.value
}

func (p *aliasesPromise) await() []URNOutput {
	withTimeout(1*time.Second, func() {
		p.waitGroup.Wait()
	})
	return p.value
}

func (p *transformationsPromise) await() []ResourceTransformation {
	withTimeout(1*time.Second, func() {
		p.waitGroup.Wait()
	})
	return p.value
}

func withTimeout(dur time.Duration, work func()) {
	c := make(chan bool)
	go func() {
		time.Sleep(dur)
		c <- true
	}()

	go func() {
		work()
		c <- false
	}()

	timedout := <-c
	if timedout {
		panic("Timeout")
	}
}
