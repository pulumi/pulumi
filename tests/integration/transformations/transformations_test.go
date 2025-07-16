// Copyright 2019-2024, Pulumi Corporation.
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

package ints

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var Dirs = []string{
	"simple",
}

func Validator(t *testing.T, stack integration.RuntimeValidationStackInfo) {
	randomResName := "testprovider:index:Random"
	foundRes1 := false
	foundRes2Child := false
	foundRes3 := false
	foundRes4Child := false
	foundRes5Child := false
	for _, res := range stack.Deployment.Resources {
		// "res1" has a transformation which adds additionalSecretOutputs
		if res.URN.Name() == "res1" {
			foundRes1 = true
			assert.Equal(t, res.Type, tokens.Type(randomResName))
			assert.Contains(t, res.AdditionalSecretOutputs, resource.PropertyKey("result"))
		}
		// "res2" has a transformation which adds additionalSecretOutputs to it's
		// "child"
		if res.URN.Name() == "res2-child" {
			foundRes2Child = true
			assert.Equal(t, res.Type, tokens.Type(randomResName))
			assert.Equal(t, res.Parent.Type(), tokens.Type("my:component:MyComponent"))
			assert.Contains(t, res.AdditionalSecretOutputs, resource.PropertyKey("result"))
			assert.Contains(t, res.AdditionalSecretOutputs, resource.PropertyKey("length"))
		}
		// "res3" is impacted by a global stack transformation which sets
		// optionalDefault to "stackDefault"
		if res.URN.Name() == "res3" {
			foundRes3 = true
			assert.Equal(t, res.Type, tokens.Type(randomResName))
			optionalPrefix := res.Inputs["prefix"]
			require.NotNil(t, optionalPrefix)
			assert.Equal(t, "stackDefault", optionalPrefix.(string))
		}
		// "res4" is impacted by two component parent transformations which set
		// optionalDefault to "default1" and then "default2" and also a global stack
		// transformation which sets optionalDefault to "stackDefault".  The end
		// result should be "stackDefault".
		if res.URN.Name() == "res4-child" {
			foundRes4Child = true
			assert.Equal(t, res.Type, tokens.Type(randomResName))
			assert.Equal(t, res.Parent.Type(), tokens.Type("my:component:MyComponent"))
			optionalPrefix := res.Inputs["prefix"]
			require.NotNil(t, optionalPrefix)
			assert.Equal(t, "stackDefault", optionalPrefix.(string))
		}
		// "res5" modifies one of its children to depend on another of its children.
		if res.URN.Name() == "res5-child1" {
			foundRes5Child = true
			assert.Equal(t, res.Type, tokens.Type(randomResName))
			assert.Equal(t, res.Parent.Type(), tokens.Type("my:component:MyOtherComponent"))
			// TODO[pulumi/pulumi#3282] Due to this bug, the dependency information
			// will not be correctly recorded in the state file, and so cannot be
			// verified here.
			//
			// assert.Len(t, res.PropertyDependencies, 1)
			input := res.Inputs["length"]
			require.NotNil(t, input)
			assert.Equal(t, 5.0, input.(float64))
		}
	}
	assert.True(t, foundRes1)
	assert.True(t, foundRes2Child)
	assert.True(t, foundRes3)
	assert.True(t, foundRes4Child)
	assert.True(t, foundRes5Child)
}
