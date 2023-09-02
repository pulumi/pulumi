package lifecycletest

import (
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"

	. "github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// TestIntegers tests that integers can be sent from SDK to engine to provider and back.
func TestIntegers(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			v := &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap,
					timeout float64, preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					id := "created-id"
					if preview {
						id = ""
					}

					i := news["int"].IntegerValue()
					state := resource.PropertyMap{
						"int": resource.NewIntegerProperty(i * 2),
					}

					return resource.ID(id), state, resource.StatusOK, nil
				},
			}
			return v, nil
		}, deploytest.WithGrpc),
	}

	opts := deploytest.ResourceOptions{
		Inputs: resource.PropertyMap{
			"int": resource.NewIntegerProperty(1024),
		},
	}
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, props, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, opts)
		assert.NoError(t, err)
		assert.Equal(t, resource.NewIntegerProperty(2048), props["int"])
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

	// Ensure that the snapshot contains the integer value.
	resA := snap.Resources[1]
	assert.Equal(t, resource.NewIntegerProperty(1024), resA.Inputs["int"])
	assert.Equal(t, resource.NewIntegerProperty(2048), resA.Outputs["int"])
}

// TestIntegers_DownlevelProvider tests that integers are properly marshaled as floats if the provider does
// not support integers.
func TestIntegers_DownlevelProvider(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			v := &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap,
					timeout float64, preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					id := "created-id"
					if preview {
						id = ""
					}

					i := news["int"].NumberValue()
					state := resource.PropertyMap{
						"int": resource.NewNumberProperty(i * 2),
					}

					return resource.ID(id), state, resource.StatusOK, nil
				},
			}
			return v, nil
		}, deploytest.WithGrpc, deploytest.RejectIntegers),
	}

	opts := deploytest.ResourceOptions{
		Inputs: resource.PropertyMap{
			"int": resource.NewIntegerProperty(1024),
		},
	}
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, props, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, opts)
		assert.NoError(t, err)
		assert.Equal(t, resource.NewNumberProperty(2048), props["int"])
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

	// Ensure that the snapshot contains the float values as the provider does not support integers.
	resA := snap.Resources[1]
	assert.Equal(t, resource.NewNumberProperty(1024), resA.Inputs["int"])
	assert.Equal(t, resource.NewNumberProperty(2048), resA.Outputs["int"])
}

// TestIntegers_DownlevelSDK tests that integers are properly marshaled as floats if the SDK does not support integers.
func TestIntegers_DownlevelSDK(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			v := &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap,
					timeout float64, preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					id := "created-id"
					if preview {
						id = ""
					}

					f := news["int"].NumberValue()
					state := resource.PropertyMap{
						"int": resource.NewIntegerProperty(int64(f) * 2),
					}

					return resource.ID(id), state, resource.StatusOK, nil
				},
			}
			return v, nil
		}, deploytest.WithGrpc),
	}

	opts := deploytest.ResourceOptions{
		Inputs: resource.PropertyMap{
			"int": resource.NewNumberProperty(1024),
		},
		DisableIntegers: true,
	}
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, props, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, opts)
		assert.NoError(t, err)
		assert.Equal(t, resource.NewNumberProperty(2048), props["int"])
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

	// Ensure that the snapshot contains the integer value output, even though we had to send a float input.
	resA := snap.Resources[1]
	assert.Equal(t, resource.NewNumberProperty(1024), resA.Inputs["int"])
	assert.Equal(t, resource.NewIntegerProperty(2048), resA.Outputs["int"])
}
