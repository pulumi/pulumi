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
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	nested := func(value string) resource.PropertyValue {
		return resource.NewProperty(resource.PropertyMap{
			"value":         resource.NewProperty(value),
			"constInNested": resource.NewProperty("nested-const"),
		})
	}

	expectedInputs := resource.PropertyMap{
		"name":        resource.NewProperty("example"),
		"directConst": resource.NewProperty("direct-const"),
		"nested":      nested("inner"),
		"arrayItems": resource.NewProperty([]resource.PropertyValue{
			nested("first"),
			nested("second"),
		}),
		"mapItems": resource.NewProperty(resource.PropertyMap{
			"one": nested("one-value"),
			"two": nested("two-value"),
		}),
	}

	LanguageTests["l2-const-values"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ConstValuesProvider{} },
		},
		Runs: []TestRun{{
			Assert: func(l *L, res AssertArgs) {
				RequireStackResource(l, res.Err, res.Changes)
				require.Len(l, res.Snap.Resources, 3, "expected 3 resources in snapshot")

				RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:const-values")
				instance := RequireSingleNamedResource(l, res.Snap.Resources, "instance")
				require.Equal(l, expectedInputs, instance.Inputs, "resource inputs")
				require.Equal(l, expectedInputs, instance.Outputs, "resource outputs")

				stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
				AssertPropertyMapMember(l, stack.Outputs, "invokeResult", resource.NewProperty(expectedInputs))
			},
		}},
	}
}
