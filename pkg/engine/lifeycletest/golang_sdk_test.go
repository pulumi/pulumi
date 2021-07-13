//nolint:goconst
package lifecycletest

import (
	"context"
	"reflect"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"

	. "github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
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
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
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
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   MakeBasicLifecycleSteps(t, 4),
	}
	p.Run(t, nil)
}

// Inspired by transformations_test.go.
func TestSingleResourceDefaultProviderGolangTransformations(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	newResource := func(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) error {
		var res testResource
		return ctx.RegisterResource("pkgA:m:typA", name, &testResourceInputs{
			Foo: pulumi.String("bar"),
		}, &res, opts...)
	}

	newComponent := func(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) error {
		var res testResource
		err := ctx.RegisterComponentResource("pkgA:m:typA", name, &res, opts...)
		if err != nil {
			return err
		}

		var resChild testResource
		return ctx.RegisterResource("pkgA:m:typA", name+"Child", &testResourceInputs{
			Foo: pulumi.String("bar"),
		}, &resChild, pulumi.Parent(&res))
	}

	program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		ctx, err := pulumi.NewContext(context.Background(), pulumi.RunInfo{
			Project:     info.Project,
			Stack:       info.Stack,
			Parallel:    info.Parallel,
			DryRun:      info.DryRun,
			MonitorAddr: info.MonitorAddress,
		})
		assert.NoError(t, err)

		return pulumi.RunWithContext(ctx, func(ctx *pulumi.Context) error {
			// Scenario #1 - apply a transformation to a CustomResource
			res1Transformation := func(args *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
				// TODO[pulumi/pulumi#3846] We should use a mergeOptions-style API here.
				return &pulumi.ResourceTransformationResult{
					Props: args.Props,
					Opts:  append(args.Opts, pulumi.AdditionalSecretOutputs([]string{"output"})),
				}
			}
			assert.NoError(t, newResource(ctx, "res1",
				pulumi.Transformations([]pulumi.ResourceTransformation{res1Transformation})))

			// Scenario #2 - apply a transformation to a Component to transform its children
			res2Transformation := func(args *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
				if args.Name == "res2Child" {
					// TODO[pulumi/pulumi#3846] We should use a mergeOptions-style API here.
					return &pulumi.ResourceTransformationResult{
						Props: args.Props,
						Opts:  append(args.Opts, pulumi.AdditionalSecretOutputs([]string{"output", "output2"})),
					}
				}

				return nil
			}
			assert.NoError(t, newComponent(ctx, "res2",
				pulumi.Transformations([]pulumi.ResourceTransformation{res2Transformation})))

			// Scenario #3 - apply a transformation to the Stack to transform all (future) resources in the stack
			res3Transformation := func(args *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
				// Props might be nil.
				var props *testResourceInputs
				if args.Props == nil {
					props = &testResourceInputs{}
				} else {
					props = args.Props.(*testResourceInputs)
				}
				props.Foo = pulumi.String("baz")

				return &pulumi.ResourceTransformationResult{
					Props: props,
					Opts:  args.Opts,
				}
			}
			assert.NoError(t, ctx.RegisterStackTransformation(res3Transformation))
			assert.NoError(t, newResource(ctx, "res3"))

			// Scenario #4 - transformations are applied in order of decreasing specificity
			// 1. (not in this example) Child transformation
			// 2. First parent transformation
			// 3. Second parent transformation
			// 4. Stack transformation
			res4Transformation1 := func(args *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
				if args.Name == "res4Child" {
					props := args.Props.(*testResourceInputs)
					props.Foo = pulumi.String("baz1")

					return &pulumi.ResourceTransformationResult{
						Props: props,
						Opts:  args.Opts,
					}
				}
				return nil
			}
			res4Transformation2 := func(args *pulumi.ResourceTransformationArgs) *pulumi.ResourceTransformationResult {
				if args.Name == "res4Child" {
					props := args.Props.(*testResourceInputs)
					props.Foo = pulumi.String("baz2")

					return &pulumi.ResourceTransformationResult{
						Props: props,
						Opts:  args.Opts,
					}
				}
				return nil
			}
			assert.NoError(t, newComponent(ctx, "res4",
				pulumi.Transformations([]pulumi.ResourceTransformation{res4Transformation1, res4Transformation2})))

			return nil
		})
	})

	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}
	p.Steps = []TestStep{{
		Op: Update,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			_ []Event, res result.Result) result.Result {

			foundRes1 := false
			foundRes2 := false
			foundRes2Child := false
			foundRes3 := false
			foundRes4Child := false
			// foundRes5Child1 := false
			for _, res := range entries.Snap(target.Snapshot).Resources {
				// "res1" has a transformation which adds additionalSecretOutputs
				if res.URN.Name() == "res1" {
					foundRes1 = true
					assert.Equal(t, res.Type, tokens.Type("pkgA:m:typA"))
					assert.Contains(t, res.AdditionalSecretOutputs, resource.PropertyKey("output"))
				}
				// "res2" has a transformation which adds additionalSecretOutputs to it's "child"
				if res.URN.Name() == "res2" {
					foundRes2 = true
					assert.Equal(t, res.Type, tokens.Type("pkgA:m:typA"))
					assert.NotContains(t, res.AdditionalSecretOutputs, resource.PropertyKey("output"))
				}
				if res.URN.Name() == "res2Child" {
					foundRes2Child = true
					assert.Equal(t, res.Parent.Name(), tokens.QName("res2"))
					assert.Equal(t, res.Type, tokens.Type("pkgA:m:typA"))
					assert.Contains(t, res.AdditionalSecretOutputs, resource.PropertyKey("output"))
					assert.Contains(t, res.AdditionalSecretOutputs, resource.PropertyKey("output2"))
				}
				// "res3" is impacted by a global stack transformation which sets
				// Foo to "baz"
				if res.URN.Name() == "res3" {
					foundRes3 = true
					assert.Equal(t, "baz", res.Inputs["foo"].StringValue())
					assert.Len(t, res.Aliases, 0)
				}
				// "res4" is impacted by two component parent transformations which set
				// Foo to "baz1" and then "baz2" and also a global stack
				// transformation which sets optionalDefault to "baz".  The end
				// result should be "baz".
				if res.URN.Name() == "res4Child" {
					foundRes4Child = true
					assert.Equal(t, res.Parent.Name(), tokens.QName("res4"))
					assert.Equal(t, "baz", res.Inputs["foo"].StringValue())
				}
			}

			assert.True(t, foundRes1)
			assert.True(t, foundRes2)
			assert.True(t, foundRes2Child)
			assert.True(t, foundRes3)
			assert.True(t, foundRes4Child)
			return res
		},
	}}

	p.Run(t, nil)
}

// This test validates the wiring of the IgnoreChanges prop in the go SDK.
// It doesn't attempt to validate underlying behavior.
func TestIgnoreChangesGolangLifecycle(t *testing.T) {
	var expectedIgnoreChanges []string

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {
					// just verify that the IgnoreChanges prop made it through
					assert.Equal(t, expectedIgnoreChanges, ignoreChanges)
					return plugin.DiffResult{}, nil
				},
			}, nil
		}),
	}

	setupAndRunProgram := func(ignoreChanges []string) *deploy.Snapshot {
		program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
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

		host := deploytest.NewPluginHost(nil, nil, program, loaders...)
		p := &TestPlan{
			Options: UpdateOptions{Host: host},
			Steps: []TestStep{
				{
					Op: Update,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						events []Event, res result.Result) result.Result {
						for _, event := range events {
							if event.Type == ResourcePreEvent {
								payload := event.Payload().(ResourcePreEventPayload)
								assert.Equal(t, []deploy.StepOp{deploy.OpCreate}, []deploy.StepOp{payload.Metadata.Op})
							}
						}
						return res
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
	p := &TestPlan{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(urn resource.URN, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {
					if !olds["foo"].DeepEquals(news["foo"]) {
						return plugin.DiffResult{
							ReplaceKeys:         []resource.PropertyKey{"foo"},
							DeleteBeforeReplace: true,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

					if !olds["foo"].DeepEquals(news["foo"]) {
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
	program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
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

	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)
	p.Steps = []TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	// Change the value of resA.A. Should create before replace
	inputsA.Foo = pulumi.String("bar")
	p.Steps = []TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)

			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: stackURN},
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpCreateReplacement, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
			}, SuccessfulSteps(entries))

			return res
		},
	}}
	snap = p.Run(t, snap)

	// Change the registration of resA such that it requires delete-before-replace and change the value of resA.A.
	// replacement should be delete-before-replace.
	dbrA, inputsA.Foo = &dbrValue, pulumi.String("baz")
	p.Steps = []TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)
			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: stackURN},
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpCreateReplacement, URN: urnA},
			}, SuccessfulSteps(entries))

			return res
		},
	}}
	p.Run(t, snap)
}

func TestReadResourceGolangLifecycle(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					assert.Equal(t, resource.ID("someId"), id)
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	var stackURN, defaultProviderURN, urnA resource.URN = "urn:pulumi:test::test::pulumi:pulumi:Stack::test-test",
		"urn:pulumi:test::test::pulumi:providers:pkgA::default", "urn:pulumi:test::test::pkgA:m:typA::resA"

	setupAndRunProgram := func() *deploy.Snapshot {
		program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
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

		host := deploytest.NewPluginHost(nil, nil, program, loaders...)
		p := &TestPlan{
			Options: UpdateOptions{Host: host},
			Steps: []TestStep{
				{
					Op: Update,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						evts []Event, res result.Result) result.Result {

						assert.Nil(t, res)

						AssertSameSteps(t, []StepSummary{
							{Op: deploy.OpCreate, URN: stackURN},
							{Op: deploy.OpCreate, URN: defaultProviderURN},
							{Op: deploy.OpRead, URN: urnA},
						}, SuccessfulSteps(entries))

						return res
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
	type invokeArgs struct {
		Bang string `pulumi:"bang"`
		Bar  string `pulumi:"bar"`
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			v := &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
			}
			v.InvokeF = func(tok tokens.ModuleMember,
				inputs resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
				assert.True(t, v.Config.DeepEquals(inputs))
				return nil, nil, nil
			}
			return v, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			v := &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
			}
			v.InvokeF = func(tok tokens.ModuleMember,
				inputs resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
				assert.True(t, v.Config.DeepEquals(inputs))
				return nil, nil, nil
			}
			return v, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
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
			err = ctx.ReadResource("pkgA:m:typA", "readResA", testID, nil, &parentResource, pulumi.ProviderMap(parentProviders))
			assert.NoError(t, err)
			// parent uses specified provider from map
			parentResultProvider = parentResource.GetProvider("pkgA:m:typA")
			assert.Equal(t, &providerA, parentResultProvider)

			// read a child resource
			err = ctx.ReadResource("pkgB:m:typB", "readResBChild", testID, nil, &childResource, pulumi.Parent(&parentResource))
			assert.NoError(t, err)

			// child uses provider value from parent
			childResultProvider = childResource.GetProvider("pkgB:m:typB")
			assert.Equal(t, &providerB, childResultProvider)

			// read a child with a provider specified
			err = ctx.ReadResource("pkgB:m:typB", "readResBChildOverride", testID, nil, &childWithOverride,
				pulumi.Parent(&parentResource), pulumi.Provider(&providerBOverride))
			assert.NoError(t, err)

			// child uses the specified provider, and not the provider from the parent
			childWithOverrideProvider = childWithOverride.GetProvider("pkgB:m:typB")
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
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   []TestStep{{Op: Update}},
	}
	p.Run(t, nil)
}

// This test validates the wiring of the ReplaceOnChanges prop in the go SDK.
// It doesn't attempt to validate underlying behavior.
func TestReplaceOnChangesGolangLifecycle(t *testing.T) {
	var expectedReplaceOnChanges []string

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, replaceOnChanges []string) (plugin.DiffResult, error) {
					// just verify that the ReplaceOnChanges prop made it through
					assert.Equal(t, expectedReplaceOnChanges, replaceOnChanges)
					return plugin.DiffResult{}, nil
				},
			}, nil
		}),
	}

	setupAndRunProgram := func(replaceOnChanges []string) *deploy.Snapshot {
		program := deploytest.NewLanguageRuntime(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
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
				err := ctx.RegisterResource("pkgA:m:typA", "resA", nil, &res, pulumi.ReplaceOnChanges(replaceOnChanges))
				assert.NoError(t, err)

				return nil
			})
		})

		host := deploytest.NewPluginHost(nil, nil, program, loaders...)
		p := &TestPlan{
			Options: UpdateOptions{Host: host},
			Steps: []TestStep{
				{
					Op: Update,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						events []Event, res result.Result) result.Result {
						for _, event := range events {
							if event.Type == ResourcePreEvent {
								payload := event.Payload().(ResourcePreEventPayload)
								assert.Equal(t, []deploy.StepOp{deploy.OpCreate}, []deploy.StepOp{payload.Metadata.Op})
							}
						}
						return res
					},
				},
			},
		}
		return p.Run(t, nil)
	}

	// replace on changes specified
	replaceOnChanges := []string{"b"}
	setupAndRunProgram(replaceOnChanges)

	// replace on changes empty
	replaceOnChanges = []string{}
	setupAndRunProgram(replaceOnChanges)
}
