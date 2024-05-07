package lifecycletest

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/google/uuid"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDuplicateURN tests that duplicate URNs are disallowed.
func TestDuplicateURN(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.Error(t, err)

		// Reads use the same URN namespace as register so make sure this also errors
		_, _, err = monitor.ReadResource("pkgA:m:typA", "resA", "id", "", resource.PropertyMap{}, "", "", "")
		assert.Error(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()
	_, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Error(t, err)
}

// TestDuplicateAlias tests that multiple new resources may not claim to be aliases for the same old resource.
func TestDuplicateAlias(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := func(monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	}

	runtimeF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return program(monitor)
	})
	hostF := deploytest.NewPluginHostF(nil, nil, runtimeF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
	}
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	project := p.GetProject()
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)

	program = func(monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			AliasURNs: []resource.URN{resURN},
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
			AliasURNs: []resource.URN{resURN},
		})
		assert.Error(t, err)
		return nil
	}

	_, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.Error(t, err)
}

func TestSecretMasked(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, inputs resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					// Return the secret value as an unmasked output. This should get masked by the engine.
					return "id", resource.PropertyMap{
						"shouldBeSecret": resource.NewStringProperty("bar"),
					}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"shouldBeSecret": resource.MakeSecret(resource.NewStringProperty("bar")),
			},
		})
		require.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		// Skip display tests because secrets are serialized with the blinding crypter and can't be restored
		Options: TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	project := p.GetProject()
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)

	assert.NotNil(t, snap)
	if snap != nil {
		assert.True(t, snap.Resources[1].Outputs["shouldBeSecret"].IsSecret())
	}
}

// TestReadReplaceStep creates a resource and then replaces it with a read resource.
func TestReadReplaceStep(t *testing.T) {
	t.Parallel()

	// Create resource.
	newTestBuilder(t, nil).
		WithProvider("pkgA", "1.0.0", &deploytest.Provider{
			CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64, preview bool,
			) (resource.ID, resource.PropertyMap, resource.Status, error) {
				return "created-id", news, resource.StatusOK, nil
			},
		}).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
			assert.NoError(t, err)
			return nil
		}, true).
		Then(func(snap *deploy.Snapshot, err error) {
			assert.NoError(t, err)
			assert.NotNil(t, snap)

			assert.Nil(t, snap.VerifyIntegrity())
			assert.Len(t, snap.Resources, 2)
			assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
			assert.False(t, snap.Resources[1].External)

			// ReadReplace resource.
			newTestBuilder(t, snap).
				WithProvider("pkgA", "1.0.0", &deploytest.Provider{
					ReadF: func(urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
					) (plugin.ReadResult, resource.Status, error) {
						return plugin.ReadResult{Outputs: resource.PropertyMap{}}, resource.StatusOK, nil
					},
				}).
				RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
					_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", "read-id", "", nil, "", "", "")
					assert.NoError(t, err)
					return nil
				}, false).
				Then(func(snap *deploy.Snapshot, err error) {
					assert.NoError(t, err)

					assert.NotNil(t, snap)
					assert.Nil(t, snap.VerifyIntegrity())
					assert.Len(t, snap.Resources, 2)
					assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
					assert.True(t, snap.Resources[1].External)
				})
		})
}

func TestRelinquishStep(t *testing.T) {
	t.Parallel()

	const resourceID = "my-resource-id"
	newTestBuilder(t, nil).
		WithProvider("pkgA", "1.0.0", &deploytest.Provider{
			CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
				preview bool,
			) (resource.ID, resource.PropertyMap, resource.Status, error) {
				// Should match the ReadResource resource ID.
				return resourceID, news, resource.StatusOK, nil
			},
		}).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
			assert.NoError(t, err)
			return nil
		}, true).
		Then(func(snap *deploy.Snapshot, err error) {
			assert.NotNil(t, snap)
			assert.Nil(t, snap.VerifyIntegrity())
			assert.Len(t, snap.Resources, 2)
			assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
			assert.False(t, snap.Resources[1].External)

			newTestBuilder(t, snap).
				WithProvider("pkgA", "1.0.0", &deploytest.Provider{
					ReadF: func(urn resource.URN, id resource.ID,
						inputs, state resource.PropertyMap,
					) (plugin.ReadResult, resource.Status, error) {
						return plugin.ReadResult{
							Outputs: resource.PropertyMap{},
						}, resource.StatusOK, nil
					},
				}).
				RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
					_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", resourceID, "", nil, "", "", "")
					assert.NoError(t, err)
					return nil
				}, true).
				Then(func(snap *deploy.Snapshot, err error) {
					assert.NoError(t, err)

					assert.NotNil(t, snap)
					assert.Nil(t, snap.VerifyIntegrity())
					assert.Len(t, snap.Resources, 2)
					assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
					assert.True(t, snap.Resources[1].External)
				})
		})
}

func TestTakeOwnershipStep(t *testing.T) {
	t.Parallel()

	newTestBuilder(t, nil).
		WithProvider("pkgA", "1.0.0", &deploytest.Provider{
			ReadF: func(urn resource.URN, id resource.ID,
				inputs, state resource.PropertyMap,
			) (plugin.ReadResult, resource.Status, error) {
				return plugin.ReadResult{
					Outputs: resource.PropertyMap{},
				}, resource.StatusOK, nil
			},
		}).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", "my-resource-id", "", nil, "", "", "")
			assert.NoError(t, err)
			return nil
		}, false).
		Then(func(snap *deploy.Snapshot, err error) {
			assert.NoError(t, err)

			assert.NotNil(t, snap)
			assert.Nil(t, snap.VerifyIntegrity())
			assert.Len(t, snap.Resources, 2)
			assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
			assert.True(t, snap.Resources[1].External)

			// Create new resource for this snapshot.
			newTestBuilder(t, snap).
				WithProvider("pkgA", "1.0.0", &deploytest.Provider{
					CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
						preview bool,
					) (resource.ID, resource.PropertyMap, resource.Status, error) {
						// Should match the ReadF resource ID.
						return "my-resource-id", news, resource.StatusOK, nil
					},
				}).
				RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
					_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
					assert.NoError(t, err)
					return nil
				}, true).
				Then(func(snap *deploy.Snapshot, err error) {
					assert.NoError(t, err)

					assert.NotNil(t, snap)
					assert.Nil(t, snap.VerifyIntegrity())
					assert.Len(t, snap.Resources, 2)
					assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
					assert.False(t, snap.Resources[1].External)
				})
		})
}

func TestInitErrorsStep(t *testing.T) {
	t.Parallel()

	// Create new resource for this snapshot.
	newTestBuilder(t, &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:   "pulumi:providers:pkgA",
				URN:    "urn:pulumi:test::test::pulumi:providers:pkgA::default",
				Custom: true,
				Delete: false,
				ID:     "935b2216-aec5-4810-96fd-5f6eae57ac88",
			},
			{
				Type:     "pkgA:m:typA",
				URN:      "urn:pulumi:test::test::pkgA:m:typA::resA",
				Custom:   true,
				ID:       "my-resource-id",
				Provider: "urn:pulumi:test::test::pulumi:providers:pkgA::default::935b2216-aec5-4810-96fd-5f6eae57ac88",
				InitErrors: []string{
					`errors should yield an empty update to "continue" awaiting initialization.`,
				},
			},
		},
	}).
		WithProvider("pkgA", "1.0.0", &deploytest.Provider{
			CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
				preview bool,
			) (resource.ID, resource.PropertyMap, resource.Status, error) {
				return "my-resource-id", news, resource.StatusOK, nil
			},
		}).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
			assert.NoError(t, err)
			return nil
		}, false).
		Then(func(snap *deploy.Snapshot, err error) {
			assert.NoError(t, err)

			assert.NotNil(t, snap)
			assert.Nil(t, snap.VerifyIntegrity())
			assert.Len(t, snap.Resources, 2)
			assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
			assert.Empty(t, snap.Resources[1].InitErrors)
		})
}

func TestReadNilOutputs(t *testing.T) {
	t.Parallel()

	const resourceID = "my-resource-id"
	newTestBuilder(t, nil).
		WithProvider("pkgA", "1.0.0", &deploytest.Provider{
			ReadF: func(urn resource.URN, id resource.ID,
				inputs, state resource.PropertyMap,
			) (plugin.ReadResult, resource.Status, error) {
				return plugin.ReadResult{}, resource.StatusOK, nil
			},
		}).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", resourceID, "", nil, "", "", "")
			assert.ErrorContains(t, err, "resource 'my-resource-id' does not exist")

			return nil
		}, true).
		Then(func(snap *deploy.Snapshot, err error) {
			assert.ErrorContains(t, err,
				"BAIL: step executor errored: step application failed: resource 'my-resource-id' does not exist")
		})
}

//nolint:paralleltest // This cannot be run in parallel because it depends on a global variable
func TestConcurrentRegisterResource(t *testing.T) {
	testCore := func() int64 {
		load := atomic.Int64{}
		maxLoad := atomic.Int64{}

		loaders := []*deploytest.ProviderLoader{
			deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
				return &deploytest.Provider{
					CheckF: func(urn resource.URN, olds, news resource.PropertyMap, randomSeed []byte) (
						resource.PropertyMap, []plugin.CheckFailure, error,
					) {
						currLoad := load.Add(1)
						defer load.Add(-1)

						for currMax := maxLoad.Load(); currLoad > currMax; currMax = maxLoad.Load() {
							maxLoad.CompareAndSwap(currMax, currLoad)
						}

						// Sleeping in tests is a recipe for flakiness, but without a delay, there is no
						// opportunity for concurrency
						time.Sleep(time.Millisecond * 5)
						runtime.Gosched()

						return news, nil, nil
					},
				}, nil
			}),
		}

		const resourceCount = 10

		programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			// create resources concurrently
			wg := sync.WaitGroup{}
			for i := 0; i != resourceCount; i++ {
				index := i

				wg.Add(1)
				go func() {
					defer wg.Done()

					name := fmt.Sprintf("test-%c", index+'a')
					_, err := monitor.RegisterResource("pkgA:m:typA", name, true)
					assert.NoError(t, err)
				}()
			}

			wg.Wait()

			return nil
		})
		hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

		p := &TestPlan{
			Options: TestUpdateOptions{
				HostF:                      hostF,
				OnlyVerifyCompleteSnapshot: true,
			},
		}

		project := p.GetProject()
		snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
		assert.NoError(t, err)

		assert.NotNil(t, snap)
		if snap != nil {
			actualResUrns := make([]resource.URN, resourceCount)
			expectedResUrns := make([]resource.URN, resourceCount)
			for i, res := range snap.Resources[1 : 1+resourceCount] {
				actualResUrns[i] = res.URN
				expectedResUrns[i] = resource.URN(fmt.Sprintf("urn:pulumi:test::test::pkgA:m:typA::test-%c", 'a'+i))
			}

			assert.ElementsMatch(t, expectedResUrns, actualResUrns)
		}

		return maxLoad.Load()
	}

	//nolint:paralleltest // This cannot be run in parallel because it depends on a global variable
	t.Run("Serialized Step Generator", func(t *testing.T) {
		deploy.UseParallelStepGen = false

		maxLoad := testCore()
		assert.Equalf(t, int64(1), maxLoad,
			"Expecting the maximum concurrent load on prov.Check to be 1 when parallel step gen is not enabled")
	})

	//nolint:paralleltest // This cannot be run in parallel because it depends on a global variable
	t.Run("Parallel Step Generator", func(t *testing.T) {
		deploy.UseParallelStepGen = true
		defer func() {
			deploy.UseParallelStepGen = false
		}()

		maxLoad := testCore()
		assert.Greaterf(t, maxLoad, int64(1),
			"Expecting the maximum concurrent load on prov.Check to be greater than 1 when parallel step gen is enabled")
	})
}

func parallelStepGenTestHelper(t *testing.T, maxDepth int, maxSpan int) (int64, int64, int) {
	load := atomic.Int64{}
	maxLoad := atomic.Int64{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CheckF: func(urn resource.URN, olds, news resource.PropertyMap, randomSeed []byte) (
					resource.PropertyMap, []plugin.CheckFailure, error,
				) {
					currLoad := load.Add(1)
					defer load.Add(-1)

					for currMax := maxLoad.Load(); currLoad > currMax; currMax = maxLoad.Load() {
						maxLoad.CompareAndSwap(currMax, currLoad)
					}

					// Sleeping in tests is a recipe for flakiness, but without a delay, there is no
					// opportunity for concurrency
					time.Sleep(time.Millisecond * 5)
					runtime.Gosched()

					return news, nil, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		var roots []resource.URN
		var m sync.Mutex
		// create roots
		wg := sync.WaitGroup{}
		for span := 0; span != maxSpan; span++ {
			rootNumber := span

			wg.Add(1)
			go func() {
				name := fmt.Sprintf("root-%c", rootNumber+'a')
				rrResp, err := monitor.RegisterResource("pkgA:m:typA", name, true)
				assert.NoError(t, err)
				m.Lock()
				defer func() {
					m.Unlock()
					wg.Done()
				}()
				roots = append(roots, rrResp.URN)
			}()
		}

		wg.Wait()

		var addChildren func(dep resource.URN, depth int)

		addChildren = func(dep resource.URN, depth int) {
			defer wg.Done()

			for span := 0; span != maxSpan; span++ {
				name := fmt.Sprintf("%v-child-%c", dep.Name(), 'a'+span)
				rrResp, err := monitor.RegisterResource("pkgA:m:typeA", name, true,
					deploytest.ResourceOptions{
						Dependencies: []resource.URN{dep},
					})
				assert.NoErrorf(t, err, "Error creating %s at depth %d, span %d", name, depth, span)
				if depth < maxDepth {
					wg.Add(1)
					go addChildren(rrResp.URN, depth+1)
				}
			}
		}

		for _, root := range roots {
			wg.Add(1)
			go addChildren(root, 1)

		}
		wg.Wait()
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{
			HostF:                      hostF,
			OnlyVerifyCompleteSnapshot: true,
		},
	}

	project := p.GetProject()
	start := time.Now().UnixMilli()
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)
	duration := time.Now().UnixMilli() - start

	assert.NotNil(t, snap)
	if snap != nil {
		actualRootUrns := make([]resource.URN, maxSpan)
		expectedRootUrns := make([]resource.URN, maxSpan)
		for i, res := range snap.Resources[1 : 1+maxSpan] {
			actualRootUrns[i] = res.URN
			expectedRootUrns[i] = resource.URN(fmt.Sprintf("urn:pulumi:test::test::pkgA:m:typA::root-%c", 'a'+i))
		}

		assert.ElementsMatch(t, expectedRootUrns, actualRootUrns)
	}

	return maxLoad.Load(), duration, len(snap.Resources)
}

//nolint:paralleltest // This cannot be run in parallel because it depends on a global variable
func TestParallelStepGenerator(t *testing.T) {
	t.Run("Fan out", func(t *testing.T) {
		const maxSpan = 3
		const maxDepth = 5

		//nolint:paralleltest // This cannot be run in parallel because it depends on a global variable
		t.Run("Serialized Step Generator", func(t *testing.T) {
			deploy.UseParallelStepGen = false

			maxLoad, duration, count := parallelStepGenTestHelper(t, maxDepth, maxSpan)
			assert.Equalf(t, int64(1), maxLoad,
				"Expecting the maximum concurrent load on prov.Check to be 1 when parallel step gen is not enabled")

			// Check that the duration per resource is at least 5 ms.
			resourceDuration := float64(duration) / float64(count)
			assert.GreaterOrEqualf(t, resourceDuration, 5.0,
				"Expecting the time per resource to be at least 5ms")
		})

		//nolint:paralleltest // This cannot be run in parallel because it depends on a global variable
		t.Run("Parallel Step Generator", func(t *testing.T) {
			deploy.UseParallelStepGen = true
			defer func() {
				deploy.UseParallelStepGen = false
			}()

			maxLoad, duration, count := parallelStepGenTestHelper(t, maxDepth, maxSpan)
			assert.Greaterf(t, maxLoad, int64(1),
				"Expecting the maximum concurrent load on prov.Check to be greater than 1 when parallel step gen is enabled")

			resourceDuration := float64(duration) / float64(count)
			// The absolute lower bound for resource duration ought to be the per resource cost / maxLoad
			assert.GreaterOrEqualf(t, resourceDuration, 5.0/float64(maxLoad),
				"Expecting the time per resource to be at least 5ms / %d (maxLoad)", maxLoad)
			// Hopefully a safe upper bound is to assume that the rsource duration is less than the absolute per resource cost
			assert.Lessf(t, resourceDuration, 5.0, "Expecting the time per resource to be at less than 5ms")
		})
	})

	//nolint:paralleltest // This cannot be run in parallel because it depends on a global variable
	t.Run("Narrow deep", func(t *testing.T) {
		const maxSpan = 1
		const maxDepth = 10

		//nolint:paralleltest // This cannot be run in parallel because it depends on a global variable
		t.Run("Serialized Step Generator", func(t *testing.T) {
			deploy.UseParallelStepGen = false

			maxLoad, duration, count := parallelStepGenTestHelper(t, maxDepth, maxSpan)
			assert.Equalf(t, int64(1), maxLoad,
				"Expecting the maximum concurrent load on prov.Check to be 1 when parallel step gen is not enabled")

			// Check that the duration per resource is at least 5 ms.
			resourceDuration := float64(duration) / float64(count)
			assert.GreaterOrEqualf(t, resourceDuration, 5.0,
				"Expecting the time per resource to be at least 5ms")
		})

		//nolint:paralleltest // This cannot be run in parallel because it depends on a global variable
		t.Run("Parallel Step Generator", func(t *testing.T) {
			deploy.UseParallelStepGen = true
			defer func() {
				deploy.UseParallelStepGen = false
			}()

			maxLoad, duration, count := parallelStepGenTestHelper(t, maxDepth, maxSpan)
			assert.Equalf(t, maxLoad, int64(1),
				"Expecting the maximum concurrent load on prov.Check to be 1, because there is no "+
					"concurrency in the RegisterResource calls")

			resourceDuration := float64(duration) / float64(count)
			assert.GreaterOrEqualf(t, resourceDuration, 5.0,
				"Expecting the time per resource to be at least 5ms")
		})
	})

	t.Run("Wide shallow", func(t *testing.T) {
		const maxSpan = 20
		const maxDepth = 1

		//nolint:paralleltest // This cannot be run in parallel because it depends on a global variable
		t.Run("Serialized Step Generator", func(t *testing.T) {
			deploy.UseParallelStepGen = false

			maxLoad, duration, count := parallelStepGenTestHelper(t, maxDepth, maxSpan)
			assert.Equalf(t, int64(1), maxLoad,
				"Expecting the maximum concurrent load on prov.Check to be 1 when parallel step gen is not enabled")

			// Check that the duration per resource is at least 5 ms.
			resourceDuration := float64(duration) / float64(count)
			assert.GreaterOrEqualf(t, resourceDuration, 5.0,
				"Expecting the time per resource to be at least 5ms")
		})

		//nolint:paralleltest // This cannot be run in parallel because it depends on a global variable
		t.Run("Parallel Step Generator", func(t *testing.T) {
			deploy.UseParallelStepGen = true
			defer func() {
				deploy.UseParallelStepGen = false
			}()

			maxLoad, duration, count := parallelStepGenTestHelper(t, maxDepth, maxSpan)
			assert.Greaterf(t, maxLoad, int64(1),
				"Expecting the maximum concurrent load on prov.Check to be greater than 1 when parallel step gen is enabled")

			resourceDuration := float64(duration) / float64(count)
			// The absolute lower bound for resource duration ought to be the per resource cost / maxLoad
			assert.GreaterOrEqualf(t, resourceDuration, 5.0/float64(maxLoad),
				"Expecting the time per resource to be at least 5ms / %d (maxLoad)", maxLoad)
			// Hopefully a safe upper bound for resource duration is less than the per resource absolute duration
			assert.Lessf(t, resourceDuration, 5.0,
				"Expecting the time per resource to be at less than 5ms")
		})
	})
}

func TestDeleteBeforeCreate(t *testing.T) {
	t.Parallel()

	makeProvider := func() *deploytest.Provider {
		return &deploytest.Provider{
			CheckF: func(urn resource.URN, olds, news resource.PropertyMap, randomSeed []byte) (
				resource.PropertyMap, []plugin.CheckFailure, error,
			) {
				return news, nil, nil
			},
			DiffF: func(urn resource.URN, id resource.ID, oldInputs, oldOutputs,
				newInputs resource.PropertyMap, ignoreChanges []string) (
				plugin.DiffResult, error,
			) {
				diff := oldInputs.Diff(newInputs)
				if diff == nil {
					return plugin.DiffResult{Changes: plugin.DiffNone}, nil
				}

				if diff.Updated("state") {
					newNoDbr, ok := newInputs["noDBR"]
					if !ok {
						newNoDbr = resource.NewPropertyValue(false)
					}

					return plugin.DiffResult{
						Changes:             plugin.DiffSome,
						ReplaceKeys:         []resource.PropertyKey{"state"},
						DeleteBeforeReplace: !newNoDbr.BoolValue(),
					}, nil
				}

				if diff.Updated("noReplace") {
					return plugin.DiffResult{Changes: plugin.DiffSome}, nil
				}

				return plugin.DiffResult{Changes: plugin.DiffNone}, nil
			},
			CreateF: func(urn resource.URN, inputs resource.PropertyMap, timeout float64, preview bool) (
				resource.ID, resource.PropertyMap, resource.Status, error,
			) {
				return resource.ID(uuid.NewString()), inputs, resource.StatusOK, nil
			},
		}
	}

	step1Result := newTestBuilder(t, nil).
		WithProvider("pkgA", "1.0.0", makeProvider()).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			aState := 42
			a, err := monitor.RegisterResource("pkgA:m:typA", "base", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"uniqueKey": 1,
							"state":     aState,
						}),
				})
			assert.NoErrorf(t, err, "Step1: RegisterResource base")

			b, err := monitor.RegisterResource("pkgA:m:typA", "dependent", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"state": aState,
						}),
					PropertyDeps: map[resource.PropertyKey][]urn.URN{
						"state": {a.URN},
					},
				})
			assert.NoError(t, err, "Step1: RegisterResource dependent")
			bState := aState

			cState := 99
			c, err := monitor.RegisterResource("pkgA:m:typA", "dependent-2", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"state": cState,
						}),
					Dependencies: []resource.URN{a.URN, b.URN},
				})
			assert.NoError(t, err, "Step1: RegisterResource dependent-2")

			_, err = monitor.RegisterResource("pkgA:m:typA", "dependent-3", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"state":     99,
							"noReplace": aState + bState,
						}),
					PropertyDeps: map[resource.PropertyKey][]urn.URN{
						"noReplace": {a.URN, b.URN},
					},
				})
			assert.NoError(t, err, "Step1: RegisterResource dependent-3")

			_, err = monitor.RegisterResource("pkgA:m:typA", "dependent-4", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"state": bState + cState,
						}),
					PropertyDeps: map[resource.PropertyKey][]urn.URN{
						"state": {b.URN, c.URN},
					},
				})
			assert.NoError(t, err, "Step1: RegisterResource dependent-4")

			return nil
		})

	assert.NoErrorf(t, step1Result.err, "Finished Step1")
	assert.NotNil(t, step1Result.snap)

	step1Snap := step1Result.snap

	assert.Nil(t, step1Snap.VerifyIntegrity())
	assert.Len(t, step1Snap.Resources, 1+5)

	step2Result := newTestBuilder(t, step1Snap).
		WithProvider("pkgA", "1.0.0", makeProvider()).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			aState := 99
			a, err := monitor.RegisterResource("pkgA:m:typA", "base", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"uniqueKey": 1,
							"state":     aState,
						}),
				})
			assert.NoErrorf(t, err, "Step2: RegisterResource base")

			b, err := monitor.RegisterResource("pkgA:m:typA", "dependent", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"state": aState,
						}),
					PropertyDeps: map[resource.PropertyKey][]urn.URN{
						"state": {a.URN},
					},
				})
			assert.NoError(t, err, "Step2: RegisterResource dependent")
			bState := aState

			cState := 99
			c, err := monitor.RegisterResource("pkgA:m:typA", "dependent-2", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"state": cState,
						}),
					Dependencies: []resource.URN{a.URN, b.URN},
				})
			assert.NoError(t, err, "Step2: RegisterResource dependent-2")

			_, err = monitor.RegisterResource("pkgA:m:typA", "dependent-3", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"state":     99,
							"noReplace": aState + bState,
						}),
					PropertyDeps: map[resource.PropertyKey][]urn.URN{
						"noReplace": {a.URN, b.URN},
					},
				})
			assert.NoError(t, err, "Step2: RegisterResource dependent-3")

			_, err = monitor.RegisterResource("pkgA:m:typA", "dependent-4", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"state": bState + cState,
						}),
					PropertyDeps: map[resource.PropertyKey][]urn.URN{
						"state": {b.URN, c.URN},
					},
				})
			assert.NoError(t, err, "Step2: RegisterResource dependent-4")
			return nil
		})

	assert.NoErrorf(t, step2Result.err, "Finished Step2")
	assert.NotNil(t, step2Result.snap)

	step2Snap := step2Result.snap
	assert.Nil(t, step2Snap.VerifyIntegrity())
	assert.Len(t, step2Snap.Resources, 1+5)

	step3Result := newTestBuilder(t, step2Snap).
		WithProvider("pkgA", "1.0.0", makeProvider()).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			aState := 100
			a, err := monitor.RegisterResource("pkgA:m:typA", "base", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"uniqueKey": 1,
							"state":     aState,
						}),
				})
			assert.NoErrorf(t, err, "Step3: RegisterResource base")

			_, err = monitor.RegisterResource("pkgA:m:typA", "dependent", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"state": aState,
						}),
					PropertyDeps: map[resource.PropertyKey][]urn.URN{
						"state": {a.URN},
					},
				})
			assert.NoError(t, err, "Step3: RegisterResource dependent")
			return nil
		})

	assert.NoErrorf(t, step3Result.err, "Finished Step3")
	assert.NotNil(t, step3Result.snap)

	step3Snap := step3Result.snap
	assert.Nil(t, step3Snap.VerifyIntegrity())
	assert.Len(t, step3Snap.Resources, 1+2)

	step4Result := newTestBuilder(t, step3Snap).
		WithProvider("pkgA", "1.0.0", makeProvider()).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			aState := 100
			a, err := monitor.RegisterResource("pkgA:m:typA", "base", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"uniqueKey": 1,
							"state":     aState,
						}),
				})
			assert.NoErrorf(t, err, "Step4: RegisterResource base")

			bState := 100
			b, err := monitor.RegisterResource("pkgA:m:typA", "base-2", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"uniqueKey": 2,
							"state":     bState,
						}),
				})
			assert.NoErrorf(t, err, "Step4: RegisterResource base-2")

			_, err = monitor.RegisterResource("pkgA:m:typA", "dependent", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"state": aState + bState,
						}),
					PropertyDeps: map[resource.PropertyKey][]urn.URN{
						"state": {a.URN, b.URN},
					},
				})
			assert.NoErrorf(t, err, "Step4: RegisterResource dependent")

			return nil
		})

	assert.NoErrorf(t, step4Result.err, "Finished Step4")
	assert.NotNil(t, step4Result.snap)

	step4Snap := step4Result.snap
	assert.Nil(t, step4Snap.VerifyIntegrity())
	assert.Len(t, step4Snap.Resources, 1+3)

	step5Result := newTestBuilder(t, step4Snap).
		WithProvider("pkgA", "1.0.0", makeProvider()).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			aState := 200
			a, err := monitor.RegisterResource("pkgA:m:typA", "base", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"uniqueKey": 1,
							"state":     aState,
						}),
				})
			assert.NoErrorf(t, err, "Step5: RegisterResource base")

			bState := 50
			b, err := monitor.RegisterResource("pkgA:m:typA", "base-2", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"uniqueKey": 2,
							"state":     bState,
						}),
				})
			assert.NoErrorf(t, err, "Step5: RegisterResource base-2")

			_, err = monitor.RegisterResource("pkgA:m:typA", "dependent", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"state": aState + bState,
						}),
					PropertyDeps: map[resource.PropertyKey][]urn.URN{
						"state": {a.URN, b.URN},
					},
				})
			assert.NoErrorf(t, err, "Step5: RegisterResource dependent")

			return nil
		})

	assert.NoErrorf(t, step5Result.err, "Finished Step5")
	assert.NotNil(t, step5Result.snap)

	step5Snap := step5Result.snap
	assert.Nil(t, step5Snap.VerifyIntegrity())
	assert.Len(t, step5Snap.Resources, 1+3)

	step6Result := newTestBuilder(t, step5Snap).
		WithProvider("pkgA", "1.0.0", makeProvider()).
		RunUpdate(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			aState := 42
			a, err := monitor.RegisterResource("pkgA:m:typA", "base", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"uniqueKey": 1,
							"state":     aState,
							"noDBR":     true,
						}),
				})
			assert.NoErrorf(t, err, "Step6: RegisterResource base")

			bState := 42
			b, err := monitor.RegisterResource("pkgA:m:typA", "base-2", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"uniqueKey": 2,
							"state":     bState,
							"noDBR":     true,
						}),
				})
			assert.NoErrorf(t, err, "Step6: RegisterResource base-2")

			_, err = monitor.RegisterResource("pkgA:m:typA", "dependent", true,
				deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(
						map[string]interface{}{
							"state": aState + bState,
						}),
					PropertyDeps: map[resource.PropertyKey][]urn.URN{
						"state": {a.URN, b.URN},
					},
				})
			assert.NoErrorf(t, err, "Step6: RegisterResource dependent")

			return nil
		})

	assert.NoErrorf(t, step6Result.err, "Finished Step6")
	assert.NotNil(t, step6Result.snap)

	step6Snap := step6Result.snap
	assert.Nil(t, step6Snap.VerifyIntegrity())
	assert.Len(t, step6Snap.Resources, 1+3)
}
