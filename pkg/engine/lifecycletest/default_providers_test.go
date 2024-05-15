package lifecycletest

import (
	"context"
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
	ctx := context.Background()

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

		newMonitor, err := monitor.CreateNewContext(ctx, &pulumirpc.CreateNewContextRequest{
			Providers: []string{string(resp.URN) + "::" + providerID.String()},
		})
		assert.NoError(t, err)

		_, err = newMonitor.RegisterResource("pkgA:m:typA", "test", true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "testnondefault", true)
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
		Steps: []TestStep{
			{Op: Update},
		},
	}

	snap := p.Run(t, nil)
	assert.NotNil(t, snap)
	assert.Equal(t, 5, len(snap.Resources)) // root stack + 2 providers + created resources
	assert.Equal(t, urn.URN("urn:pulumi:test::test::pulumi:providers:pkgA::pkgA"), snap.Resources[1].URN)
	assert.Equal(t, "urn:pulumi:test::test::pulumi:providers:pkgA::pkgA::"+string(providerID), snap.Resources[2].Provider)
	assert.Equal(t, urn.URN("urn:pulumi:test::test::pkgA:m:typA::testnondefault"), snap.Resources[4].URN)
	assert.Contains(t, snap.Resources[4].Provider, "providers:pkgA::default")
}
