package lifecycletest

import (
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func TestTargetSkipsRegisterOutputsForNewComponent(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader(
			tokens.Package("pkgA"),
			semver.MustParse("1.0.0"),
			func() (plugin.Provider, error) {
				return &deploytest.Provider{}, nil
			},
		),
	}

	programF := deploytest.NewLanguageRuntimeF(
		func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
			require.NoError(t, err)

			// New ComponentResource that is NOT targeted
			_, err = monitor.RegisterResource(
				"pkgA:m:Component",
				"newComponent",
				false,
			)
			require.NoError(t, err)

			return nil
		},
	)

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{}
	project := p.GetProject()

	_, err := lt.TestOp(engine.Update).RunStep(
		project,
		p.GetTarget(t, nil),
		lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
			UpdateOptions: engine.UpdateOptions{
				Targets: deploy.NewUrnTargets([]string{
					"urn:pulumi:test::test::pkgA:m:Other::other",
				}),
			},
		},
		false,
		p.BackendClient,
		nil,
		"1",
	)

	// Passes if no panic occurs
	require.NoError(t, err)
}
