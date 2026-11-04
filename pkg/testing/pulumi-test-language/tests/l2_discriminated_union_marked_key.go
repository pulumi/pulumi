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

package tests

import (
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests that a discriminated union whose discriminator property is unknown (during preview) or
// secret (during the actual update) flows through a program without error: from the resource that
// returns it, into another resource's union-typed input, and out to a stack output.
func init() {
	LanguageTests["l2-discriminated-union-marked-key"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.DiscriminatedUnionMarkedKeyProvider{} },
		},
		Runs: []TestRun{{
			AssertPreview: func(l *L, res AssertPreviewArgs) {
				RequireStackResource(l, res.Err, res.Changes)

				var firstOutputs, secondInputs property.Map
				var foundFirst, foundSecond bool
				for _, evt := range res.Events {
					if evt.Type != engine.ResourceOutputsEvent {
						continue
					}
					payload := evt.Payload().(engine.ResourceOutputsEventPayload)
					switch payload.Metadata.URN.Name() {
					case "first":
						firstOutputs = resource.FromResourcePropertyMap(payload.Metadata.New.Outputs)
						foundFirst = true
					case "second":
						secondInputs = resource.FromResourcePropertyMap(payload.Metadata.New.Inputs)
						foundSecond = true
					}
				}
				require.True(l, foundFirst, "expected a resource outputs event for first")
				require.True(l, foundSecond, "expected a resource outputs event for second")

				// The provider plans unionOut with an unknown discriminator.
				assert.Equal(l, property.New(map[string]property.Value{
					"discriminantKind": property.New(property.Computed),
					"field1":           property.New("hello"),
				}), firstOutputs.Get("unionOut"))

				// The unknown discriminator flows through the program into second's input. SDKs may
				// collapse the partially known union into a fully unknown value, so only require
				// that the input still contains unknowns.
				unionIn, has := secondInputs.GetOk("unionIn")
				require.True(l, has, "expected second to have input unionIn")
				assert.True(l, unionIn.HasComputed(),
					"expected second's unionIn input to be unknown: %v", unionIn)
			},
			Assert: func(l *L, res AssertArgs) {
				RequireStackResource(l, res.Err, res.Changes)

				secretUnion := property.New(map[string]property.Value{
					"discriminantKind": property.New("variant1").WithSecret(true),
					"field1":           property.New("hello"),
				})
				plainUnion := property.New(map[string]property.Value{
					"discriminantKind": property.New("variant1"),
					"field1":           property.New("hello"),
				})

				first := RequireSingleNamedResource(l, res.Snap.Resources, "first")
				require.Equal(l, property.NewMap(map[string]property.Value{
					"unionIn": property.New(map[string]property.Value{
						"discriminantKind": property.New("variant2"),
						"field2":           property.New("known"),
					}),
					"unionOut": secretUnion,
				}), resource.FromResourcePropertyMap(first.Outputs))

				// The secret discriminator flows through the program into second's input. SDKs may
				// promote the nested secret to the whole union value, so compare with the secret
				// markers stripped and assert secretness separately.
				second := RequireSingleNamedResource(l, res.Snap.Resources, "second")
				secondInputs := resource.FromResourcePropertyMap(second.Inputs)
				unionIn, has := secondInputs.GetOk("unionIn")
				require.True(l, has, "expected second to have input unionIn")
				assert.True(l, unionIn.HasSecrets(),
					"expected second's unionIn input to be secret: %v", unionIn)
				assert.Equal(l, plainUnion, stripSecrets(unionIn))
				require.Equal(l, secretUnion, resource.FromResourcePropertyMap(second.Outputs).Get("unionOut"))

				stack := RequireSingleResource(l, res.Snap.Resources, resource.RootStackType)
				stackOutputs := resource.FromResourcePropertyMap(stack.Outputs)
				out, has := stackOutputs.GetOk("out")
				require.True(l, has, "expected stack output out")
				assert.True(l, out.HasSecrets(), "expected stack output out to be secret: %v", out)
				assert.Equal(l, plainUnion, stripSecrets(out))
			},
		}},
	}
}

// stripSecrets removes all secret markers from a property value, leaving the underlying values in
// place.
func stripSecrets(v property.Value) property.Value {
	v = v.WithSecret(false)
	switch {
	case v.IsMap():
		obj := map[string]property.Value{}
		for k, e := range v.AsMap().AsMap() {
			obj[k] = stripSecrets(e)
		}
		return property.New(obj)
	case v.IsArray():
		slice := v.AsArray().AsSlice()
		arr := make([]property.Value, len(slice))
		for i, e := range slice {
			arr[i] = stripSecrets(e)
		}
		return property.New(arr)
	}
	return v
}
