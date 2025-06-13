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

package ints

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
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
	foundRes5 := false
	foundRes6 := false
	foundRes7 := false
	foundRes8 := false
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
			assert.NotNil(t, optionalPrefix)
			assert.Equal(t, "stackDefault", optionalPrefix)
			length := res.Inputs["length"]
			assert.NotNil(t, length)
			// length should be secret
			secret, ok := length.(map[string]interface{})
			assert.True(t, ok, "length should be a secret")
			assert.Equal(t, resource.SecretSig, secret[resource.SigKey])
			assert.Contains(t, res.AdditionalSecretOutputs, resource.PropertyKey("result"))
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
			assert.NotNil(t, optionalPrefix)
			assert.Equal(t, "stackDefault", optionalPrefix)
		}
		// "res5" should have mutated the length
		if res.URN.Name() == "res5" {
			foundRes5 = true
			assert.Equal(t, res.Type, tokens.Type(randomResName))
			length := res.Inputs["length"]
			assert.NotNil(t, length)
			assert.Equal(t, 20.0, length)
		}
		// "res6" should have changed the provider
		if res.URN.Name() == "res6" {
			foundRes6 = true
			ref, err := providers.ParseReference(res.Provider)
			require.NoError(t, err)
			urn := ref.URN()
			assert.Equal(t, "provider2", urn.Name())
		}
		// "res7" should have changed the provider
		if res.URN.Name() == "res7" {
			foundRes7 = true
			// we change the provider but because this is a remote component resource it ends up empty in state.
			assert.Equal(t, "", res.Provider)
		}
		if res.URN.Name() == "res8" {
			foundRes8 = true
		}
	}
	assert.True(t, foundRes1)
	assert.True(t, foundRes2Child)
	assert.True(t, foundRes3)
	assert.True(t, foundRes4Child)
	assert.True(t, foundRes5)
	assert.True(t, foundRes6)
	assert.True(t, foundRes7)
	assert.True(t, foundRes8)
}
