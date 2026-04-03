// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lifecycletest

import (
	"context"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	rstack "github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// TestOutputDependenciesRoundTrip verifies that when SaveOutputDependencies is set in the
// project options, Output values with dependency information are preserved through a state file
// round-trip (serialize → deserialize → serialize), and that the resulting deployment is tagged
// with the "outputDependencies" feature and written at schema version 4 so that older CLIs
// refuse to open it.
func TestOutputDependenciesRoundTrip(t *testing.T) {
	t.Parallel()

	var componentURN resource.URN
	var innerResURN resource.URN

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					// Register the component itself.
					resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{
						Parent: req.Parent,
					})
					require.NoError(t, err)
					componentURN = resp.URN

					// Register a child custom resource that the component owns.
					innerResp, err := monitor.RegisterResource("pkgA:m:typB", "inner", true, deploytest.ResourceOptions{
						Parent: resp.URN,
					})
					require.NoError(t, err)
					innerResURN = innerResp.URN

					// Return outputs that are Output values with dependency info pointing at
					// the inner resource.
					deps := []resource.URN{innerResURN}
					outputs := resource.PropertyMap{
						"knownOutput": resource.NewProperty(resource.Output{
							Element:      resource.NewProperty("hello"),
							Known:        true,
							Secret:       false,
							Dependencies: deps,
						}),
						"secretOutput": resource.NewProperty(resource.Output{
							Element:      resource.NewProperty("secret-value"),
							Known:        true,
							Secret:       true,
							Dependencies: deps,
						}),
						"unknownOutput": resource.NewProperty(resource.Output{
							Known:        false,
							Dependencies: deps,
						}),
					}

					err = monitor.RegisterResourceOutputs(resp.URN, outputs)
					require.NoError(t, err)

					return plugin.ConstructResponse{
						URN:     resp.URN,
						Outputs: outputs,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "comp", false, deploytest.ResourceOptions{
			Remote: true,
		})
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:                t,
			HostF:            hostF,
			SkipDisplayTests: true,
			UpdateOptions: engine.UpdateOptions{
				SaveOutputDependencies: true,
			},
		},
	}

	p.Steps = []lt.TestStep{
		{
			Op:          engine.Update,
			SkipPreview: true,
			Validate: func(
				project workspace.Project,
				target deploy.Target,
				entries engine.JournalEntries,
				_ []engine.Event,
				err error,
			) error {
				require.NoError(t, err)

				snap, snapErr := entries.Snap(target.Snapshot)
				require.NoError(t, snapErr)
				require.NotNil(t, snap)

				// Serialize the snapshot with the feature enabled.
				serCtx := rstack.WithSaveOutputDependencies(t.Context())
				deployment, version, features, serErr := rstack.SerializeDeploymentWithMetadata(
					serCtx, snap, false /*showSecrets*/)
				require.NoError(t, serErr)

				// The state file must be version 4 (feature-gated) because we have output
				// values with dependency info.
				assert.Equal(t, apitype.DeploymentSchemaVersionLatest, version,
					"expected schema version 4 when outputDependencies feature is active")
				assert.Contains(t, features, "outputDependencies",
					"expected outputDependencies feature in serialized deployment")

				// Find the component resource in the serialized deployment and verify
				// its output values have the OutputValueSig.
				var compRes *apitype.ResourceV3
				for i := range deployment.Resources {
					if deployment.Resources[i].URN == componentURN {
						compRes = &deployment.Resources[i]
						break
					}
				}
				require.NotNil(t, compRes, "component resource not found in deployment")

				checkOutputValueSig := func(key string) {
					v, ok := compRes.Outputs[key]
					require.True(t, ok, "output %q not found", key)
					m, ok := v.(map[string]any)
					require.True(t, ok, "output %q is not a map", key)
					assert.Equal(t, resource.OutputValueSig, m[resource.SigKey],
						"output %q missing OutputValueSig", key)
					deps, ok := m["dependencies"].([]any)
					require.True(t, ok, "output %q has no dependencies array", key)
					assert.Equal(t, []any{string(innerResURN)}, deps,
						"output %q has wrong dependencies", key)
				}
				checkOutputValueSig("knownOutput")
				checkOutputValueSig("secretOutput")
				checkOutputValueSig("unknownOutput")

				// Also verify that the values can be deserialized back to resource.Output
				// values with their dependency info intact.
				deserialized, deserErr := rstack.DeserializeDeploymentV3(
					t.Context(), *deployment, nil /*secretsProvider*/)
				require.NoError(t, deserErr)

				var compState *resource.State
				for _, res := range deserialized.Resources {
					if res.URN == componentURN {
						compState = res
						break
					}
				}
				require.NotNil(t, compState, "component resource not found after deserialization")

				checkRoundTrip := func(key string, wantKnown, wantSecret bool) {
					v, ok := compState.Outputs[resource.PropertyKey(key)]
					require.True(t, ok, "output %q not found after deserialization", key)
					require.True(t, v.IsOutput(), "output %q is not an Output after deserialization", key)
					o := v.OutputValue()
					assert.Equal(t, wantKnown, o.Known, "output %q Known mismatch", key)
					assert.Equal(t, wantSecret, o.Secret, "output %q Secret mismatch", key)
					require.Len(t, o.Dependencies, 1, "output %q Dependencies length mismatch", key)
					assert.Equal(t, innerResURN, o.Dependencies[0],
						"output %q dependency URN mismatch", key)
				}
				checkRoundTrip("knownOutput", true, false)
				checkRoundTrip("secretOutput", true, true)
				checkRoundTrip("unknownOutput", false, false)

				return nil
			},
		},
	}

	p.Run(t, nil)
}

// TestOutputDependenciesNotSavedByDefault verifies that without SaveOutputDependencies in the
// project options, Output values are degraded to their plain values (existing behaviour), the
// state remains at schema version 3, and old CLIs can continue to read it.
func TestOutputDependenciesNotSavedByDefault(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{
						Parent: req.Parent,
					})
					require.NoError(t, err)

					innerResp, err := monitor.RegisterResource("pkgA:m:typB", "inner", true, deploytest.ResourceOptions{
						Parent: resp.URN,
					})
					require.NoError(t, err)

					deps := []resource.URN{innerResp.URN}
					outputs := resource.PropertyMap{
						"out": resource.NewProperty(resource.Output{
							Element:      resource.NewProperty("value"),
							Known:        true,
							Dependencies: deps,
						}),
					}
					err = monitor.RegisterResourceOutputs(resp.URN, outputs)
					require.NoError(t, err)

					return plugin.ConstructResponse{URN: resp.URN, Outputs: outputs}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "comp", false, deploytest.ResourceOptions{
			Remote: true,
		})
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	p.Steps = []lt.TestStep{
		{
			Op:          engine.Update,
			SkipPreview: true,
			Validate: func(
				project workspace.Project,
				target deploy.Target,
				entries engine.JournalEntries,
				_ []engine.Event,
				err error,
			) error {
				require.NoError(t, err)

				snap, snapErr := entries.Snap(target.Snapshot)
				require.NoError(t, snapErr)
				require.NotNil(t, snap)

				_, version, features, serErr := rstack.SerializeDeploymentWithMetadata(
					t.Context(), snap, false /*showSecrets*/)
				require.NoError(t, serErr)

				// Without the option, we should NOT get version 4 or the outputDependencies feature.
				assert.Equal(t, apitype.DeploymentSchemaVersionCurrent, version,
					"expected schema version 3 when outputDependencies is disabled")
				assert.NotContains(t, features, "outputDependencies",
					"did not expect outputDependencies feature when option is unset")

				return nil
			},
		},
	}

	p.Run(t, nil)
}
