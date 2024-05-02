package lifecycletest

import (
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterDefaultProvider(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				Package: "pkgA",
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "some-id", nil, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	var providerID resource.ID
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		resp, err = monitor.RegisterResource("pulumi:providers:pkgA", "pkgA", true, deploytest.ResourceOptions{
			Parent: resp.URN,
		},
		)
		require.NoError(t, err)
		providerID = resp.ID

		err = monitor.RegisterDefaultProvider(&pulumirpc.RegisterDefaultProviderRequest{
			Provider: string(resp.URN) + "::" + providerID.String(),
		})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "test", true)
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
		Steps: []TestStep{
			{Op: Update},
		},
	}

	snap := p.Run(t, nil)
	assert.NotNil(t, snap)
	assert.Equal(t, 3, len(snap.Resources)) // root stack + provider + created resource
	assert.Equal(t, urn.URN("urn:pulumi:test::test::pulumi:providers:pkgA::pkgA"), snap.Resources[1].URN)
	assert.Equal(t, "urn:pulumi:test::test::pulumi:providers:pkgA::pkgA::"+string(providerID), snap.Resources[2].Provider)
}

func TestImplicitDefaultProviderWithDifferntVersionDoesNotGetCreated(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				Package: "pkgA",
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "some-id", nil, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	var providerID resource.ID
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		resp, err = monitor.RegisterResource("pulumi:providers:pkgA", "pkgA", true, deploytest.ResourceOptions{
			Parent: resp.URN,
		},
		)
		require.NoError(t, err)
		providerID = resp.ID

		err = monitor.RegisterDefaultProvider(&pulumirpc.RegisterDefaultProviderRequest{
			Provider: string(resp.URN) + "::" + providerID.String(),
		})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "test", true, deploytest.ResourceOptions{
			Version: "1.0.0",
		},
		)
		require.ErrorContains(t, err, "bla")

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
		Steps: []TestStep{
			{
				Op:            Update,
				ExpectFailure: true, // We expect a failure here because we refuse to register a default provider for pkgA_1_0_0
			},
		},
	}

	p.Run(t, nil)
}
