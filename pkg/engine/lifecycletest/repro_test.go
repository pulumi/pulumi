package lifecycletest

import (
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

func TestRepro(t *testing.T) {
	t.Skip()

	p := &lt.TestPlan{
		Project: "test-project",
		Stack:   "test-stack",
	}
	project := p.GetProject()

	setupLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	setupProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		pkgAProv, err := monitor.RegisterResource("pulumi:providers:pkgA", "provA", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		pkgAProvRef, err := providers.NewReference(pkgAProv.URN, pkgAProv.ID)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:index:type-c58h", "foo", true, deploytest.ResourceOptions{
			Provider: pkgAProvRef.String(),
		})
		require.NoError(t, err)

		return nil
	})

	setupHostF := deploytest.NewPluginHostF(nil, nil, setupProgramF, setupLoaders...)
	setupOpts := lt.TestUpdateOptions{
		T:     t,
		HostF: setupHostF,
	}

	setupSnap, err := lt.TestOp(engine.Update).RunStep(project, p.GetTarget(t, nil), setupOpts, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	reproLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	reproProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		pkgBProv, err := monitor.RegisterResource("pulumi:providers:pkgB", "provB", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		pkgBProvRef, err := providers.NewReference(pkgBProv.URN, pkgBProv.ID)
		require.NoError(t, err)

		resX, err := monitor.RegisterResource("pkgB:index:type-c58h", "resX", false, deploytest.ResourceOptions{
			Provider: pkgBProvRef.String(),
		})
		require.NoError(t, err)

		pkgAProv, err := monitor.RegisterResource("pulumi:providers:pkgA", "provA", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		pkgAProvRef, err := providers.NewReference(pkgAProv.URN, pkgAProv.ID)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:index:type-c58h", "foo", true, deploytest.ResourceOptions{
			Provider: pkgAProvRef.String(),
			Dependencies: []resource.URN{
				resX.URN,
			},
		})
		require.NoError(t, err)

		return nil
	})

	reproHostF := deploytest.NewPluginHostF(nil, nil, reproProgramF, reproLoaders...)
	reproOpts := lt.TestUpdateOptions{
		T:     t,
		HostF: reproHostF,
		UpdateOptions: engine.UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test-stack::test-project::pkgA:index:type-c58h::foo",
			}),
		},
	}

	_, err = lt.TestOp(engine.Update).RunStep(project, p.GetTarget(t, setupSnap), reproOpts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}
