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
	"math/rand"
	"runtime"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestResourceLock(t *testing.T) {
	t.Parallel()

	urns := []resource.URN{
		"urn:pulumi:zero",
		"urn:pulumi:one",
		"urn:pulumi:two",
		"urn:pulumi:three",
		"urn:pulumi:four",
		"urn:pulumi:five",
		"urn:pulumi:six",
		"urn:pulumi:seven",
		"urn:pulumi:eight",
		"urn:pulumi:nine",
	}
	t.Run("Two workers accessing the same urn", func(t *testing.T) {
		t.Parallel()

		resourceLock := newResourceLock()

		wg := &sync.WaitGroup{}
		wg.Add(2)

		urn := resource.URN("urn:pulumi:test")
		update := 0

		worker := func() {
			defer func() {
				resourceLock.lock()
				resourceLock.UnlockResource(urn)
				resourceLock.unlock()
				wg.Done()
			}()

			resourceLock.lock()
			resourceLock.LockResource(urn)
			resourceLock.unlock()

			update++
		}

		go worker()
		go worker()

		wg.Wait()

		assert.Equalf(t, 2, update, "Two updates should have happened")
	})

	t.Run("Attempt multiple concurrent actions", func(t *testing.T) {
		t.Parallel()
		resourceLock := newResourceLock()
		values := make(map[resource.URN]int)
		wg := &sync.WaitGroup{}

		// run a bunch of passes that access urns sequentially either
		// forwards or backwards
		for pass := 0; pass != 8; pass++ {
			wg.Add(1)
			go func(fwd bool) {
				for i := range urns {
					urn := urns[i]
					if !fwd {
						urn = urns[len(urns)-i-1]
					}
					resourceLock.lock()
					resourceLock.LockResource(urn)

					if prev, ok := values[urn]; ok {
						values[urn] = prev + 1
					} else {
						values[urn] = 1
					}
					resourceLock.unlock()
					runtime.Gosched()

					resourceLock.lock()
					resourceLock.UnlockResource(urn)
					resourceLock.unlock()
				}
				wg.Done()
			}(pass&1 == 0)
		}

		// run passes which access resources in pairs
		for pass := 0; pass != 8; pass++ {
			wg.Add(1)
			go func(fwd bool) {
				for i := 0; i+1 < len(urns); i += 2 {
					a := urns[i]
					b := urns[i+1]
					if !fwd {
						j := len(urns) - i - 2
						a = urns[j]
						b = urns[j+1]
					}
					resourceLock.lock()
					resourceLock.LockResources(func() []*resource.State {
						return []*resource.State{
							{URN: a},
							{URN: b},
						}
					})

					values[a]++
					values[b]++
					resourceLock.unlock()
					runtime.Gosched()

					resourceLock.lock()
					resourceLock.UnlockResources(
						[]*resource.State{
							{URN: a},
							{URN: b},
						})
					resourceLock.unlock()
				}
				wg.Done()
			}(pass&1 == 0)
		}

		// run passes which access half of the resources at a time
		for pass := 0; pass != 8; pass++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				half := len(urns) / 2
				for i := 0; i+half <= len(urns); i += half {
					resources := make([]*resource.State, half)

					for j := 0; j != half; j++ {
						resources[j] = &resource.State{
							URN: urns[i+j],
						}
					}

					resourceLock.lock()
					resourceLock.LockResources(func() []*resource.State {
						return resources
					})

					replacements := make([]dependentReplace, half)
					for j, res := range resources {
						values[res.URN]++
						replacements[j].res = res
					}
					resourceLock.unlock()
					runtime.Gosched()

					resourceLock.lock()
					resourceLock.UnlockDependentReplaces(replacements)
					resourceLock.unlock()
				}
			}()
		}
		wg.Wait()

		for _, urn := range urns {
			assert.Equalf(t, 24, values[urn], "expecting 24 for %v, got %v", urn, values[urn])
		}
	})

	t.Run("randomly call resourceLock methods checking for deadlock or panic", func(t *testing.T) {
		t.Parallel()

		rapid.Check(t, func(rt *rapid.T) {
			resourceLock := newResourceLock()

			rt.Run(map[string]func(*rapid.T){
				"LockResource": func(*rapid.T) {
					urn := urns[rand.Intn(len(urns))] //nolint:gosec
					resourceLock.lock()
					resourceLock.LockResource(urn)
					resourceLock.unlock()

					runtime.Gosched()

					resourceLock.lock()
					resourceLock.UnlockResource(urn)
					resourceLock.unlock()
				},
				"LockResourceWithInversion": func(*rapid.T) {
					urn := urns[rand.Intn(len(urns))] //nolint:gosec
					resourceLock.lock()
					resourceLock.LockResource(urn)

					assert.NoError(t, resourceLock.InvertLock(func() error {
						runtime.Gosched()
						return nil
					}))

					resourceLock.UnlockResource(urn)
					resourceLock.unlock()
				},
				"LockResources": func(*rapid.T) {
					resources := make([]*resource.State, len(urns)/3)
					for i := range resources {
					Retry:
						for {
							urn := urns[rand.Intn(len(urns))] //nolint:gosec
							for j := 0; j < i; j++ {
								if resources[j].URN == urn {
									continue Retry
								}
							}
							resources[i] = &resource.State{
								URN: urn,
							}
							break
						}
					}

					resourceLock.lock()
					resourceLock.LockResources(func() []*resource.State {
						return resources
					})
					resourceLock.unlock()

					runtime.Gosched()

					resourceLock.lock()
					resourceLock.UnlockResources(resources)
					resourceLock.unlock()
				},
				"UnlockDependentReplaces": func(*rapid.T) {
					resources := make([]*resource.State, len(urns)/3)
					for i := range resources {
					Retry:
						for {
							urn := urns[rand.Intn(len(urns))] //nolint:gosec
							for j := 0; j < i; j++ {
								if resources[j].URN == urn {
									continue Retry
								}
							}
							resources[i] = &resource.State{
								URN: urn,
							}
							break
						}
					}

					resourceLock.lock()
					resourceLock.LockResources(func() []*resource.State {
						return resources
					})
					resourceLock.unlock()

					runtime.Gosched()

					replaces := make([]dependentReplace, len(resources))
					for i, res := range resources {
						replaces[i].res = res
					}

					resourceLock.lock()
					resourceLock.UnlockDependentReplaces(replaces)
					resourceLock.unlock()
				},
			})
		})
	})
}
