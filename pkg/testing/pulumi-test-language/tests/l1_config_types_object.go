// Copyright 2025, Pulumi Corporation.
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
	"maps"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/assert"
)

func init() {
	requiredConfig := config.Map{
		config.MustMakeKey("l1-config-types-object", "aMap"):      config.NewObjectValue("{\"a\": 1, \"b\": 2}"),
		config.MustMakeKey("l1-config-types-object", "anObject"):  config.NewObjectValue("{\"prop\": [true]}"),
		config.MustMakeKey("l1-config-types-object", "anyObject"): config.NewObjectValue("{\"a\": 10, \"b\": 20}"),
	}

	optionalConfig := config.Map{
		config.MustMakeKey("l1-config-types-object", "optionalList"): config.NewObjectValue(
			`["a","b"]`),
		config.MustMakeKey("l1-config-types-object", "optionalMap"): config.NewObjectValue(
			`{"key":"value"}`),
		config.MustMakeKey("l1-config-types-object", "optionalObject"): config.NewObjectValue(
			`{"prop":"value1","other":2}`),
	}
	maps.Copy(optionalConfig, requiredConfig)

	// The outputs that don't depend on the optional config values are the same in every run.
	assertCommonOutputs := func(l *L, outputs resource.PropertyMap) {
		assert.Equal(l, resource.PropertyMap{
			"theMap": resource.NewProperty(resource.PropertyMap{
				"a": resource.NewProperty(2.0),
				"b": resource.NewProperty(3.0),
			}),
			"theObject": resource.NewProperty(true),
			"theThing":  resource.NewProperty(30.0),

			// Default values
			"defaultUntypedObject": resource.NewProperty(resource.PropertyMap{
				"key": resource.NewProperty("value"),
			}),
		}, outputs)
	}

	// The optional config outputs are JSON encoded; assert and remove them so the remaining
	// outputs can be compared as a whole. assert.JSONEq is used because JSON whitespace and
	// key order differ between language runtimes.
	assertJSONOutput := func(l *L, outputs resource.PropertyMap, key resource.PropertyKey, want string) {
		assert.JSONEq(l, want, outputs[key].StringValue())
		delete(outputs, key)
	}

	LanguageTests["l1-config-types-object"] = LanguageTest{
		RunsShareSource: true,
		Runs: []TestRun{
			{
				// The optional (null-defaulted) config values are unset, so their defaults apply.
				Config: requiredConfig,
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")

					outputs := resource.ToResourcePropertyMap(stack.Outputs)
					assertJSONOutput(l, outputs, "optionalList", `null`)
					assertJSONOutput(l, outputs, "optionalMap", `null`)
					assertJSONOutput(l, outputs, "optionalObject", `null`)
					assertCommonOutputs(l, outputs)
				},
			},
			{
				// The optional config values are set. This run updates the stack from the previous
				// run, so nothing is created and RequireStackResource does not apply.
				Config: optionalConfig,
				Assert: func(l *L, res AssertArgs) {
					assert.Nil(l, res.Err, "expected no error, got %v", res.Err)
					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")

					outputs := resource.ToResourcePropertyMap(stack.Outputs)
					assertJSONOutput(l, outputs, "optionalList", `["a","b"]`)
					assertJSONOutput(l, outputs, "optionalMap", `{"key":"value"}`)
					assertJSONOutput(l, outputs, "optionalObject", `{"prop":"value1","other":2}`)
					assertCommonOutputs(l, outputs)
				},
			},
		},
	}
}
