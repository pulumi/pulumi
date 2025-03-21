// Copyright 2016-2022, Pulumi Corporation.
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

package pulumi

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// The test is extracted from a panic using pulumi-docker and minified
// while still reproducing the panic. The issue is that the resource
// constructor `NewImage` processes `StringInput` and logs into a
// captured `*pulumi.Context` from `ApplyT`. The user program passes a
// vanilla `String`. The `ApplyT` is not tracked against the context
// join group, but the logging is, which causes the logging statement
// to appear as "dynamic" work that appeared unexpectedly after "all
// work was done", and race with the program completion `Wait`.
//
// The test is was made to pass by using a custom-made `workGroup`.
func TestLoggingFromApplyCausesNoPanics(t *testing.T) {
	t.Parallel()

	// Usually panics on iteration 100-200
	for i := 0; i < 1000; i++ {
		t.Logf("Iteration %d\n", i)
		mocks := &testMonitor{}
		err := RunErr(func(ctx *Context) error {
			String("X").ToStringOutput().ApplyT(func(string) int {
				err := ctx.Log.Debug("Zzz", &LogArgs{})
				assert.NoError(t, err)
				return 0
			})
			return nil
		}, WithMocks("project", "stack", mocks))
		assert.NoError(t, err)
	}
}

func TestRunningUnderMocks(t *testing.T) {
	t.Parallel()

	t.Run("With mocks", func(t *testing.T) {
		t.Parallel()
		testCtxState := &contextState{
			monitor: &mockMonitor{},
		}
		testCtx := &Context{
			state: testCtxState,
		}
		assert.True(t, testCtx.RunningWithMocks())
	})

	t.Run("Without mocks", func(t *testing.T) {
		t.Parallel()
		testCtxState := &contextState{
			monitor: nil,
		}
		testCtx := &Context{
			state: testCtxState,
		}
		assert.False(t, testCtx.RunningWithMocks())
	})
}

// An extended version of `TestLoggingFromApplyCausesNoPanics`, more
// realistically demonstrating the original usage pattern.
func TestLoggingFromResourceApplyCausesNoPanics(t *testing.T) {
	t.Parallel()

	// Usually panics on iteration 100-200
	for i := 0; i < 1000; i++ {
		t.Logf("Iteration %d\n", i)
		mocks := &testMonitor{}
		err := RunErr(func(ctx *Context) error {
			_, err := NewLoggingTestResource(t, ctx, "res", String("A"))
			assert.NoError(t, err)
			return nil
		}, WithMocks("project", "stack", mocks))
		assert.NoError(t, err)
	}
}

type LoggingTestResource struct {
	ResourceState
	TestOutput StringOutput
}

func NewLoggingTestResource(
	t *testing.T,
	ctx *Context,
	name string,
	input StringInput,
	opts ...ResourceOption,
) (*LoggingTestResource, error) {
	resource := &LoggingTestResource{}
	err := ctx.RegisterComponentResource("test:go:NewLoggingTestResource", name, resource, opts...)
	if err != nil {
		return nil, err
	}

	resource.TestOutput = input.ToStringOutput().ApplyT(func(inputValue string) (string, error) {
		time.Sleep(10 * time.Nanosecond)
		err := ctx.Log.Debug("Zzz", &LogArgs{})
		assert.NoError(t, err)
		return inputValue, nil
	}).(StringOutput)

	outputs := Map(map[string]Input{
		"testOutput": resource.TestOutput,
	})

	err = ctx.RegisterResourceOutputs(resource, outputs)
	if err != nil {
		return nil, err
	}

	return resource, nil
}

// A contrived test demonstrating queueing work dynamically (`ApplyT`
// called from a separate goroutine). This used to cause a panic but
// is now resolved by using `workGroup`.
func TestWaitingCausesNoPanics(t *testing.T) {
	t.Parallel()

	for i := 0; i < 10; i++ {
		mocks := &testMonitor{}
		err := RunErr(func(ctx *Context) error {
			o, set, _ := ctx.NewOutput()
			go func() {
				set(1)
				o.ApplyT(func(x interface{}) interface{} { return x })
			}()
			return nil
		}, WithMocks("project", "stack", mocks))
		assert.NoError(t, err)
	}
}

func TestCollapseAliases(t *testing.T) {
	t.Parallel()

	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			assert.Equal(t, "test:resource:type", args.TypeToken)
			return "myID", resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
		},
	}

	testCases := []struct {
		parentAliases  []Alias
		childAliases   []Alias
		totalAliasUrns int
		results        []URN
	}{
		{
			parentAliases:  []Alias{},
			childAliases:   []Alias{},
			totalAliasUrns: 0,
			results:        []URN{},
		},
		{
			parentAliases:  []Alias{},
			childAliases:   []Alias{{Type: String("test:resource:child2")}},
			totalAliasUrns: 1,
			results:        []URN{"urn:pulumi:stack::project::test:resource:type$test:resource:child2::myres-child"},
		},
		{
			parentAliases:  []Alias{},
			childAliases:   []Alias{{Name: String("child2")}},
			totalAliasUrns: 1,
			results:        []URN{"urn:pulumi:stack::project::test:resource:type$test:resource:child::child2"},
		},
		{
			parentAliases:  []Alias{{Type: String("test:resource:type3")}},
			childAliases:   []Alias{{Name: String("myres-child2")}},
			totalAliasUrns: 3,
			results: []URN{
				"urn:pulumi:stack::project::test:resource:type$test:resource:child::myres-child2",
				"urn:pulumi:stack::project::test:resource:type3$test:resource:child::myres-child",
				"urn:pulumi:stack::project::test:resource:type3$test:resource:child::myres-child2",
			},
		},
		{
			parentAliases:  []Alias{{Name: String("myres2")}},
			childAliases:   []Alias{{Name: String("myres-child2")}},
			totalAliasUrns: 3,
			results: []URN{
				"urn:pulumi:stack::project::test:resource:type$test:resource:child::myres-child2",
				"urn:pulumi:stack::project::test:resource:type$test:resource:child::myres2-child",
				"urn:pulumi:stack::project::test:resource:type$test:resource:child::myres2-child2",
			},
		},
		{
			parentAliases:  []Alias{{Name: String("myres2")}, {Type: String("test:resource:type3")}, {Name: String("myres3")}},
			childAliases:   []Alias{{Name: String("myres-child2")}, {Type: String("test:resource:child2")}},
			totalAliasUrns: 11,
			results: []URN{
				"urn:pulumi:stack::project::test:resource:type$test:resource:child::myres-child2",
				"urn:pulumi:stack::project::test:resource:type$test:resource:child2::myres-child",
				"urn:pulumi:stack::project::test:resource:type$test:resource:child::myres2-child",
				"urn:pulumi:stack::project::test:resource:type$test:resource:child::myres2-child2",
				"urn:pulumi:stack::project::test:resource:type$test:resource:child2::myres2-child",
				"urn:pulumi:stack::project::test:resource:type3$test:resource:child::myres-child",
				"urn:pulumi:stack::project::test:resource:type3$test:resource:child::myres-child2",
				"urn:pulumi:stack::project::test:resource:type3$test:resource:child2::myres-child",
				"urn:pulumi:stack::project::test:resource:type$test:resource:child::myres3-child",
				"urn:pulumi:stack::project::test:resource:type$test:resource:child::myres3-child2",
				"urn:pulumi:stack::project::test:resource:type$test:resource:child2::myres3-child",
			},
		},
	}

	for i := range testCases {
		testCase := testCases[i]
		err := RunErr(func(ctx *Context) error {
			var res testResource2
			err := ctx.RegisterResource("test:resource:type", "myres", &testResource2Inputs{}, &res,
				Aliases(testCase.parentAliases))
			assert.NoError(t, err)
			urns, err := ctx.collapseAliases(testCase.childAliases, "test:resource:child", "myres-child", &res)
			assert.NoError(t, err)
			assert.Len(t, urns, testCase.totalAliasUrns)
			var items []interface{}
			for _, item := range urns {
				items = append(items, item)
			}
			All(items...).ApplyT(func(urns interface{}) bool {
				assert.ElementsMatch(t, urns, testCase.results)
				return true
			})
			return nil
		}, WithMocks("project", "stack", mocks))
		assert.NoError(t, err)
	}
}

// Context with which to create a ProviderResource.
type Prov struct {
	name string
	t    string
}

// Invoke the creation
func (pr *Prov) i(ctx *Context, t *testing.T) ProviderResource {
	if pr == nil {
		return nil
	}
	p := &testProv{foo: pr.name}
	err := ctx.RegisterResource("pulumi:providers:"+pr.t, pr.name, nil, p)
	assert.NoError(t, err)
	return p
}

// Context with which to create a Resource.
type Res struct {
	name string
	t    string
	// Providers to register with
	parent *Prov
}

// Invoke the creation
func (rs *Res) i(ctx *Context, t *testing.T) Resource {
	if rs == nil {
		return nil
	}
	r := &testRes{foo: rs.name}
	var err error
	if rs.parent == nil {
		err = ctx.RegisterResource(rs.t, rs.name, nil, r)
	} else {
		err = ctx.RegisterResource(rs.t, rs.name, nil, r, Provider(rs.parent.i(ctx, t)))
	}
	assert.NoError(t, err)
	return r
}

func TestMergeProviders(t *testing.T) {
	t.Parallel()

	provType := func(t string) string {
		return "pulumi:providers:" + t
	}

	tests := []struct {
		t         string
		parent    *Res
		provider  *Prov
		providers []Prov

		// We expect that the names in expected match up with the providers in
		// the resulting map.
		expected []string
	}{
		{
			t:         provType("t"),
			providers: []Prov{{"t1", "t"}, {"r0", "r"}},
			expected:  []string{"t1", "r0"},
		},
		{
			t:         provType("t"),
			provider:  &Prov{"t0", "t"},
			providers: []Prov{{"t1", "t"}, {"r0", "r"}},
			// We expect that providers overrides provider
			expected: []string{"t1", "r0"},
		},
		{
			t:         provType("t"),
			provider:  &Prov{"t0", "t"},
			providers: []Prov{{"r0", "r"}},
			expected:  []string{"t0", "r0"},
		},
		{
			t:        provType("t"),
			parent:   &Res{"t0", "t", &Prov{"t1", "t"}},
			expected: []string{"t1"},
		},
		{
			t:        provType("t"),
			parent:   &Res{"t0", "t", nil},
			expected: []string{},
		},
		{
			t:         provType("t"),
			parent:    &Res{"t0", "t", &Prov{"t1", "t"}},
			provider:  &Prov{"t3", "t"},
			providers: []Prov{{"t2", "t"}},
			expected:  []string{"t2"},
		},
	}
	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for i, tt := range tests {
		i, tt := i, tt
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()

			err := RunErr(func(ctx *Context) error {
				providers := map[string]ProviderResource{}
				for _, p := range tt.providers {
					p := p // Move out of loop, for gosec
					providers[p.t] = p.i(ctx, t)
				}

				provMap, err := ctx.mergeProviders(tt.t, tt.parent.i(ctx, t), tt.provider.i(ctx, t), providers)
				if err != nil {
					return err
				}

				result := slice.Prealloc[string](len(provMap))
				for k, p := range provMap {
					assert.Equal(t, k, p.getPackage(), "pkg should match map key")
					result = append(result, strings.TrimPrefix(p.PulumiResourceName(), "pulumi:providers:"))
				}

				assert.ElementsMatch(t, tt.expected, result)
				return nil
			}, WithMocks("project", "stack", &testMonitor{}))
			assert.NoError(t, err)
		})
	}
}

func TestRegisterResource_aliasesSpecs(t *testing.T) {
	t.Parallel()

	parentURN := CreateURN(
		String("parent"),
		String("test:resource:parentType"),
		String(""),
		String("project"),
		String("stack"),
	)

	tests := []struct {
		desc string
		give []Alias

		// Whether the monitor supports aliasSpecs.
		supportsAliasSpecs bool

		// Specifies what we expect on the RegisterResourceRequest.
		// Typically, if a server supports AliasSpecs,
		// we won't send AliasURNs.
		wantAliases   []*pulumirpc.Alias
		wantAliasURNs []string
	}{
		{
			desc: "no parent/before alias specs",
			give: []Alias{
				{Name: String("resA"), NoParent: Bool(true)},
				{Name: String("resB"), NoParent: Bool(true)},
			},
			wantAliasURNs: []string{
				"urn:pulumi:stack::project::test:resource:type::resA",
				"urn:pulumi:stack::project::test:resource:type::resB",
			},
		},
		{
			desc:               "no parent/with alias specs",
			supportsAliasSpecs: true,
			give: []Alias{
				{Name: String("resA"), NoParent: Bool(true)},
				{Name: String("resB"), NoParent: Bool(true)},
			},
			wantAliases: []*pulumirpc.Alias{
				{
					Alias: &pulumirpc.Alias_Spec_{
						Spec: &pulumirpc.Alias_Spec{
							Name:   "resA",
							Parent: &pulumirpc.Alias_Spec_NoParent{NoParent: true},
						},
					},
				},
				{
					Alias: &pulumirpc.Alias_Spec_{
						Spec: &pulumirpc.Alias_Spec{
							Name:   "resB",
							Parent: &pulumirpc.Alias_Spec_NoParent{NoParent: true},
						},
					},
				},
			},
		},
		{
			desc: "parent urn/no alias specs",
			give: []Alias{
				{Name: String("child"), ParentURN: parentURN},
			},
			wantAliasURNs: []string{
				"urn:pulumi:stack::project::test:resource:parentType$test:resource:type::child",
			},
		},
		{
			desc: "parent urn/alias specs",
			give: []Alias{
				{Name: String("child"), ParentURN: parentURN},
			},
			supportsAliasSpecs: true,
			wantAliases: []*pulumirpc.Alias{
				{
					Alias: &pulumirpc.Alias_Spec_{
						Spec: &pulumirpc.Alias_Spec{
							Name: "child",
							Parent: &pulumirpc.Alias_Spec_ParentUrn{
								ParentUrn: "urn:pulumi:stack::project::test:resource:parentType::parent",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			var (
				gotAliases   []*pulumirpc.Alias
				gotAliasURNS []string
			)
			monitor := &testMonitor{
				NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
					gotAliases = append(gotAliases, args.RegisterRPC.Aliases...)
					gotAliasURNS = append(gotAliasURNS, args.RegisterRPC.AliasURNs...)
					return args.Name, resource.PropertyMap{}, nil
				},
			}

			opts := []RunOption{
				WithMocks("project", "stack", monitor),
			}

			// The mock resource monitor client does not support
			// alias specs.
			// So if that's needed, wrap the monitor to claim it
			// does.
			if !tt.supportsAliasSpecs {
				opts = append(opts, WrapResourceMonitorClient(
					func(rmc pulumirpc.ResourceMonitorClient) pulumirpc.ResourceMonitorClient {
						return resourceMonitorClientWithoutFeatures(rmc, "aliasSpecs")
					}))
			}

			err := RunErr(func(ctx *Context) error {
				var res testResource2
				err := ctx.RegisterResource(
					"test:resource:type",
					"resNew",
					&testResource2Inputs{Foo: String("oof")},
					&res,
					Aliases(tt.give),
				)
				require.NoError(t, err)
				return nil
			}, opts...)
			require.NoError(t, err)

			if tt.supportsAliasSpecs {
				assert.Equal(t, tt.wantAliases, gotAliases, "Aliases did not match")
			} else {
				assert.Equal(t, tt.wantAliasURNs, gotAliasURNS, "AliasURNs did not match")
			}
		})
	}
}

// resmonClientWithFeatures wraps a ResourceMonitorClient
// to report various additional features as supported.
type resmonClientWithFeatures struct {
	pulumirpc.ResourceMonitorClient

	notFeatures map[string]struct{}
}

// resourceMonitorClientWithOutFeatures builds a ResourceMonitorClient
// that reports the provided feature names as not supported
// even if it is supported in the client
func resourceMonitorClientWithoutFeatures(
	cl pulumirpc.ResourceMonitorClient,
	features ...string,
) pulumirpc.ResourceMonitorClient {
	notFeatureSet := make(map[string]struct{}, len(features))
	for _, f := range features {
		notFeatureSet[f] = struct{}{}
	}
	return &resmonClientWithFeatures{
		ResourceMonitorClient: cl,
		notFeatures:           notFeatureSet,
	}
}

func (c *resmonClientWithFeatures) SupportsFeature(
	ctx context.Context,
	req *pulumirpc.SupportsFeatureRequest,
	opts ...grpc.CallOption,
) (*pulumirpc.SupportsFeatureResponse, error) {
	if _, ok := c.notFeatures[req.GetId()]; ok {
		return &pulumirpc.SupportsFeatureResponse{
			HasSupport: false,
		}, nil
	}
	return c.ResourceMonitorClient.SupportsFeature(ctx, req, opts...)
}

func TestSourcePosition(t *testing.T) {
	t.Parallel()

	mocks := &testMonitor{
		NewResourceF: func(args MockResourceArgs) (string, resource.PropertyMap, error) {
			var sourcePosition *pulumirpc.SourcePosition
			switch {
			case args.RegisterRPC != nil:
				sourcePosition = args.RegisterRPC.SourcePosition
			case args.ReadRPC != nil:
				sourcePosition = args.ReadRPC.SourcePosition
			}

			require.NotNil(t, sourcePosition)
			assert.True(t, strings.HasSuffix(sourcePosition.Uri, "context_test.go"))

			return "myID", resource.PropertyMap{"foo": resource.NewStringProperty("qux")}, nil
		},
	}

	err := RunErr(func(ctx *Context) error {
		reg := func() error {
			var res testResource2
			return ctx.RegisterResource("test:resource:type", "reg", &testResource2Inputs{}, &res)
		}

		read := func() error {
			var res testResource2
			return ctx.ReadResource("test:resource:type", "read", ID("myid"), &testResource2Inputs{}, &res)
		}

		err := reg()
		require.NoError(t, err)

		err = read()
		require.NoError(t, err)

		return nil
	}, WithMocks("project", "stack", mocks))
	assert.NoError(t, err)
}

func TestWithValue(t *testing.T) {
	t.Parallel()

	key := "key"
	val := "val"
	testCtx := &Context{
		state: &contextState{},
		ctx:   context.Background(),
	}
	newCtx := testCtx.WithValue(key, val)

	assert.Equal(t, nil, testCtx.Value(key))
	assert.Equal(t, val, newCtx.Value(key))
	assert.Equal(t, newCtx.state, testCtx.state)
}

func TestInvokeOutput(t *testing.T) {
	t.Parallel()

	mocks := &testMonitor{
		CallF: func(args MockCallArgs) (resource.PropertyMap, error) {
			if args.Token == "test:invoke:fail" {
				return nil, errors.New("invoke error")
			}
			return resource.PropertyMap{"result": resource.NewStringProperty("success!")}, nil
		},
	}

	type invokeArgs struct {
		Arg string
	}

	err := RunErr(func(ctx *Context) error {
		outType := AnyOutput{}
		output := ctx.InvokeOutput("test:invoke:success", &invokeArgs{"will succeed"}, outType, InvokeOutputOptions{})
		ctx.Export("output", output)
		return nil
	}, WithMocks("project", "stack", mocks))
	require.NoError(t, err)

	err = RunErr(func(ctx *Context) error {
		outType := AnyOutput{}
		output := ctx.InvokeOutput("test:invoke:fail", &invokeArgs{"will fail"}, outType, InvokeOutputOptions{})
		ctx.Export("output", output)
		return nil
	}, WithMocks("project", "stack", mocks))
	require.ErrorContains(t, err, "invoke error")
}

func TestInvokePlainWithOutputArgument(t *testing.T) {
	// Unlike Node.js and Python, Go sensibly does not permit passing in outputs
	// as an argument to a plain invoke. This test verifies that we return an
	// error in this case.

	t.Parallel()

	mocks := &testMonitor{
		CallF: func(args MockCallArgs) (resource.PropertyMap, error) {
			return resource.PropertyMap{"result": resource.NewStringProperty("success!")}, nil
		},
	}

	type result struct {
		Result string
	}

	type InvokeOutputArgs struct {
		Arg StringInput `pulumi:"arg"`
	}

	args := InvokeOutputArgs{
		Arg: String("hello").ToStringOutput(),
	}

	err := RunErr(func(ctx *Context) error {
		res := result{}
		return ctx.Invoke("test:invoke:success", args, &res)
	}, WithMocks("project", "stack", mocks))
	require.ErrorContains(t, err, "cannot marshal an input of type")
}
