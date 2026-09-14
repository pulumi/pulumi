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
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAwaitingSuspendAndResume proves the non-terminal `awaiting` disposition end to end:
// a provider whose Create returns `awaiting` suspends the deployment -- its resource is
// left uncreated and its dependents are skipped, but resources created before it persist
// and the run reports an AwaitingError rather than failing. A later update, once the
// provider is ready, resumes and converges the whole graph. It runs over both the
// in-process provider interface and the full gRPC wire (proto + client/server mapping).
func TestAwaitingSuspendAndResume(t *testing.T) {
	t.Parallel()

	for _, transport := range []struct {
		name string
		grpc func(*deploytest.PluginLoader)
	}{
		{"in-process", deploytest.WithoutGrpc},
		{"grpc", deploytest.WithGrpc},
	} {
		transport := transport
		t.Run(transport.name, func(t *testing.T) {
			t.Parallel()

			gateReady := false
			downstreamChecks := 0
			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{CheckF: func(_ context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
						if req.URN.Name() == "downstream" {
							downstreamChecks++
						}
						return plugin.CheckResponse{Properties: req.News}, nil
					}}, nil
				}, transport.grpc),
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
				}, transport.grpc),
			}

			partialValues := true
			programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				up, err := monitor.RegisterResource("pkgA:m:typA", "upstream", true, deploytest.ResourceOptions{
					SupportsResultReporting: true,
				})
				require.NoError(t, err)
				assert.Equal(t, pulumirpc.Result_SUCCESS, up.Result)

				gate, err := monitor.RegisterResource("pkgGate:m:typGate", "gate", true, deploytest.ResourceOptions{
					SupportsResultReporting: true,
					SupportsPartialValues:   &partialValues,
					Inputs: resource.PropertyMap{
						"release": resource.NewStringProperty("release:input"),
					},
					Dependencies: []resource.URN{up.URN},
				})
				require.NoError(t, err)
				assert.Equal(t, pulumirpc.Result_SUCCESS, gate.Result)
				if !gateReady {
					assert.True(t, gate.Unknown)
					assert.True(t, gate.Outputs["release"].IsComputed())
				}

				downstream, err := monitor.RegisterResource("pkgA:m:typA", "downstream", true, deploytest.ResourceOptions{
					SupportsResultReporting: true,
					Dependencies:            []resource.URN{gate.URN},
				})
				require.NoError(t, err)
				assert.Equal(t, pulumirpc.Result_SUCCESS, downstream.Result)
				if !gateReady {
					assert.True(t, downstream.Unknown)
				}
				return nil
			})
			hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

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
			require.Len(t, snap.DeferredResources, 2)
			deferredURNs := []resource.URN{snap.DeferredResources[0].URN, snap.DeferredResources[1].URN}
			assert.ElementsMatch(t, []resource.URN{gateURN, downstreamURN}, deferredURNs)

			urns := snapshotURNs(snap)
			assert.Contains(t, urns, upstreamURN, "upstream should be persisted")
			assert.NotContains(t, urns, gateURN, "the awaiting gate must not be persisted")
			assert.NotContains(t, urns, downstreamURN, "the gate's dependent must be skipped")
			assert.Zero(t, downstreamChecks, "an awaiting dependent must be skipped before provider Check")

			// Run 2: the gate is now ready, so the deployment resumes and converges everything.
			gateReady = true
			snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
			require.NoError(t, err)
			require.NotNil(t, snap)
			assert.Empty(t, snap.DeferredResources)

			urns = snapshotURNs(snap)
			assert.Contains(t, urns, upstreamURN)
			assert.Contains(t, urns, gateURN, "the gate should be created once ready")
			assert.Contains(t, urns, downstreamURN, "the dependent should converge after the gate")
			assert.Equal(t, 1, downstreamChecks)
		})
	}
}

// TestAwaitingDoesNotMaskFailedDependency verifies that ordinary failures take
// precedence when a resource depends on both a failed and an awaiting resource.
func TestAwaitingDoesNotMaskFailedDependency(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgFail", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{CreateF: func(
				context.Context, plugin.CreateRequest,
			) (plugin.CreateResponse, error) {
				return plugin.CreateResponse{}, errors.New("intentional create failure")
			}}, nil
		}, deploytest.WithoutGrpc),
		deploytest.NewProviderLoader("pkgAwait", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{CreateF: func(
				context.Context, plugin.CreateRequest,
			) (plugin.CreateResponse, error) {
				return plugin.CreateResponse{Status: resource.StatusOK, Awaiting: true}, nil
			}}, nil
		}, deploytest.WithoutGrpc),
		deploytest.NewProviderLoader("pkgChild", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}, deploytest.WithoutGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		failed, err := monitor.RegisterResource("pkgFail:m:typ", "failed", true, deploytest.ResourceOptions{
			SupportsResultReporting: true,
		})
		require.NoError(t, err)
		require.Equal(t, pulumirpc.Result_FAIL, failed.Result)

		awaiting, err := monitor.RegisterResource("pkgAwait:m:typ", "awaiting", true, deploytest.ResourceOptions{
			SupportsResultReporting: true,
		})
		require.NoError(t, err)
		require.True(t, awaiting.Unknown)

		child, err := monitor.RegisterResource("pkgChild:m:typ", "child", true, deploytest.ResourceOptions{
			SupportsResultReporting: true,
			Dependencies:            []resource.URN{failed.URN, awaiting.URN},
		})
		require.NoError(t, err)
		require.Equal(t, pulumirpc.Result_SKIP, child.Result)
		return nil
	})

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{
		T:                t,
		SkipDisplayTests: true,
		UpdateOptions: UpdateOptions{
			ContinueOnError: true,
		},
		HostF: deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...),
	}}
	_, err := lt.TestOp(Update).Run(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.ErrorContains(t, err, "intentional create failure")
}

// TestAwaitingSkippedComponentOutputs proves that a component held behind an awaiting
// resource can still complete its registration. When a component's step is skipped because
// it depends on an awaiting resource, the program (or a remote provider's construct) still
// calls RegisterResourceOutputs on it -- that must be a no-op for a resource that was never
// persisted this run, not an error that turns an honest suspension into a failure. This is
// the delivery train's promote-behind-approval shape: Stage(production-base) depends on an
// unresolved gate, gets skipped, and registers its outputs on unwind.
func TestAwaitingSkippedComponentOutputs(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgGate", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						Status:         resource.StatusOK,
						Awaiting:       true,
						AwaitingReason: "gate held",
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		gate, err := monitor.RegisterResource("pkgGate:m:typGate", "gate", true, deploytest.ResourceOptions{
			SupportsResultReporting: true,
		})
		require.NoError(t, err)

		comp, err := monitor.RegisterResource("my:mod:Comp", "comp", false, deploytest.ResourceOptions{
			SupportsResultReporting: true,
			Dependencies:            []resource.URN{gate.URN},
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "child", true, deploytest.ResourceOptions{
			SupportsResultReporting: true,
			Parent:                  comp.URN,
			Dependencies:            []resource.URN{gate.URN},
		})
		require.NoError(t, err)

		// The component completes its registration on unwind, exactly as a language SDK or a
		// remote construct does. A skipped component's outputs are a no-op, never an error.
		err = monitor.RegisterResourceOutputs(comp.URN, resource.PropertyMap{
			"summary": resource.NewStringProperty("skipped this run"),
		})
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, SkipDisplayTests: true, HostF: hostF},
	}
	project := p.GetProject()

	// The run suspends (awaiting), and the suspension is the ONLY abnormality: the skipped
	// component's RegisterResourceOutputs must not surface as a deployment error.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	var awaitErr *deploy.AwaitingError
	require.True(t, errors.As(err, &awaitErr), "expected an AwaitingError, got %v", err)
	require.NotNil(t, snap)
	urns := snapshotURNs(snap)
	assert.NotContains(t, urns, resource.URN("urn:pulumi:test::test::pkgGate:m:typGate::gate"))
}

func TestAwaitingCreateReplacementPreservesOldUntilResume(t *testing.T) {
	t.Parallel()

	for _, transport := range []struct {
		name string
		grpc func(*deploytest.PluginLoader)
	}{
		{"in-process", deploytest.WithoutGrpc},
		{"grpc", deploytest.WithGrpc},
	} {
		transport := transport
		t.Run(transport.name, func(t *testing.T) {
			t.Parallel()
			ready := true
			deletes := 0
			creates := 0
			loader := deploytest.NewProviderLoader("pkgGate", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
				return &deploytest.Provider{
					CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
						creates++
						if !ready {
							return plugin.CreateResponse{Status: resource.StatusOK, Awaiting: true}, nil
						}
						return plugin.CreateResponse{ID: resource.ID("gate-id"), Properties: req.Properties, Status: resource.StatusOK}, nil
					},
					DeleteF: func(context.Context, plugin.DeleteRequest) (plugin.DeleteResponse, error) {
						deletes++
						return plugin.DeleteResponse{Status: resource.StatusOK}, nil
					},
				}, nil
			}, transport.grpc)
			programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				result, err := monitor.RegisterResource("pkgGate:m:typGate", "gate", true, deploytest.ResourceOptions{
					SupportsResultReporting: true,
				})
				require.NoError(t, err)
				assert.Equal(t, pulumirpc.Result_SUCCESS, result.Result)
				return nil
			})
			p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, SkipDisplayTests: true,
				HostF: deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loader)}}
			project := p.GetProject()
			gateURN := resource.URN("urn:pulumi:test::test::pkgGate:m:typGate::gate")

			snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
			require.NoError(t, err)
			require.NotNil(t, findResourceByURN(snap.Resources, gateURN))

			ready = false
			p.Options.ReplaceTargets = deploy.NewUrnTargetsFromUrns([]resource.URN{gateURN})
			suspended, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
			var awaitErr *deploy.AwaitingError
			require.True(t, errors.As(err, &awaitErr), "expected AwaitingError, got %v", err)
			old := findResourceByURN(suspended.Resources, gateURN)
			require.NotNil(t, old)
			assert.Equal(t, resource.ID("gate-id"), old.ID)
			assert.False(t, old.Delete)
			require.Len(t, suspended.DeferredResources, 1)
			assert.Zero(t, deletes)

			ready = true
			resumed, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, suspended), p.Options, false, p.BackendClient, nil, "2")
			require.NoError(t, err)
			require.NotNil(t, findResourceByURN(resumed.Resources, gateURN))
			assert.Empty(t, resumed.DeferredResources)
			assert.Equal(t, 3, creates)
			assert.Equal(t, 1, deletes)
		})
	}
}

func TestAwaitingProviderReplacementPreservesOldProviderUntilResume(t *testing.T) {
	t.Parallel()

	ready := true
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgGate", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}, deploytest.WithoutGrpc),
		deploytest.NewProviderLoader("pkgGate", semver.MustParse("2.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(context.Context, plugin.DiffConfigRequest) (plugin.DiffResult, error) {
					return plugin.DiffResult{Changes: plugin.DiffSome,
						ReplaceKeys: []resource.PropertyKey{"version"}}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if !ready {
						return plugin.CreateResponse{Status: resource.StatusOK, Awaiting: true}, nil
					}
					return plugin.CreateResponse{ID: resource.ID("resource-id"),
						Properties: req.Properties, Status: resource.StatusOK}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}
	version := "1.0.0"
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		provider, err := monitor.RegisterResource(providers.MakeProviderType("pkgGate"), "provider", true,
			deploytest.ResourceOptions{Version: version})
		require.NoError(t, err)
		providerID := provider.ID
		if providerID == "" {
			providerID = providers.UnknownID
		}
		providerRef, err := providers.NewReference(provider.URN, providerID)
		require.NoError(t, err)
		_, err = monitor.RegisterResource("pkgGate:m:typGate", "gate", true,
			deploytest.ResourceOptions{Provider: providerRef.String(), SupportsResultReporting: true})
		require.NoError(t, err)
		return nil
	})
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, SkipDisplayTests: true,
		HostF: deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)}}
	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NoError(t, snap.VerifyIntegrity())
	oldProvider := findResourceByURN(snap.Resources,
		resource.URN("urn:pulumi:test::test::pulumi:providers:pkgGate::provider"))
	require.NotNil(t, oldProvider)
	oldProviderRef, err := providers.NewReference(oldProvider.URN, oldProvider.ID)
	require.NoError(t, err)

	version = "2.0.0"
	ready = false
	suspended, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	var awaitErr *deploy.AwaitingError
	require.True(t, errors.As(err, &awaitErr), "expected AwaitingError, got %v", err)
	require.NoError(t, suspended.VerifyIntegrity())
	assert.NotNil(t, findResourceByURN(suspended.Resources, oldProvider.URN))
	require.Len(t, suspended.DeferredResources, 1)
	assert.Equal(t, oldProviderRef.String(),
		findResourceByURN(suspended.Resources,
			resource.URN("urn:pulumi:test::test::pkgGate:m:typGate::gate")).Provider)

	ready = true
	resumed, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, suspended), p.Options, false, p.BackendClient, nil, "2")
	require.NoError(t, err)
	require.NoError(t, resumed.VerifyIntegrity())
	assert.Empty(t, resumed.DeferredResources)
	assert.NotEqual(t, oldProviderRef.String(),
		findResourceByURN(resumed.Resources,
			resource.URN("urn:pulumi:test::test::pkgGate:m:typGate::gate")).Provider)
}

func snapshotURNs(snap *deploy.Snapshot) []resource.URN {
	urns := make([]resource.URN, 0, len(snap.Resources))
	for _, r := range snap.Resources {
		urns = append(urns, r.URN)
	}
	return urns
}
