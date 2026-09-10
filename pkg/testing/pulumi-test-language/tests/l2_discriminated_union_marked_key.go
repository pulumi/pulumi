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

				var firstOutputs, secondInputs resource.PropertyMap
				for _, evt := range res.Events {
					if evt.Type != engine.ResourceOutputsEvent {
						continue
					}
					payload := evt.Payload().(engine.ResourceOutputsEventPayload)
					switch payload.Metadata.URN.Name() {
					case "first":
						firstOutputs = payload.Metadata.New.Outputs
					case "second":
						secondInputs = payload.Metadata.New.Inputs
					}
				}
				require.NotNil(l, firstOutputs, "expected a resource outputs event for first")
				require.NotNil(l, secondInputs, "expected a resource outputs event for second")

				// The provider plans unionOut with an unknown discriminator.
				assert.Equal(l, resource.NewProperty(resource.PropertyMap{
					"discriminantKind": resource.MakeComputed(resource.NewProperty("")),
					"field1":           resource.NewProperty("hello"),
				}), firstOutputs["unionOut"])

				// The unknown discriminator flows through the program into second's input. SDKs may
				// collapse the partially known union into a fully unknown value, so only require
				// that the input still contains unknowns.
				unionIn, has := secondInputs["unionIn"]
				require.True(l, has, "expected second to have input unionIn")
				assert.True(l, unionIn.ContainsUnknowns(),
					"expected second's unionIn input to be unknown: %v", unionIn)
			},
			Assert: func(l *L, res AssertArgs) {
				RequireStackResource(l, res.Err, res.Changes)

				secretUnion := resource.NewProperty(resource.PropertyMap{
					"discriminantKind": resource.MakeSecret(resource.NewProperty("variant1")),
					"field1":           resource.NewProperty("hello"),
				})
				plainUnion := resource.NewProperty(resource.PropertyMap{
					"discriminantKind": resource.NewProperty("variant1"),
					"field1":           resource.NewProperty("hello"),
				})

				first := RequireSingleNamedResource(l, res.Snap.Resources, "first")
				require.Equal(l, resource.PropertyMap{
					"unionIn": resource.NewProperty(resource.PropertyMap{
						"discriminantKind": resource.NewProperty("variant2"),
						"field2":           resource.NewProperty("known"),
					}),
					"unionOut": secretUnion,
				}, first.Outputs)

				// The secret discriminator flows through the program into second's input. SDKs may
				// promote the nested secret to the whole union value, so compare with the secret
				// markers stripped and assert secretness separately.
				second := RequireSingleNamedResource(l, res.Snap.Resources, "second")
				unionIn, has := second.Inputs["unionIn"]
				require.True(l, has, "expected second to have input unionIn")
				assert.True(l, unionIn.ContainsSecrets(),
					"expected second's unionIn input to be secret: %v", unionIn)
				assert.Equal(l, plainUnion, stripSecrets(unionIn))
				require.Equal(l, secretUnion, second.Outputs["unionOut"])

				stack := RequireSingleResource(l, res.Snap.Resources, resource.RootStackType)
				out, has := stack.Outputs["out"]
				require.True(l, has, "expected stack output out")
				assert.True(l, out.ContainsSecrets(), "expected stack output out to be secret: %v", out)
				assert.Equal(l, plainUnion, stripSecrets(out))
			},
		}},
	}
}

// stripSecrets removes all secret markers from a property value, leaving the underlying values in
// place.
func stripSecrets(v resource.PropertyValue) resource.PropertyValue {
	switch {
	case v.IsSecret():
		return stripSecrets(v.SecretValue().Element)
	case v.IsObject():
		obj := resource.PropertyMap{}
		for k, e := range v.ObjectValue() {
			obj[k] = stripSecrets(e)
		}
		return resource.NewProperty(obj)
	case v.IsArray():
		arr := make([]resource.PropertyValue, len(v.ArrayValue()))
		for i, e := range v.ArrayValue() {
			arr[i] = stripSecrets(e)
		}
		return resource.NewProperty(arr)
	}
	return v
}
