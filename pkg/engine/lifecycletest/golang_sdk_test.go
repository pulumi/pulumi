package lifecycletest

import (
	"context"
	"reflect"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/display"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type testResource struct {
	pulumi.CustomResourceState

	Foo pulumi.StringOutput `pulumi:"foo"`
}

type testResourceArgs struct {
	Foo  string `pulumi:"foo"`
	Bar  string `pulumi:"bar"`
	Baz  string `pulumi:"baz"`
	Bang string `pulumi:"bang"`
}

type testResourceInputs struct {
	Foo  pulumi.StringInput
	Bar  pulumi.StringInput
	Baz  pulumi.StringInput
	Bang pulumi.StringInput
}

func (*testResourceInputs) ElementType() reflect.Type {
	return reflect.TypeOf((*testResourceArgs)(nil))
}

func TestSingleResourceDefaultProviderGolangLifecycle(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		ctx, err := pulumi.NewContext(context.Background(), pulumi.RunInfo{
			Project:     info.Project,
			Stack:       info.Stack,
			Parallel:    info.Parallel,
			DryRun:      info.DryRun,
			MonitorAddr: info.MonitorAddress,
		})
		assert.NoError(t, err)

		return pulumi.RunWithContext(ctx, func(ctx *pulumi.Context) error {
			var resA testResource
			err := ctx.RegisterResource("pkgA:m:typA", "resA", &testResourceInputs{
				Foo: pulumi.String("bar"),
			}, &resA)
			assert.NoError(t, err)

			var resB testResource
			err = ctx.RegisterResource("pkgA:m:typA", "resB", &testResourceInputs{
				Baz: resA.Foo.ApplyT(func(v string) string {
					return v + "bar"
				}).(pulumi.StringOutput),
			}, &resB)
			assert.NoError(t, err)

			return nil
		})
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
		Steps:   MakeBasicLifecycleSteps(t, 4),
	}
	p.Run(t, nil)
}

// This test validates the wiring of the IgnoreChanges prop in the go SDK.
// It doesn't attempt to validate underlying behavior.
func TestIgnoreChangesGolangLifecycle(t *testing.T) {
	t.Parallel()

	var expectedIgnoreChanges []string

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
				DiffF: func(urn resource.URN, id resource.ID,
					oldInputs, oldOutputs, newInputs resource.PropertyMap, ignoreChanges []string,
				) (plugin.DiffResult, error) {
					// just verify that the IgnoreChanges prop made it through
					assert.Equal(t, expectedIgnoreChanges, ignoreChanges)
					return plugin.DiffResult{}, nil
				},
			}, nil
		}),
	}

	setupAndRunProgram := func(ignoreChanges []string) *deploy.Snapshot {
		programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			ctx, err := pulumi.NewContext(context.Background(), pulumi.RunInfo{
				Project:     info.Project,
				Stack:       info.Stack,
				Parallel:    info.Parallel,
				DryRun:      info.DryRun,
				MonitorAddr: info.MonitorAddress,
			})
			assert.NoError(t, err)

			return pulumi.RunWithContext(ctx, func(ctx *pulumi.Context) error {
				var res pulumi.CustomResourceState
				err := ctx.RegisterResource("pkgA:m:typA", "resA", nil, &res, pulumi.IgnoreChanges(ignoreChanges))
				assert.NoError(t, err)

				return nil
			})
		})

		hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
		p := &TestPlan{
			Options: TestUpdateOptions{HostF: hostF},
			Steps: []TestStep{
				{
					Op: Update,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						events []Event, err error,
					) error {
						for _, event := range events {
							if event.Type == ResourcePreEvent {
								payload := event.Payload().(ResourcePreEventPayload)
								assert.Equal(t, []display.StepOp{deploy.OpCreate}, []display.StepOp{payload.Metadata.Op})
							}
						}
						return err
					},
				},
			},
		}
		return p.Run(t, nil)
	}

	// ignore changes specified
	ignoreChanges := []string{"b"}
	setupAndRunProgram(ignoreChanges)

	// ignore changes empty
	ignoreChanges = []string{}
	setupAndRunProgram(ignoreChanges)
}

func TestExplicitDeleteBeforeReplaceGoSDK(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(urn resource.URN, oldInputs, oldOutputs, newInputs resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					if !oldOutputs["foo"].DeepEquals(newInputs["foo"]) {
						return plugin.DiffResult{
							ReplaceKeys:         []resource.PropertyKey{"foo"},
							DeleteBeforeReplace: true,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				DiffF: func(urn resource.URN, id resource.ID,
					oldInputs, oldOutputs, newInputs resource.PropertyMap, ignoreChanges []string,
				) (plugin.DiffResult, error) {
					if !oldOutputs["foo"].DeepEquals(newInputs["foo"]) {
						return plugin.DiffResult{ReplaceKeys: []resource.PropertyKey{"foo"}}, nil
					}
					return plugin.DiffResult{}, nil
				},
			}, nil
		}),
	}

	inputsA := &testResourceInputs{Foo: pulumi.String("foo")}

	dbrValue, dbrA := true, (*bool)(nil)
	getDbr := func() bool {
		if dbrA == nil {
			return false
		}
		return *dbrA
	}

	var stackURN, provURN, urnA resource.URN = "urn:pulumi:test::test::pulumi:pulumi:Stack::test-test",
		"urn:pulumi:test::test::pulumi:providers:pkgA::provA", "urn:pulumi:test::test::pkgA:m:typA::resA"
	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		ctx, err := pulumi.NewContext(context.Background(), pulumi.RunInfo{
			Project:     info.Project,
			Stack:       info.Stack,
			Parallel:    info.Parallel,
			DryRun:      info.DryRun,
			MonitorAddr: info.MonitorAddress,
		})
		assert.NoError(t, err)

		return pulumi.RunWithContext(ctx, func(ctx *pulumi.Context) error {
			provider := &pulumi.ProviderResourceState{}
			err := ctx.RegisterResource(string(providers.MakeProviderType("pkgA")), "provA", nil, provider)
			assert.NoError(t, err)

			var res pulumi.CustomResourceState
			err = ctx.RegisterResource("pkgA:m:typA", "resA", inputsA, &res,
				pulumi.Provider(provider), pulumi.DeleteBeforeReplace(getDbr()))
			assert.NoError(t, err)

			return nil
		})
	})

	p.Options.HostF = deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p.Steps = []TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	// Change the value of resA.A. Should create before replace
	inputsA.Foo = pulumi.String("bar")
	p.Steps = []TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			assert.NoError(t, err)

			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: stackURN},
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpCreateReplacement, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
			}, SuccessfulSteps(entries))

			return err
		},
	}}
	snap = p.Run(t, snap)

	// Change the registration of resA such that it requires delete-before-replace and change the value of resA.A.
	// replacement should be delete-before-replace.
	dbrA, inputsA.Foo = &dbrValue, pulumi.String("baz")
	p.Steps = []TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			assert.NoError(t, err)
			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: stackURN},
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpCreateReplacement, URN: urnA},
			}, SuccessfulSteps(entries))

			return err
		},
	}}
	p.Run(t, snap)
}

func TestReadResourceGolangLifecycle(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					assert.Equal(t, resource.ID("someId"), id)
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	var stackURN, defaultProviderURN, urnA resource.URN = "urn:pulumi:test::test::pulumi:pulumi:Stack::test-test",
		"urn:pulumi:test::test::pulumi:providers:pkgA::default", "urn:pulumi:test::test::pkgA:m:typA::resA"

	setupAndRunProgram := func() *deploy.Snapshot {
		programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			ctx, err := pulumi.NewContext(context.Background(), pulumi.RunInfo{
				Project:     info.Project,
				Stack:       info.Stack,
				Parallel:    info.Parallel,
				DryRun:      info.DryRun,
				MonitorAddr: info.MonitorAddress,
			})
			assert.NoError(t, err)

			return pulumi.RunWithContext(ctx, func(ctx *pulumi.Context) error {
				var res pulumi.CustomResourceState
				err := ctx.ReadResource("pkgA:m:typA", "resA", pulumi.ID("someId"), nil, &res)
				assert.NoError(t, err)

				return nil
			})
		})

		hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
		p := &TestPlan{
			Options: TestUpdateOptions{HostF: hostF},
			Steps: []TestStep{
				{
					Op: Update,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						evts []Event, err error,
					) error {
						assert.NoError(t, err)

						AssertSameSteps(t, []StepSummary{
							{Op: deploy.OpCreate, URN: stackURN},
							{Op: deploy.OpCreate, URN: defaultProviderURN},
							{Op: deploy.OpRead, URN: urnA},
						}, SuccessfulSteps(entries))

						return err
					},
				},
			},
		}
		return p.Run(t, nil)
	}

	setupAndRunProgram()
}

// ensures that RegisterResource, ReadResource (TODO https://github.com/pulumi/pulumi/issues/3562),
// and Invoke all respect the provider hierarchy
// most specific providers are used first 1. resource.provider, 2. resource.providers, 3. resource.parent.providers
func TestProviderInheritanceGolangLifecycle(t *testing.T) {
	t.Parallel()

	type invokeArgs struct {
		Bang string `pulumi:"bang"`
		Bar  string `pulumi:"bar"`
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			v := &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
			}
			v.InvokeF = func(tok tokens.ModuleMember,
				inputs resource.PropertyMap,
			) (resource.PropertyMap, []plugin.CheckFailure, error) {
				assert.True(t, v.Config.DeepEquals(inputs))
				return nil, nil, nil
			}
			return v, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			v := &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
			}
			v.InvokeF = func(tok tokens.ModuleMember,
				inputs resource.PropertyMap,
			) (resource.PropertyMap, []plugin.CheckFailure, error) {
				assert.True(t, v.Config.DeepEquals(inputs))
				return nil, nil, nil
			}
			return v, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		ctx, err := pulumi.NewContext(context.Background(), pulumi.RunInfo{
			Project:     info.Project,
			Stack:       info.Stack,
			Parallel:    info.Parallel,
			DryRun:      info.DryRun,
			MonitorAddr: info.MonitorAddress,
		})
		assert.NoError(t, err)

		return pulumi.RunWithContext(ctx, func(ctx *pulumi.Context) error {
			// register a couple of providers, pass in some props that we can use to indentify it during invoke
			var providerA pulumi.ProviderResourceState
			err := ctx.RegisterResource(string(providers.MakeProviderType("pkgA")), "prov1",
				&testResourceInputs{
					Foo: pulumi.String("1"),
				}, &providerA)
			assert.NoError(t, err)
			var providerB pulumi.ProviderResourceState
			err = ctx.RegisterResource(string(providers.MakeProviderType("pkgB")), "prov2",
				&testResourceInputs{
					Bar:  pulumi.String("2"),
					Bang: pulumi.String(""),
				}, &providerB)
			assert.NoError(t, err)
			var providerBOverride pulumi.ProviderResourceState
			err = ctx.RegisterResource(string(providers.MakeProviderType("pkgB")), "prov3",
				&testResourceInputs{
					Bar:  pulumi.String(""),
					Bang: pulumi.String("3"),
				}, &providerBOverride)
			assert.NoError(t, err)
			parentProviders := make(map[string]pulumi.ProviderResource)
			parentProviders["pkgA"] = &providerA
			parentProviders["pkgB"] = &providerB
			// create a parent resource that uses provider map
			var parentResource pulumi.CustomResourceState
			err = ctx.RegisterResource("pkgA:m:typA", "resA", nil, &parentResource, pulumi.ProviderMap(parentProviders))
			assert.NoError(t, err)
			// parent uses specified provider from map
			parentResultProvider := parentResource.GetProvider("pkgA:m:typA")
			assert.Equal(t, &providerA, parentResultProvider)

			// create a child resource
			var childResource pulumi.CustomResourceState
			err = ctx.RegisterResource("pkgB:m:typB", "resBChild", nil, &childResource, pulumi.Parent(&parentResource))
			assert.NoError(t, err)

			// child uses provider value from parent
			childResultProvider := childResource.GetProvider("pkgB:m:typB")
			assert.Equal(t, &providerB, childResultProvider)

			// create a child with a provider specified
			var childWithOverride pulumi.CustomResourceState
			err = ctx.RegisterResource("pkgB:m:typB", "resBChildOverride", nil, &childWithOverride,
				pulumi.Parent(&parentResource), pulumi.Provider(&providerBOverride))
			assert.NoError(t, err)

			// child uses the specified provider, and not the provider from the parent
			childWithOverrideProvider := childWithOverride.GetProvider("pkgB:m:typB")
			assert.Equal(t, &providerBOverride, childWithOverrideProvider)

			// pass in a fake ID
			testID := pulumi.ID("testID")

			// read a resource that uses provider map
			var rereadParent pulumi.CustomResourceState
			err = ctx.ReadResource("pkgA:m:typA", "readResA", testID, nil, &rereadParent, pulumi.ProviderMap(parentProviders))
			assert.NoError(t, err)
			// parent uses specified provider from map
			parentResultProvider = rereadParent.GetProvider("pkgA:m:typA")
			assert.Equal(t, &providerA, parentResultProvider)

			// read a child resource
			var rereadChild pulumi.CustomResourceState
			err = ctx.ReadResource("pkgB:m:typB", "readResBChild", testID, nil, &rereadChild, pulumi.Parent(&parentResource))
			assert.NoError(t, err)

			// child uses provider value from parent
			childResultProvider = rereadChild.GetProvider("pkgB:m:typB")
			assert.Equal(t, &providerB, childResultProvider)

			// read a child with a provider specified
			var rereadChildWithOverride pulumi.CustomResourceState
			err = ctx.ReadResource("pkgB:m:typB", "readResBChildOverride", testID, nil, &rereadChildWithOverride,
				pulumi.Parent(&parentResource), pulumi.Provider(&providerBOverride))
			assert.NoError(t, err)

			// child uses the specified provider, and not the provider from the parent
			childWithOverrideProvider = rereadChildWithOverride.GetProvider("pkgB:m:typB")
			assert.Equal(t, &providerBOverride, childWithOverrideProvider)

			// invoke with specific provider
			var invokeResult struct{}
			err = ctx.Invoke("pkgB:do:something", invokeArgs{
				Bang: "3",
			}, &invokeResult, pulumi.Provider(&providerBOverride))
			assert.NoError(t, err)

			// invoke with parent
			err = ctx.Invoke("pkgB:do:something", invokeArgs{
				Bar: "2",
			}, &invokeResult, pulumi.Parent(&parentResource))
			assert.NoError(t, err)

			// invoke with parent and provider
			err = ctx.Invoke("pkgB:do:something", invokeArgs{
				Bang: "3",
			}, &invokeResult, pulumi.Parent(&parentResource), pulumi.Provider(&providerBOverride))
			assert.NoError(t, err)

			return nil
		})
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
		Steps:   []TestStep{{Op: Update}},
	}
	p.Run(t, nil)
}

// This test validates the wiring of the ReplaceOnChanges prop in the go SDK.
func TestReplaceOnChangesGolangLifecycle(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	resourceProperties := &testResourceInputs{
		Foo: pulumi.String("bar"),
	}

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		ctx, err := pulumi.NewContext(context.Background(), pulumi.RunInfo{
			Project:     info.Project,
			Stack:       info.Stack,
			Parallel:    info.Parallel,
			DryRun:      info.DryRun,
			MonitorAddr: info.MonitorAddress,
		})
		assert.NoError(t, err)

		return pulumi.RunWithContext(ctx, func(ctx *pulumi.Context) error {
			var res pulumi.CustomResourceState
			err := ctx.RegisterResource("pkgA:m:typA", "resA", resourceProperties, &res,
				pulumi.ReplaceOnChanges([]string{"foo"}))
			assert.NoError(t, err)

			return nil
		})
	})

	expectedOps := []display.StepOp{deploy.OpCreate}

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
		Steps: []TestStep{
			{
				Op: Update,
				Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
					events []Event, err error,
				) error {
					collectedOps := make([]display.StepOp, 0)
					for _, event := range events {
						if event.Type == ResourcePreEvent {
							payload := event.Payload().(ResourcePreEventPayload)
							if payload.Metadata.URN == "urn:pulumi:test::test::pkgA:m:typA::resA" {
								collectedOps = append(collectedOps, payload.Metadata.Op)
							}
						}
					}

					assert.Equal(t, expectedOps, collectedOps)

					return err
				},
			},
		},
	}

	snap := p.Run(t, nil)
	assert.NotNil(t, snap)

	// Change the property Foo, should now replace
	resourceProperties = &testResourceInputs{
		Foo: pulumi.String("baz"),
	}
	expectedOps = []display.StepOp{deploy.OpCreateReplacement, deploy.OpReplace, deploy.OpDeleteReplaced}

	snap = p.Run(t, snap)
	assert.NotNil(t, snap)
}

type remoteComponentArgs struct {
	Foo pulumi.URN `pulumi:"foo"`
	Bar *string    `pulumi:"bar"`
}

type remoteComponentInputs struct {
	Foo pulumi.URNInput       `pulumi:"foo"`
	Bar pulumi.StringPtrInput `pulumi:"bar"`
}

func (*remoteComponentInputs) ElementType() reflect.Type {
	return reflect.TypeOf((*remoteComponentArgs)(nil)).Elem()
}

type remoteComponent struct {
	pulumi.ResourceState

	Foo pulumi.StringOutput `pulumi:"foo"`
	Baz pulumi.StringOutput `pulumi:"baz"`
}

func TestRemoteComponentGolang(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
			}, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(monitor *deploytest.ResourceMonitor, typ, name string, parent resource.URN,
					inputs resource.PropertyMap, info plugin.ConstructInfo, options plugin.ConstructOptions,
				) (plugin.ConstructResult, error) {
					_, ok := inputs["bar"]
					assert.False(t, ok)

					urn, _, _, err := monitor.RegisterResource("pkgB:index:component", "componentA", false)
					require.NoError(t, err)

					outs := resource.PropertyMap{}

					err = monitor.RegisterResourceOutputs(urn, outs)
					require.NoError(t, err)

					return plugin.ConstructResult{
						URN:     urn,
						Outputs: outs,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		ctx, err := pulumi.NewContext(context.Background(), pulumi.RunInfo{
			Project:     info.Project,
			Stack:       info.Stack,
			Parallel:    info.Parallel,
			DryRun:      info.DryRun,
			MonitorAddr: info.MonitorAddress,
		})
		require.NoError(t, err)

		return pulumi.RunWithContext(ctx, func(ctx *pulumi.Context) error {
			var resB pulumi.CustomResourceState
			err := ctx.RegisterResource("pkgA:index:typA", "resA", pulumi.Map{}, &resB)
			require.NoError(t, err)

			inputs := remoteComponentInputs{
				Foo: resB.URN(),
			}

			var res remoteComponent
			err = ctx.RegisterRemoteComponentResource("pkgB:index:component", "componentA", &inputs, &res)
			require.NoError(t, err)

			return nil
		})
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
		Steps:   []TestStep{{Op: Update}},
	}
	p.Run(t, nil)
}
