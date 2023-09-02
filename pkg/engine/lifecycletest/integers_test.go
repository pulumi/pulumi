package lifecycletest

import (
	"context"
	"math/big"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
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
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := "created-id"
					if req.Preview {
						id = ""
					}

					i := req.Properties["int"].IntegerValue()
					state := resource.PropertyMap{
						"int": resource.NewIntegerProperty(i.Mul(i, big.NewInt(2))),
					}

					return plugin.CreateResponse{
						ID:         resource.ID(id),
						Properties: state,
						Status:     resource.StatusOK,
					}, nil
				},
			}
			return v, nil
		}, deploytest.WithGrpc),
	}

	opts := deploytest.ResourceOptions{
		Inputs: resource.PropertyMap{
			"int": resource.NewIntegerProperty(big.NewInt(1024)),
		},
	}
	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		stk, err := monitor.RegisterResource(
			resource.RootStackType,
			info.Project+"-"+info.Stack,
			false,
			deploytest.ResourceOptions{})
		assert.NoError(t, err)

		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, opts)
		assert.NoError(t, err)
		assert.Equal(t, resource.NewIntegerProperty(big.NewInt(2048)), resp.Outputs["int"])

		err = monitor.RegisterResourceOutputs(stk.URN, resource.PropertyMap{
			"roundtrip": resp.Outputs["int"],
			"bigint":    resource.NewIntegerProperty(big.NewInt(987654321987654321)),
		})
		assert.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)

	// Ensure that the snapshot contains the integer value.
	resA := snap.Resources[2]
	assert.Equal(t, resource.NewIntegerProperty(big.NewInt(1024)), resA.Inputs["int"])
	assert.Equal(t, resource.NewIntegerProperty(big.NewInt(2048)), resA.Outputs["int"])
	// Ensure the stack outputs contain the integer value.
	stack := snap.Resources[0]
	assert.Equal(t, resource.NewIntegerProperty(big.NewInt(2048)), stack.Outputs["roundtrip"])
	assert.Equal(t, resource.NewIntegerProperty(big.NewInt(987654321987654321)), stack.Outputs["bigint"])
}

// TestIntegers_DownlevelProvider tests that integers are properly marshaled as floats if the provider does
// not support integers.
func TestIntegers_DownlevelProvider(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			v := &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := "created-id"
					if req.Preview {
						id = ""
					}

					i := req.Properties["int"].NumberValue()
					state := resource.PropertyMap{
						"int": resource.NewNumberProperty(i * 2),
					}

					return plugin.CreateResponse{
						ID:         resource.ID(id),
						Properties: state,
						Status:     resource.StatusOK,
					}, nil
				},
			}
			return v, nil
		}, deploytest.WithGrpc, deploytest.RejectIntegers),
	}

	opts := deploytest.ResourceOptions{
		Inputs: resource.PropertyMap{
			"int": resource.NewIntegerProperty(big.NewInt(1024)),
		},
	}
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, opts)
		assert.NoError(t, err)
		assert.Equal(t, resource.NewNumberProperty(2048), resp.Outputs["int"])
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
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
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := "created-id"
					if req.Preview {
						id = ""
					}

					f := req.Properties["int"].NumberValue()
					state := resource.PropertyMap{
						"int": resource.NewIntegerProperty(big.NewInt(int64(f) * 2)),
					}

					return plugin.CreateResponse{
						ID:         resource.ID(id),
						Properties: state,
						Status:     resource.StatusOK,
					}, nil
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
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, opts)
		assert.NoError(t, err)
		assert.Equal(t, resource.NewNumberProperty(2048), resp.Outputs["int"])
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)

	// Ensure that the snapshot contains the integer value output, even though we had to send a float input.
	resA := snap.Resources[1]
	assert.Equal(t, resource.NewNumberProperty(1024), resA.Inputs["int"])
	assert.Equal(t, resource.NewIntegerProperty(big.NewInt(2048)), resA.Outputs["int"])
}

// TestIntegers_UplevelSDK tests that integers from an incorrect upstream SDK can be rejected by the engine. This tests
// that RejectIntegers works correctly on the SDK boundary which is used in conformance tests to test SDKs respect the
// engine's integer support flag.
func TestIntegers_UplevelSDK(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}, deploytest.WithGrpc),
	}

	opts := deploytest.ResourceOptions{
		Inputs: resource.PropertyMap{
			"int": resource.NewIntegerProperty(big.NewInt(1024)),
		},
	}
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		was := monitor.OverrideIntegerSupport(plugin.MarshalOptionKeep)
		assert.Equal(t, plugin.MarshalOptionReplace, was, "expected integer support to have been disabled by default")

		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, opts)
		assert.ErrorContains(t, err, "unexpected integer property value for \"int\"")
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF, UpdateOptions: UpdateOptions{DisableIntegers: true}},
	}

	project := p.GetProject()

	snap, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
	assert.NotNil(t, snap)
	assert.Empty(t, snap.Resources)
}
