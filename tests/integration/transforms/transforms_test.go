// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.
//go:build !xplatform_acceptance

package ints

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
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
			assert.Equal(t, "stackDefault", optionalPrefix.(string))
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
			assert.Equal(t, "stackDefault", optionalPrefix.(string))
		}
		// "res5" should have mutated the length
		if res.URN.Name() == "res5" {
			foundRes5 = true
			assert.Equal(t, res.Type, tokens.Type(randomResName))
			length := res.Inputs["length"]
			assert.NotNil(t, length)
			assert.Equal(t, 20.0, length.(float64))
		}
	}
	assert.True(t, foundRes1)
	assert.True(t, foundRes2Child)
	assert.True(t, foundRes3)
	assert.True(t, foundRes4Child)
	assert.True(t, foundRes5)
}
