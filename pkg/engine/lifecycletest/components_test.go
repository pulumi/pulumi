// Copyright 2024, Pulumi Corporation.
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
	"fmt"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests that a remote component (that is, one implemented by a provider's Construct method) can be created and its
// outputs correctly read.
func TestSingleComponentDefaultProviderLifecycle(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			construct := func(
				_ context.Context,
				req plugin.ConstructRequest,
				monitor *deploytest.ResourceMonitor,
			) (plugin.ConstructResponse, error) {
				resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{
					Parent:  req.Parent,
					Aliases: aliasesFromAliases(req.Options.Aliases),
					Protect: req.Options.Protect,
				})
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typB", "resA", true, deploytest.ResourceOptions{
					Parent: resp.URN,
				})
				assert.NoError(t, err)

				outs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
				err = monitor.RegisterResourceOutputs(resp.URN, outs)
				assert.NoError(t, err)

				return plugin.ConstructResponse{
					URN:     resp.URN,
					Outputs: outs,
				}, nil
			}

			return &deploytest.Provider{
				ConstructF: construct,
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", false, deploytest.ResourceOptions{
			Remote: true,
		})
		assert.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{
			"foo": resource.NewStringProperty("bar"),
		}, resp.Outputs)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		// Skip display tests because different ordering makes the colouring different.
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
		Steps:   lt.MakeBasicLifecycleSteps(t, 3),
	}
	p.Run(t, nil)
}

// Tests that two remote components implemented by provider Construct methods interact correctly when they have
// interdependencies specified only in the user program (that is, the Construct implementations themselves do not have
// explicit dependencies).
func TestComponentDeleteDependencies(t *testing.T) {
	t.Parallel()

	var (
		firstURN  resource.URN
		nestedURN resource.URN
		sgURN     resource.URN
		secondURN resource.URN
		ruleURN   resource.URN

		err error
	)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					switch string(req.Type) {
					case "pkgB:m:first":
						resp, err := monitor.RegisterResource("pkgB:m:first", req.Name, false)
						require.NoError(t, err)
						firstURN = resp.URN

						resp, err = monitor.RegisterResource("nested", "nested", false,
							deploytest.ResourceOptions{
								Parent: firstURN,
							})
						require.NoError(t, err)
						nestedURN = resp.URN

						resp, err = monitor.RegisterResource("pkgA:m:sg", "sg", true, deploytest.ResourceOptions{
							Parent: nestedURN,
						})
						require.NoError(t, err)
						sgURN = resp.URN

						err = monitor.RegisterResourceOutputs(nestedURN, resource.PropertyMap{})
						require.NoError(t, err)

						err = monitor.RegisterResourceOutputs(firstURN, resource.PropertyMap{})
						require.NoError(t, err)

						return plugin.ConstructResponse{URN: firstURN}, nil
					case "pkgB:m:second":
						resp, err := monitor.RegisterResource("pkgB:m:second", req.Name, false,
							deploytest.ResourceOptions{
								Dependencies: req.Options.Dependencies,
							})
						require.NoError(t, err)
						secondURN = resp.URN

						resp, err = monitor.RegisterResource("pkgA:m:rule", "rule", true,
							deploytest.ResourceOptions{
								Parent:       secondURN,
								Dependencies: req.Options.PropertyDependencies["sgID"],
							})
						require.NoError(t, err)
						ruleURN = resp.URN

						err = monitor.RegisterResourceOutputs(secondURN, resource.PropertyMap{})
						require.NoError(t, err)

						return plugin.ConstructResponse{URN: secondURN}, nil
					default:
						return plugin.ConstructResponse{}, fmt.Errorf("unexpected type %v", req.Type)
					}
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err = monitor.RegisterResource("pkgB:m:first", "first", false, deploytest.ResourceOptions{
			Remote: true,
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgB:m:second", "second", false, deploytest.ResourceOptions{
			Remote: true,
			PropertyDeps: map[resource.PropertyKey][]resource.URN{
				"sgID": {sgURN},
			},
			Dependencies: []resource.URN{firstURN},
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}

	p.Steps = []lt.TestStep{
		{
			Op:          engine.Update,
			SkipPreview: true,
		},
		{
			Op:          engine.Destroy,
			SkipPreview: true,
			Validate: func(project workspace.Project, target deploy.Target, entries engine.JournalEntries,
				evts []engine.Event, err error,
			) error {
				assert.NoError(t, err)

				firstIndex, nestedIndex, sgIndex, secondIndex, ruleIndex := -1, -1, -1, -1, -1

				for i, entry := range entries {
					switch urn := entry.Step.URN(); urn {
					case firstURN:
						firstIndex = i
					case nestedURN:
						nestedIndex = i
					case sgURN:
						sgIndex = i
					case secondURN:
						secondIndex = i
					case ruleURN:
						ruleIndex = i
					}
				}

				assert.Less(t, ruleIndex, sgIndex)
				assert.Less(t, ruleIndex, secondIndex)
				assert.Less(t, secondIndex, firstIndex)
				assert.Less(t, secondIndex, sgIndex)
				assert.Less(t, sgIndex, nestedIndex)
				assert.Less(t, nestedIndex, firstIndex)

				return err
			},
		},
	}
	p.Run(t, nil)
}

// Tests that the engine only sends OutputValues when invoking Construct (remote component resource) and Call (remote
// component method) methods on a provider.
func TestConstructCallSecretsUnknowns(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							Inputs:  req.Inputs,
							Outputs: req.State,
						},
						Status: resource.StatusOK,
					}, nil
				},
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					// Assert that "foo" is secret and "bar" is unknown
					foo := req.Inputs["foo"]
					assert.True(t, foo.IsOutput())
					assert.True(t, foo.OutputValue().Known)
					assert.True(t, foo.OutputValue().Secret)

					bar := req.Inputs["bar"]
					assert.True(t, bar.IsOutput())
					assert.False(t, bar.OutputValue().Known)
					assert.False(t, bar.OutputValue().Secret)

					resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{})
					assert.NoError(t, err)

					return plugin.ConstructResponse{
						URN: resp.URN,
					}, nil
				},
				CallF: func(
					_ context.Context,
					req plugin.CallRequest,
					_ *deploytest.ResourceMonitor,
				) (plugin.CallResponse, error) {
					// Assert that "foo" is secret and "bar" is unknown
					foo := req.Args["foo"]
					assert.True(t, foo.IsOutput())
					assert.True(t, foo.OutputValue().Known)
					assert.True(t, foo.OutputValue().Secret)

					bar := req.Args["bar"]
					assert.True(t, bar.IsOutput())
					assert.False(t, bar.OutputValue().Known)
					assert.False(t, bar.OutputValue().Secret)

					return plugin.CallResponse{}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		inputs := resource.PropertyMap{
			"foo": resource.MakeSecret(resource.NewStringProperty("foo")),
			"bar": resource.MakeComputed(resource.NewStringProperty("")),
		}

		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", false, deploytest.ResourceOptions{
			Remote: true,
			Inputs: inputs,
		})
		assert.NoError(t, err)

		_, _, _, err = monitor.Call("pkgA:m:typA", inputs, nil, "", "", "")
		assert.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()
	_, err := lt.TestOp(engine.Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)
}

// Tests that the engine propagates the dependencies of outputs received from Construct (remote component resource) and
// Call (remote component method) methods on a provider.
func TestConstructCallReturnDependencies(t *testing.T) {
	t.Parallel()

	test := func(t *testing.T, opt deploytest.PluginOption) {
		loaders := []*deploytest.ProviderLoader{
			deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
				return &deploytest.Provider{
					CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
						return plugin.CreateResponse{
							ID:         "created-id",
							Properties: req.Properties,
							Status:     resource.StatusOK,
						}, nil
					},
					ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
						return plugin.ReadResponse{
							ReadResult: plugin.ReadResult{
								Inputs:  req.Inputs,
								Outputs: req.State,
							},
							Status: resource.StatusOK,
						}, nil
					},
					ConstructF: func(
						_ context.Context,
						req plugin.ConstructRequest,
						monitor *deploytest.ResourceMonitor,
					) (plugin.ConstructResponse, error) {
						resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{})
						assert.NoError(t, err)

						respA, err := monitor.RegisterResource("pkgA:m:typA", req.Name+"-a", true, deploytest.ResourceOptions{
							Parent: resp.URN,
						})
						assert.NoError(t, err)

						// Return a secret and unknown output depending on some internal resource
						deps := []resource.URN{respA.URN}
						return plugin.ConstructResponse{
							URN: resp.URN,
							Outputs: resource.PropertyMap{
								"foo": resource.MakeSecret(resource.NewStringProperty("foo")),
								"bar": resource.MakeComputed(resource.NewStringProperty("")),
							},
							OutputDependencies: map[resource.PropertyKey][]resource.URN{
								"foo": deps,
								"bar": deps,
							},
						}, nil
					},
					CallF: func(
						_ context.Context,
						req plugin.CallRequest,
						_ *deploytest.ResourceMonitor,
					) (plugin.CallResponse, error) {
						// Arg was sent as an output but the dependency map should still be filled in for providers to look at
						assert.Equal(t,
							[]resource.URN{"urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resA-a"},
							req.Options.ArgDependencies["arg"])

						// Assume a single output arg that this call depends on
						arg := req.Args["arg"]
						deps := arg.OutputValue().Dependencies

						return plugin.CallResponse{
							Return: resource.PropertyMap{
								"foo": resource.MakeSecret(resource.NewStringProperty("foo")),
								"bar": resource.MakeComputed(resource.NewStringProperty("")),
							},
							ReturnDependencies: map[resource.PropertyKey][]resource.URN{
								"foo": deps,
								"bar": deps,
							},
						}, nil
					},
				}, nil
			}, opt),
		}

		programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", false, deploytest.ResourceOptions{
				Remote: true,
			})
			assert.NoError(t, err)

			// The urn of the internal resource the component created
			urn := resource.URN("urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resA-a")

			// Assert that the outputs are received as just plain values because SDKs don't yet support output
			// values returned from RegisterResource.
			assert.Equal(t, resource.PropertyMap{
				"foo": resource.MakeSecret(resource.NewStringProperty("foo")),
				"bar": resource.MakeComputed(resource.NewStringProperty("")),
			}, resp.Outputs)
			assert.Equal(t, map[resource.PropertyKey][]resource.URN{
				"foo": {urn},
				"bar": {urn},
			}, resp.Dependencies)

			result, deps, _, err := monitor.Call("pkgA:m:typA", resource.PropertyMap{
				// Send this as an output value using the dependencies returned.
				"arg": resource.NewOutputProperty(resource.Output{
					Element:      resp.Outputs["foo"].SecretValue().Element,
					Known:        true,
					Secret:       true,
					Dependencies: []resource.URN{urn},
				}),
			}, nil, "", "", "")
			assert.NoError(t, err)

			// Assert that the outputs are received as just plain values because SDKs don't yet support output
			// values returned from Call.
			assert.Equal(t, resource.PropertyMap{
				"foo": resource.MakeSecret(resource.NewStringProperty("foo")),
				"bar": resource.MakeComputed(resource.NewStringProperty("")),
			}, result)
			assert.Equal(t, map[resource.PropertyKey][]resource.URN{
				"foo": {urn},
				"bar": {urn},
			}, deps)

			return nil
		})
		hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

		p := &lt.TestPlan{
			Options: lt.TestUpdateOptions{T: t, HostF: hostF},
		}

		project := p.GetProject()
		_, err := lt.TestOp(engine.Update).Run(project, p.GetTarget(t, nil), p.Options, true, p.BackendClient, nil)
		assert.NoError(t, err)
	}

	t.Run("WithGrpc", func(t *testing.T) {
		t.Parallel()
		test(t, deploytest.WithGrpc)
	})
	t.Run("WithoutGrpc", func(t *testing.T) {
		t.Parallel()
		test(t, deploytest.WithoutGrpc)
	})
}

// Tests that the engine correctly receives OutputValues from Construct (remote component resource) and Call (remote
// component method) methods on a provider.
func TestConstructCallReturnOutputs(t *testing.T) {
	t.Parallel()

	test := func(t *testing.T, opt deploytest.PluginOption) {
		loaders := []*deploytest.ProviderLoader{
			deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
				return &deploytest.Provider{
					CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
						return plugin.CreateResponse{
							ID:         "created-id",
							Properties: req.Properties,
							Status:     resource.StatusOK,
						}, nil
					},
					ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
						return plugin.ReadResponse{
							ReadResult: plugin.ReadResult{
								Inputs:  req.Inputs,
								Outputs: req.State,
							},
							Status: resource.StatusOK,
						}, nil
					},
					ConstructF: func(
						_ context.Context,
						req plugin.ConstructRequest,
						monitor *deploytest.ResourceMonitor,
					) (plugin.ConstructResponse, error) {
						resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{})
						assert.NoError(t, err)

						respA, err := monitor.RegisterResource("pkgA:m:typA", req.Name+"-a", true, deploytest.ResourceOptions{
							Parent: resp.URN,
						})
						assert.NoError(t, err)

						// Return a secret and unknown output depending on some internal resource
						deps := []resource.URN{respA.URN}
						return plugin.ConstructResponse{
							URN: resp.URN,
							Outputs: resource.PropertyMap{
								"foo": resource.NewOutputProperty(resource.Output{
									Element:      resource.NewStringProperty("foo"),
									Known:        true,
									Secret:       true,
									Dependencies: deps,
								}),
								"bar": resource.NewOutputProperty(resource.Output{
									Dependencies: deps,
								}),
							},
							OutputDependencies: nil, // Left blank on purpose because AcceptsOutputs is true
						}, nil
					},
					CallF: func(
						_ context.Context,
						req plugin.CallRequest,
						_ *deploytest.ResourceMonitor,
					) (plugin.CallResponse, error) {
						// Arg was sent as an output but the dependency map should still be filled in for providers to look at
						assert.Equal(t,
							[]resource.URN{"urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resA-a"},
							req.Options.ArgDependencies["arg"])

						// Assume a single output arg that this call depends on
						arg := req.Args["arg"]
						deps := arg.OutputValue().Dependencies

						return plugin.CallResponse{
							Return: resource.PropertyMap{
								"foo": resource.NewOutputProperty(resource.Output{
									Element:      resource.NewStringProperty("foo"),
									Known:        true,
									Secret:       true,
									Dependencies: deps,
								}),
								"bar": resource.NewOutputProperty(resource.Output{
									Dependencies: deps,
								}),
							},
							ReturnDependencies: nil, // Left blank on purpose because AcceptsOutputs is true
						}, nil
					},
				}, nil
			}, opt),
		}

		programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", false, deploytest.ResourceOptions{
				Remote: true,
			})
			assert.NoError(t, err)

			// The urn of the internal resource the component created
			urn := resource.URN("urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resA-a")

			// Assert that the outputs are received as just plain values because SDKs don't yet support output
			// values returned from RegisterResource.
			assert.Equal(t, resource.PropertyMap{
				"foo": resource.MakeSecret(resource.NewStringProperty("foo")),
				"bar": resource.MakeComputed(resource.NewStringProperty("")),
			}, resp.Outputs)
			assert.Equal(t, map[resource.PropertyKey][]resource.URN{
				"foo": {urn},
				"bar": {urn},
			}, resp.Dependencies)

			result, deps, _, err := monitor.Call("pkgA:m:typA", resource.PropertyMap{
				// Send this as an output value using the dependencies returned.
				"arg": resource.NewOutputProperty(resource.Output{
					Element:      resp.Outputs["foo"].SecretValue().Element,
					Known:        true,
					Secret:       true,
					Dependencies: []resource.URN{urn},
				}),
			}, nil, "", "", "")
			assert.NoError(t, err)

			// Assert that the outputs are received as just plain values because SDKs don't yet support output
			// values returned from Call.
			assert.Equal(t, resource.PropertyMap{
				"foo": resource.MakeSecret(resource.NewStringProperty("foo")),
				"bar": resource.MakeComputed(resource.NewStringProperty("")),
			}, result)
			assert.Equal(t, map[resource.PropertyKey][]resource.URN{
				"foo": {urn},
				"bar": {urn},
			}, deps)

			return nil
		})
		hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

		p := &lt.TestPlan{
			Options: lt.TestUpdateOptions{T: t, HostF: hostF},
		}

		project := p.GetProject()
		_, err := lt.TestOp(engine.Update).Run(project, p.GetTarget(t, nil), p.Options, true, p.BackendClient, nil)
		assert.NoError(t, err)
	}
	t.Run("WithGrpc", func(t *testing.T) {
		t.Parallel()
		test(t, deploytest.WithGrpc)
	})
	t.Run("WithoutGrpc", func(t *testing.T) {
		t.Parallel()
		test(t, deploytest.WithoutGrpc)
	})
}

// Tests that the engine correctly sends explicit dependency information to Construct (remote component resource) and
// Call (remote component method) methods on a provider given just OutputValues.
func TestConstructCallSendDependencies(t *testing.T) {
	t.Parallel()

	test := func(t *testing.T, opt deploytest.PluginOption) {
		loaders := []*deploytest.ProviderLoader{
			deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
				return &deploytest.Provider{
					CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
						return plugin.CreateResponse{
							ID:         "created-id",
							Properties: req.Properties,
							Status:     resource.StatusOK,
						}, nil
					},
					ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
						return plugin.ReadResponse{
							ReadResult: plugin.ReadResult{
								Inputs:  req.Inputs,
								Outputs: req.State,
							},
							Status: resource.StatusOK,
						}, nil
					},
					ConstructF: func(
						_ context.Context,
						req plugin.ConstructRequest,
						monitor *deploytest.ResourceMonitor,
					) (plugin.ConstructResponse, error) {
						// Arg was sent as an output but the dependency map should still be filled in for providers to look at
						assert.Equal(t,
							[]resource.URN{"urn:pulumi:test::test::pkgA:m:typC::resC"},
							req.Options.PropertyDependencies["arg"])

						resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{})
						assert.NoError(t, err)

						respA, err := monitor.RegisterResource("pkgA:m:typA", req.Name+"-a", true, deploytest.ResourceOptions{
							Parent: resp.URN,
						})
						assert.NoError(t, err)

						// Return a secret and unknown output depending on some internal resource
						deps := []resource.URN{respA.URN}
						return plugin.ConstructResponse{
							URN: resp.URN,
							Outputs: resource.PropertyMap{
								"foo": resource.MakeSecret(resource.NewStringProperty("foo")),
								"bar": resource.MakeComputed(resource.NewStringProperty("")),
							},
							OutputDependencies: map[resource.PropertyKey][]resource.URN{
								"foo": deps,
								"bar": deps,
							},
						}, nil
					},
					CallF: func(
						_ context.Context,
						req plugin.CallRequest,
						_ *deploytest.ResourceMonitor,
					) (plugin.CallResponse, error) {
						// Arg was sent as an output but the dependency map should still be filled in for providers to look at
						assert.Equal(t,
							[]resource.URN{"urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resA-a"},
							req.Options.ArgDependencies["arg"])

						// Assume a single output arg that this call depends on
						arg := req.Args["arg"]
						deps := arg.OutputValue().Dependencies

						return plugin.CallResponse{
							Return: resource.PropertyMap{
								"foo": resource.MakeSecret(resource.NewStringProperty("foo")),
								"bar": resource.MakeComputed(resource.NewStringProperty("")),
							},
							ReturnDependencies: map[resource.PropertyKey][]resource.URN{
								"foo": deps,
								"bar": deps,
							},
						}, nil
					},
				}, nil
			}, opt),
		}

		programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			respC, err := monitor.RegisterResource("pkgA:m:typC", "resC", false, deploytest.ResourceOptions{
				Inputs: resource.PropertyMap{
					"arg": resource.NewNumberProperty(1),
				},
			})
			assert.NoError(t, err)

			resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", false, deploytest.ResourceOptions{
				Remote: true,
				Inputs: resource.PropertyMap{
					"arg": resource.NewOutputProperty(resource.Output{
						Element:      respC.Outputs["arg"],
						Known:        true,
						Dependencies: []resource.URN{respC.URN},
					}),
				},
			})
			assert.NoError(t, err)

			// The urn of the internal resource the component created
			urn := resource.URN("urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resA-a")

			// Assert that the outputs are received as just plain values because SDKs don't yet support output
			// values returned from RegisterResource.
			assert.Equal(t, resource.PropertyMap{
				"foo": resource.MakeSecret(resource.NewStringProperty("foo")),
				"bar": resource.MakeComputed(resource.NewStringProperty("")),
			}, resp.Outputs)
			assert.Equal(t, map[resource.PropertyKey][]resource.URN{
				"foo": {urn},
				"bar": {urn},
			}, resp.Dependencies)

			result, deps, _, err := monitor.Call("pkgA:m:typA", resource.PropertyMap{
				// Send this as an output value using the dependencies returned.
				"arg": resource.NewOutputProperty(resource.Output{
					Element:      resp.Outputs["foo"].SecretValue().Element,
					Known:        true,
					Secret:       true,
					Dependencies: []resource.URN{urn},
				}),
			}, nil, "", "", "")
			assert.NoError(t, err)

			// Assert that the outputs are received as just plain values because SDKs don't yet support output
			// values returned from Call.
			assert.Equal(t, resource.PropertyMap{
				"foo": resource.MakeSecret(resource.NewStringProperty("foo")),
				"bar": resource.MakeComputed(resource.NewStringProperty("")),
			}, result)
			assert.Equal(t, map[resource.PropertyKey][]resource.URN{
				"foo": {urn},
				"bar": {urn},
			}, deps)

			return nil
		})
		hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

		p := &lt.TestPlan{
			Options: lt.TestUpdateOptions{T: t, HostF: hostF},
		}

		project := p.GetProject()
		_, err := lt.TestOp(engine.Update).Run(project, p.GetTarget(t, nil), p.Options, true, p.BackendClient, nil)
		assert.NoError(t, err)
	}

	t.Run("WithGrpc", func(t *testing.T) {
		t.Parallel()
		test(t, deploytest.WithGrpc)
	})
	t.Run("WithoutGrpc", func(t *testing.T) {
		t.Parallel()
		test(t, deploytest.WithoutGrpc)
	})
}

// Tests that the engine deduplicates dependency information sent to Construct (remote component resource) and Call
// (remote component method) methods on a provider.
func TestConstructCallDependencyDedeuplication(t *testing.T) {
	t.Parallel()

	test := func(t *testing.T, opt deploytest.PluginOption) {
		loaders := []*deploytest.ProviderLoader{
			deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
				return &deploytest.Provider{
					CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
						return plugin.CreateResponse{
							ID:         "created-id",
							Properties: req.Properties,
							Status:     resource.StatusOK,
						}, nil
					},
					ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
						return plugin.ReadResponse{
							ReadResult: plugin.ReadResult{
								Inputs:  req.Inputs,
								Outputs: req.State,
							},
							Status: resource.StatusOK,
						}, nil
					},
					ConstructF: func(
						_ context.Context,
						req plugin.ConstructRequest,
						monitor *deploytest.ResourceMonitor,
					) (plugin.ConstructResponse, error) {
						// Arg was sent as an output but the dependency map should still be filled in for providers to look at
						assert.Equal(t,
							[]resource.URN{"urn:pulumi:test::test::pkgA:m:typC::resC"},
							req.Options.PropertyDependencies["arg"])

						resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{})
						assert.NoError(t, err)

						respA, err := monitor.RegisterResource("pkgA:m:typA", req.Name+"-a", true, deploytest.ResourceOptions{
							Parent: resp.URN,
						})
						assert.NoError(t, err)

						// Return a secret and unknown output depending on some internal resource
						deps := []resource.URN{respA.URN}
						return plugin.ConstructResponse{
							URN: resp.URN,
							Outputs: resource.PropertyMap{
								"foo": resource.MakeSecret(resource.NewStringProperty("foo")),
								"bar": resource.MakeComputed(resource.NewStringProperty("")),
							},
							OutputDependencies: map[resource.PropertyKey][]resource.URN{
								"foo": deps,
								"bar": deps,
							},
						}, nil
					},
					CallF: func(
						_ context.Context,
						req plugin.CallRequest,
						_ *deploytest.ResourceMonitor,
					) (plugin.CallResponse, error) {
						// Arg was sent as an output but the dependency map should still be filled in for providers to look at
						assert.Equal(t,
							[]resource.URN{"urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resA-a"},
							req.Options.ArgDependencies["arg"])

						// Assume a single output arg that this call depends on
						arg := req.Args["arg"]
						deps := arg.OutputValue().Dependencies

						return plugin.CallResponse{
							Return: resource.PropertyMap{
								"foo": resource.MakeSecret(resource.NewStringProperty("foo")),
								"bar": resource.MakeComputed(resource.NewStringProperty("")),
							},
							ReturnDependencies: map[resource.PropertyKey][]resource.URN{
								"foo": deps,
								"bar": deps,
							},
						}, nil
					},
				}, nil
			}, opt),
		}

		programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			respC, err := monitor.RegisterResource("pkgA:m:typC", "resC", false, deploytest.ResourceOptions{
				Inputs: resource.PropertyMap{
					"arg": resource.NewNumberProperty(1),
				},
			})
			assert.NoError(t, err)

			resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", false, deploytest.ResourceOptions{
				Remote: true,
				Inputs: resource.PropertyMap{
					"arg": resource.NewOutputProperty(resource.Output{
						Element:      respC.Outputs["arg"],
						Known:        true,
						Dependencies: []resource.URN{respC.URN},
					}),
				},
				PropertyDeps: map[resource.PropertyKey][]resource.URN{
					"arg": {respC.URN},
				},
			})
			assert.NoError(t, err)

			// The urn of the internal resource the component created
			urn := resource.URN("urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resA-a")

			// Assert that the outputs are received as just plain values because SDKs don't yet support output
			// values returned from RegisterResource.
			assert.Equal(t, resource.PropertyMap{
				"foo": resource.MakeSecret(resource.NewStringProperty("foo")),
				"bar": resource.MakeComputed(resource.NewStringProperty("")),
			}, resp.Outputs)
			assert.Equal(t, map[resource.PropertyKey][]resource.URN{
				"foo": {urn},
				"bar": {urn},
			}, resp.Dependencies)

			result, deps, _, err := monitor.Call("pkgA:m:typA", resource.PropertyMap{
				// Send this as an output value using the dependencies returned.
				"arg": resource.NewOutputProperty(resource.Output{
					Element:      resp.Outputs["foo"].SecretValue().Element,
					Known:        true,
					Secret:       true,
					Dependencies: []resource.URN{urn},
				}),
			}, map[resource.PropertyKey][]resource.URN{
				"arg": {urn},
			}, "", "", "")
			assert.NoError(t, err)

			// Assert that the outputs are received as just plain values because SDKs don't yet support output
			// values returned from Call.
			assert.Equal(t, resource.PropertyMap{
				"foo": resource.MakeSecret(resource.NewStringProperty("foo")),
				"bar": resource.MakeComputed(resource.NewStringProperty("")),
			}, result)
			assert.Equal(t, map[resource.PropertyKey][]resource.URN{
				"foo": {urn},
				"bar": {urn},
			}, deps)

			return nil
		})
		hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

		p := &lt.TestPlan{
			Options: lt.TestUpdateOptions{T: t, HostF: hostF},
		}

		project := p.GetProject()
		_, err := lt.TestOp(engine.Update).Run(project, p.GetTarget(t, nil), p.Options, true, p.BackendClient, nil)
		assert.NoError(t, err)
	}

	t.Run("WithGrpc", func(t *testing.T) {
		t.Parallel()
		test(t, deploytest.WithGrpc)
	})
	t.Run("WithoutGrpc", func(t *testing.T) {
		t.Parallel()
		test(t, deploytest.WithoutGrpc)
	})
}

// Tests that the a resource can be created from within a Call (remote component method) provider method implementation.
func TestSingleComponentMethodResourceDefaultProviderLifecycle(t *testing.T) {
	t.Parallel()

	var urn resource.URN
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			construct := func(
				_ context.Context,
				req plugin.ConstructRequest,
				monitor *deploytest.ResourceMonitor,
			) (plugin.ConstructResponse, error) {
				var err error
				resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{
					Parent:  req.Parent,
					Aliases: aliasesFromAliases(req.Options.Aliases),
					Protect: req.Options.Protect,
				})
				assert.NoError(t, err)
				urn = resp.URN

				_, err = monitor.RegisterResource("pkgA:m:typB", "resA", true, deploytest.ResourceOptions{
					Parent: resp.URN,
				})
				assert.NoError(t, err)

				outs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
				err = monitor.RegisterResourceOutputs(resp.URN, outs)
				assert.NoError(t, err)

				return plugin.ConstructResponse{
					URN:     resp.URN,
					Outputs: outs,
				}, nil
			}

			call := func(
				_ context.Context,
				req plugin.CallRequest,
				monitor *deploytest.ResourceMonitor,
			) (plugin.CallResponse, error) {
				_, err := monitor.RegisterResource("pkgA:m:typC", "resA", true, deploytest.ResourceOptions{
					Parent: urn,
				})
				assert.NoError(t, err)

				return plugin.CallResponse{}, nil
			}

			return &deploytest.Provider{
				ConstructF: construct,
				CallF:      call,
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", false, deploytest.ResourceOptions{
			Remote: true,
		})
		assert.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{
			"foo": resource.NewStringProperty("bar"),
		}, resp.Outputs)

		_, _, _, err = monitor.Call("pkgA:m:typA/methodA", resource.PropertyMap{}, nil, "", "", "")
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		// Skip display tests because different ordering makes the colouring different.
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
		Steps:   lt.MakeBasicLifecycleSteps(t, 4),
	}
	p.Run(t, nil)
}

// Tests that a remote component method (that is, one implemented by a provider's Call method) can be called and its
// outputs correctly read.
func TestSingleComponentMethodDefaultProviderLifecycle(t *testing.T) {
	t.Parallel()

	var urn resource.URN

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			construct := func(
				_ context.Context,
				req plugin.ConstructRequest,
				monitor *deploytest.ResourceMonitor,
			) (plugin.ConstructResponse, error) {
				var err error
				resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{
					Parent:  req.Parent,
					Aliases: aliasesFromAliases(req.Options.Aliases),
					Protect: req.Options.Protect,
				})
				assert.NoError(t, err)
				urn = resp.URN

				_, err = monitor.RegisterResource("pkgA:m:typB", "resA", true, deploytest.ResourceOptions{
					Parent: urn,
				})
				assert.NoError(t, err)

				outs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}
				err = monitor.RegisterResourceOutputs(urn, outs)
				assert.NoError(t, err)

				return plugin.ConstructResponse{
					URN:     urn,
					Outputs: outs,
				}, nil
			}

			call := func(
				_ context.Context,
				req plugin.CallRequest,
				monitor *deploytest.ResourceMonitor,
			) (plugin.CallResponse, error) {
				assert.Equal(t, resource.PropertyMap{
					"name": resource.NewStringProperty("Alice"),
				}, req.Args)
				name := req.Args["name"].StringValue()

				result, _, err := monitor.Invoke("pulumi:pulumi:getResource", resource.PropertyMap{
					"urn": resource.NewStringProperty(string(urn)),
				}, "", "", "")
				assert.NoError(t, err)
				state := result["state"]
				foo := state.ObjectValue()["foo"].StringValue()

				message := fmt.Sprintf("%s, %s!", name, foo)
				return plugin.CallResponse{
					Return: resource.PropertyMap{
						"message": resource.NewStringProperty(message),
					},
				}, nil
			}

			return &deploytest.Provider{
				ConstructF: construct,
				CallF:      call,
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", false, deploytest.ResourceOptions{
			Remote: true,
		})
		assert.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{
			"foo": resource.NewStringProperty("bar"),
		}, resp.Outputs)

		outs, _, _, err := monitor.Call("pkgA:m:typA/methodA", resource.PropertyMap{
			"name": resource.NewStringProperty("Alice"),
		}, nil, "", "", "")
		assert.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{
			"message": resource.NewStringProperty("Alice, bar!"),
		}, outs)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		// Skip display tests because different ordering makes the colouring different.
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
		Steps:   lt.MakeBasicLifecycleSteps(t, 4),
	}
	p.Run(t, nil)
}

// Tests that a remote component (that is, one implemented by a provider's Construct method) can be created and a
// returned registered resource reference correctly rehydrated by the program using the pulumi:pulumi:getResource
// invoke.
func TestComponentRegisteredResourceOutputCanBeHydratedByProgram(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	project := p.GetProject()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("0.0.1"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				// This Create implementation will be called as a result of the custom resource registration (of type
				// pkgA:index:Custom) in the Component Construct implementation.
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if req.URN.Type() == "pkgA:index:Custom" {
						return plugin.CreateResponse{
							ID:         resource.ID(req.URN.Name() + "-id"),
							Properties: req.Properties,
							Status:     resource.StatusOK,
						}, nil
					}

					return plugin.CreateResponse{}, fmt.Errorf("unexpected resource type %s", req.Type)
				},

				// Construct will be called as a result of the program registering a remote component of type
				// pkgA:index:Component.
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					rm *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					if req.Type == "pkgA:index:Component" {
						component, err := rm.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{
							Parent: req.Parent,
						})
						require.NoError(t, err)

						custom, err := rm.RegisterResource("pkgA:index:Custom", "custom", true, deploytest.ResourceOptions{
							Parent: component.URN,
							Inputs: resource.PropertyMap{
								"foo": resource.NewStringProperty("bar"),
							},
						})
						require.NoError(t, err)

						return plugin.ConstructResponse{
							URN: component.URN,
							Outputs: resource.PropertyMap{
								"custom": resource.MakeCustomResourceReference(custom.URN, custom.ID, ""),
							},
						}, nil
					}

					return plugin.ConstructResponse{}, fmt.Errorf("unexpected resource type %s", req.Type)
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		component, err := monitor.RegisterResource("pkgA:index:Component", "component", false, deploytest.ResourceOptions{
			Remote: true,
		})
		require.NoError(t, err)

		customResRef := component.Outputs["custom"].ResourceReferenceValue()

		state, _, err := monitor.Invoke(
			"pulumi:pulumi:getResource",
			resource.PropertyMap{
				"urn": resource.NewStringProperty(string(customResRef.URN)),
			},
			"", /*provider*/
			"", /*version*/
			"", /*packageRef*/
		)
		require.NoError(t, err)
		require.Equal(
			t,
			resource.PropertyMap{
				"urn": resource.NewStringProperty(string(customResRef.URN)),
				"id":  resource.NewStringProperty(customResRef.ID.StringValue()),
				"state": resource.NewObjectProperty(resource.PropertyMap{
					"foo": resource.NewStringProperty("bar"),
				}),
			},
			state,
		)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p.Options = lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
	}

	_, err := lt.TestOp(engine.Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
}

// Tests that a remote component (that is, one implemented by a provider's Construct method) can be created and a
// returned registered resource reference correctly rehydrated by another component using the pulumi:pulumi:getResource
// invoke.
func TestComponentRegisteredResourceOutputCanBeHydratedByComponent(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	project := p.GetProject()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("0.0.1"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				// This Create implementation will be called as a result of the custom resource registration (of type
				// pkgA:index:Custom) in the Component Construct implementation.
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					if req.URN.Type() == "pkgA:index:Custom" {
						return plugin.CreateResponse{
							ID:         resource.ID(req.URN.Name() + "-id"),
							Properties: req.Properties,
							Status:     resource.StatusOK,
						}, nil
					}

					return plugin.CreateResponse{}, fmt.Errorf("unexpected resource type %s", req.Type)
				},

				// Construct will be called as a result of the program registering remote components of type
				// pkgA:index:Component1 and pkgA:index:Component2.
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					rm *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					if req.Type == "pkgA:index:Component1" {
						component, err := rm.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{
							Parent: req.Parent,
						})
						require.NoError(t, err)

						custom, err := rm.RegisterResource("pkgA:index:Custom", "custom", true, deploytest.ResourceOptions{
							Parent: component.URN,
							Inputs: resource.PropertyMap{
								"foo": resource.NewStringProperty("bar"),
							},
						})
						require.NoError(t, err)

						return plugin.ConstructResponse{
							URN: component.URN,
							Outputs: resource.PropertyMap{
								"custom": resource.MakeCustomResourceReference(custom.URN, custom.ID, ""),
							},
						}, nil
					}

					if req.Type == "pkgA:index:Component2" {
						component, err := rm.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{
							Parent: req.Parent,
						})
						require.NoError(t, err)

						customResRef := req.Inputs["custom"].ResourceReferenceValue()

						state, _, err := rm.Invoke(
							"pulumi:pulumi:getResource",
							resource.PropertyMap{
								"urn": resource.NewStringProperty(string(customResRef.URN)),
							},
							"", /*provider*/
							"", /*version*/
							"", /*packageRef*/
						)
						require.NoError(t, err)
						require.Equal(
							t,
							resource.PropertyMap{
								"urn": resource.NewStringProperty(string(customResRef.URN)),
								"id":  resource.NewStringProperty(customResRef.ID.StringValue()),
								"state": resource.NewObjectProperty(resource.PropertyMap{
									"foo": resource.NewStringProperty("bar"),
								}),
							},
							state,
						)

						return plugin.ConstructResponse{
							URN:     component.URN,
							Outputs: resource.PropertyMap{},
						}, nil
					}

					return plugin.ConstructResponse{}, fmt.Errorf("unexpected resource type %s", req.Type)
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		component1, err := monitor.RegisterResource("pkgA:index:Component1", "component1", false, deploytest.ResourceOptions{
			Remote: true,
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:index:Component2", "component2", false, deploytest.ResourceOptions{
			Remote: true,
			Inputs: resource.PropertyMap{
				"custom": component1.Outputs["custom"],
			},
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p.Options = lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
	}

	_, err := lt.TestOp(engine.Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
}

// Tests that a remote component (that is, one implemented by a provider's Construct method) can be created and a
// returned read resource reference correctly rehydrated by the program using the pulumi:pulumi:getResource invoke.
func TestComponentReadResourceOutputCanBeHydratedByProgram(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	project := p.GetProject()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("0.0.1"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				// This Read implementation will be called as a result of the custom resource read (of type pkgA:index:Custom)
				// in the Component Construct implementation.
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					if req.URN.Type() == "pkgA:index:Custom" {
						return plugin.ReadResponse{
							ReadResult: plugin.ReadResult{
								ID:      req.ID,
								Outputs: req.Inputs,
							},
							Status: resource.StatusOK,
						}, nil
					}

					return plugin.ReadResponse{}, fmt.Errorf("unexpected resource type %s", req.Type)
				},

				// Construct will be called as a result of the program registering a remote component of type
				// pkgA:index:Component.
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					rm *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					if req.Type == "pkgA:index:Component" {
						component, err := rm.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{
							Parent: req.Parent,
						})
						require.NoError(t, err)

						customID := resource.ID("custom-id")
						customURN, _, err := rm.ReadResource(
							"pkgA:index:Custom",
							"custom",
							customID,
							component.URN, /*parent*/
							resource.PropertyMap{
								"foo": resource.NewStringProperty("bar"),
							},
							"", /*provider*/
							"", /*version*/
							"", /*sourcePosition*/
							"", /*packageRef*/
						)
						require.NoError(t, err)

						return plugin.ConstructResponse{
							URN: component.URN,
							Outputs: resource.PropertyMap{
								"custom": resource.MakeCustomResourceReference(customURN, customID, ""),
							},
						}, nil
					}

					return plugin.ConstructResponse{}, fmt.Errorf("unexpected resource type %s", req.Type)
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		component, err := monitor.RegisterResource("pkgA:index:Component", "component", false, deploytest.ResourceOptions{
			Remote: true,
		})
		require.NoError(t, err)

		customResRef := component.Outputs["custom"].ResourceReferenceValue()

		state, _, err := monitor.Invoke(
			"pulumi:pulumi:getResource",
			resource.PropertyMap{
				"urn": resource.NewStringProperty(string(customResRef.URN)),
			},
			"", /*provider*/
			"", /*version*/
			"", /*packageRef*/
		)
		require.NoError(t, err)
		require.Equal(
			t,
			resource.PropertyMap{
				"urn": resource.NewStringProperty(string(customResRef.URN)),
				"id":  resource.NewStringProperty(customResRef.ID.StringValue()),
				"state": resource.NewObjectProperty(resource.PropertyMap{
					"foo": resource.NewStringProperty("bar"),
				}),
			},
			state,
		)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p.Options = lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
	}

	_, err := lt.TestOp(engine.Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
}

// Tests that a remote component (that is, one implemented by a provider's Construct method) can be created and a
// returned read resource reference correctly rehydrated by another component using the pulumi:pulumi:getResource
// invoke.
func TestComponentReadResourceOutputCanBeHydratedByComponent(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	project := p.GetProject()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("0.0.1"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				// This Read implementation will be called as a result of the custom resource read (of type pkgA:index:Custom)
				// in the Component Construct implementation.
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					if req.URN.Type() == "pkgA:index:Custom" {
						return plugin.ReadResponse{
							ReadResult: plugin.ReadResult{
								ID:      req.ID,
								Outputs: req.Inputs,
							},
							Status: resource.StatusOK,
						}, nil
					}

					return plugin.ReadResponse{}, fmt.Errorf("unexpected resource type %s", req.Type)
				},

				// Construct will be called as a result of the program registering remote components of type
				// pkgA:index:Component1 and pkgA:index:Component2.
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					rm *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					if req.Type == "pkgA:index:Component1" {
						component, err := rm.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{
							Parent: req.Parent,
						})
						require.NoError(t, err)

						customID := resource.ID("custom-id")
						customURN, _, err := rm.ReadResource(
							"pkgA:index:Custom",
							"custom",
							customID,
							component.URN, /*parent*/
							resource.PropertyMap{
								"foo": resource.NewStringProperty("bar"),
							},
							"", /*provider*/
							"", /*version*/
							"", /*sourcePosition*/
							"", /*packageRef*/
						)
						require.NoError(t, err)

						return plugin.ConstructResponse{
							URN: component.URN,
							Outputs: resource.PropertyMap{
								"custom": resource.MakeCustomResourceReference(customURN, customID, ""),
							},
						}, nil
					}

					if req.Type == "pkgA:index:Component2" {
						component, err := rm.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{
							Parent: req.Parent,
						})
						require.NoError(t, err)

						customResRef := req.Inputs["custom"].ResourceReferenceValue()

						state, _, err := rm.Invoke(
							"pulumi:pulumi:getResource",
							resource.PropertyMap{
								"urn": resource.NewStringProperty(string(customResRef.URN)),
							},
							"", /*provider*/
							"", /*version*/
							"", /*packageRef*/
						)
						require.NoError(t, err)
						require.Equal(
							t,
							resource.PropertyMap{
								"urn": resource.NewStringProperty(string(customResRef.URN)),
								"id":  resource.NewStringProperty(customResRef.ID.StringValue()),
								"state": resource.NewObjectProperty(resource.PropertyMap{
									"foo": resource.NewStringProperty("bar"),
								}),
							},
							state,
						)

						return plugin.ConstructResponse{
							URN:     component.URN,
							Outputs: resource.PropertyMap{},
						}, nil
					}

					return plugin.ConstructResponse{}, fmt.Errorf("unexpected resource type %s", req.Type)
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		component1, err := monitor.RegisterResource("pkgA:index:Component1", "component1", false, deploytest.ResourceOptions{
			Remote: true,
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:index:Component2", "component2", false, deploytest.ResourceOptions{
			Remote: true,
			Inputs: resource.PropertyMap{
				"custom": component1.Outputs["custom"],
			},
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p.Options = lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
	}

	_, err := lt.TestOp(engine.Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
}
