// Copyright 2026, Pulumi Corporation.

package lifecycletest

import (
	"context"
	"errors"
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAwaitingSuspendAndResume proves the non-terminal `awaiting` disposition end to end:
// a provider whose Create returns `awaiting` suspends the deployment -- its resource is
// left uncreated and its dependents are skipped, but resources created before it persist
// and the run reports an AwaitingError rather than failing. A later update, once the
// provider is ready, resumes and converges the whole graph.
func TestAwaitingSuspendAndResume(t *testing.T) {
	t.Parallel()

	gateReady := false
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}, deploytest.WithoutGrpc),
		deploytest.NewProviderLoader("pkgGate", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if !gateReady {
						return plugin.CreateResponse{
							Status:         resource.StatusOK,
							Awaiting:       true,
							AwaitingReason: "condition not yet met",
						}, nil
					}
					return plugin.CreateResponse{
						ID:         "gate-1",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		up, err := monitor.RegisterResource("pkgA:m:typA", "upstream", true, deploytest.ResourceOptions{
			SupportsResultReporting: true,
		})
		require.NoError(t, err)
		assert.Equal(t, pulumirpc.Result_SUCCESS, up.Result)

		gate, err := monitor.RegisterResource("pkgGate:m:typGate", "gate", true, deploytest.ResourceOptions{
			SupportsResultReporting: true,
			Dependencies:            []resource.URN{up.URN},
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "downstream", true, deploytest.ResourceOptions{
			SupportsResultReporting: true,
			Dependencies:            []resource.URN{gate.URN},
		})
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, SkipDisplayTests: true, HostF: hostF},
	}
	project := p.GetProject()

	upstreamURN := resource.URN("urn:pulumi:test::test::pkgA:m:typA::upstream")
	gateURN := resource.URN("urn:pulumi:test::test::pkgGate:m:typGate::gate")
	downstreamURN := resource.URN("urn:pulumi:test::test::pkgA:m:typA::downstream")

	// Run 1: the gate is not ready, so the deployment suspends.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	var awaitErr *deploy.AwaitingError
	require.True(t, errors.As(err, &awaitErr), "expected an AwaitingError, got %v", err)
	require.Len(t, awaitErr.Steps, 1)
	require.NotNil(t, snap)

	urns := snapshotURNs(snap)
	assert.Contains(t, urns, upstreamURN, "upstream should be persisted")
	assert.NotContains(t, urns, gateURN, "the awaiting gate must not be persisted")
	assert.NotContains(t, urns, downstreamURN, "the gate's dependent must be skipped")

	// Run 2: the gate is now ready, so the deployment resumes and converges everything.
	gateReady = true
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.NotNil(t, snap)

	urns = snapshotURNs(snap)
	assert.Contains(t, urns, upstreamURN)
	assert.Contains(t, urns, gateURN, "the gate should be created once ready")
	assert.Contains(t, urns, downstreamURN, "the dependent should converge after the gate")
}

func snapshotURNs(snap *deploy.Snapshot) []resource.URN {
	urns := make([]resource.URN, 0, len(snap.Resources))
	for _, r := range snap.Resources {
		urns = append(urns, r.URN)
	}
	return urns
}
