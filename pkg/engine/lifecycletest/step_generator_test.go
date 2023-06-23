package lifecycletest

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
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

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		require.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.Error(t, err)

		// Reads use the same URN namespace as register so make sure this also errors
		_, _, err = monitor.ReadResource("pkgA:m:typA", "resA", "id", "", resource.PropertyMap{}, "", "")
		assert.Error(t, err)

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()
	_, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NotNil(t, res)
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
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	}

	runtime := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return program(monitor)
	})
	host := deploytest.NewPluginHost(nil, nil, runtime, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}
	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	program = func(monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			AliasURNs: []resource.URN{resURN},
		})
		require.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
			AliasURNs: []resource.URN{resURN},
		})
		assert.Error(t, err)
		return nil
	}

	_, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.NotNil(t, res)
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

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"shouldBeSecret": resource.MakeSecret(resource.NewStringProperty("bar")),
			},
		})
		require.NoError(t, err)

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	assert.NotNil(t, snap)
	if snap != nil {
		assert.True(t, snap.Resources[1].Outputs["shouldBeSecret"].IsSecret())
	}
}

func TestStepGeneratorParallel(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					time.Sleep(1 * time.Second)
					return plugin.DiffResult{}, nil
				},
			}, nil
		}),
	}

	var cond bool
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		wg := sync.WaitGroup{}
		// Create 5 resources res-N
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(i int) {
				_, _, _, err := monitor.RegisterResource(
					"pkgA:m:typA", fmt.Sprintf("res-%d", i), true, deploytest.ResourceOptions{
						Inputs: resource.PropertyMap{
							"foo": resource.NewBoolProperty(cond),
						},
					})
				require.NoError(t, err)
				wg.Done()
			}(i)
		}
		wg.Wait()
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	project := p.GetProject()
	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)

	// Cause diff.
	cond = true

	// Start timer to measure how long it takes to register all resources.
	start := time.Now()
	snap, res = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	end := time.Now()
	// Check that it took less than 2 seconds to register all resources.
	assert.True(
		t, end.Sub(start) < 2*time.Second, "5 diffs should have been run in parallel and taken less than 2 seconds")

	assert.Nil(t, res)

	if snap != nil {
		assert.True(t, len(snap.Resources) == 6 /* 5 resources + 1 stack */)
	}
}
