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
		mu := sync.Mutex{}

		resourceLock := *newResourceLock(&mu)

		wg := &sync.WaitGroup{}
		wg.Add(2)

		urn := resource.URN("urn:pulumi:test")
		update := 0

		worker := func() {
			defer func() {
				mu.Lock()
				resourceLock.UnlockResource(urn)
				mu.Unlock()
				wg.Done()
			}()

			mu.Lock()
			resourceLock.LockResource(urn)
			mu.Unlock()

			update++
		}

		go worker()
		go worker()

		wg.Wait()

		assert.Equalf(t, 2, update, "Two updates should have happened")
	})

	t.Run("Attempt multiple concurrent actions", func(t *testing.T) {
		t.Parallel()
		mu := sync.Mutex{}
		resourceLock := *newResourceLock(&mu)
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
					mu.Lock()
					resourceLock.LockResource(urn)

					if prev, ok := values[urn]; ok {
						values[urn] = prev + 1
					} else {
						values[urn] = 1
					}
					mu.Unlock()
					runtime.Gosched()

					mu.Lock()
					resourceLock.UnlockResource(urn)
					mu.Unlock()
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
					mu.Lock()
					resourceLock.LockResources(func() []*resource.State {
						return []*resource.State{
							{URN: a},
							{URN: b},
						}
					})

					values[a] += 1
					values[b] += 1
					mu.Unlock()
					runtime.Gosched()

					mu.Lock()
					resourceLock.UnlockResources(
						[]*resource.State{
							{URN: a},
							{URN: b},
						})
					mu.Unlock()
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

					mu.Lock()
					resourceLock.LockResources(func() []*resource.State {
						return resources
					})

					replacements := make([]dependentReplace, half)
					for j, res := range resources {
						values[res.URN] += 1
						replacements[j].res = res
					}
					mu.Unlock()
					runtime.Gosched()

					mu.Lock()
					resourceLock.UnlockDependentReplaces(replacements)
					mu.Unlock()
				}
			}()
		}
		wg.Wait()

		for _, urn := range urns {
			assert.Equalf(t, 24, values[urn], "expecting 24 for %v, got %v", urn, values[urn])
		}
	})

	t.Run("randomly call resourceLock methods checking for deadlock or panic", func(t *testing.T) {
		rapid.Check(t, func(rt *rapid.T) {
			mu := sync.Mutex{}
			resourceLock := newResourceLock(&mu)

			rt.Run(map[string]func(*rapid.T){
				"LockResource": func(*rapid.T) {
					urn := urns[rand.Intn(len(urns))]
					mu.Lock()
					resourceLock.LockResource(urn)
					mu.Unlock()

					runtime.Gosched()

					mu.Lock()
					resourceLock.UnlockResource(urn)
					mu.Unlock()
				},
				"LockResources": func(*rapid.T) {
					resources := make([]*resource.State, len(urns)/3)
					for i := range resources {
					Retry:
						for {
							urn := urns[rand.Intn(len(urns))]
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

					mu.Lock()
					resourceLock.LockResources(func() []*resource.State {
						return resources
					})
					mu.Unlock()

					runtime.Gosched()

					mu.Lock()
					resourceLock.UnlockResources(resources)
					mu.Unlock()
				},
				"UnlockDependentReplaces": func(*rapid.T) {
					resources := make([]*resource.State, len(urns)/3)
					for i := range resources {
					Retry:
						for {
							urn := urns[rand.Intn(len(urns))]
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

					mu.Lock()
					resourceLock.LockResources(func() []*resource.State {
						return resources
					})
					mu.Unlock()

					runtime.Gosched()

					replaces := make([]dependentReplace, len(resources))
					for i, res := range resources {
						replaces[i].res = res
					}

					mu.Lock()
					resourceLock.UnlockDependentReplaces(replaces)
					mu.Unlock()
				},
			})
		})
	})
}
