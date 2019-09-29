// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package ints

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/pulumi/pulumi/pkg/tokens"
)

var dirs = []string{
	"simple",
}

// TestNodejsAliases tests a case where a resource's name changes but it provides an `alias`
// pointing to the old URN to ensure the resource is preserved across the update.
func TestNodejsAliases(t *testing.T) {
	for _, dir := range dirs {
		d := path.Join("nodejs", dir)
		t.Run(d, func(t *testing.T) {
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir:          d,
				Dependencies: []string{"@pulumi/pulumi"},
				Quick:        true,
				ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
					foundRes1 := false
					foundRes2Child := false
					foundRes3 := false
					foundRes4Child := false
					foundRes5Child := false
					for _, res := range stack.Deployment.Resources {
						// "res1" has a transformation which adds additionalSecretOutputs
						if res.URN.Name() == "res1" {
							foundRes1 = true
							assert.Equal(t, res.Type, tokens.Type("pulumi-nodejs:dynamic:Resource"))
							assert.Contains(t, res.AdditionalSecretOutputs, resource.PropertyKey("output"))
						}
						// "res2" has a transformation which adds additionalSecretOutputs to it's
						// "child"
						if res.URN.Name() == "res2-child" {
							foundRes2Child = true
							assert.Equal(t, res.Type, tokens.Type("pulumi-nodejs:dynamic:Resource"))
							assert.Equal(t, res.Parent.Type(), tokens.Type("my:component:MyComponent"))
							assert.Contains(t, res.AdditionalSecretOutputs, resource.PropertyKey("output"))
							assert.Contains(t, res.AdditionalSecretOutputs, resource.PropertyKey("output2"))
						}
						// "res3" is impacted by a global stack transformation which sets
						// optionalDefault to "stackDefault"
						if res.URN.Name() == "res3" {
							foundRes3 = true
							assert.Equal(t, res.Type, tokens.Type("pulumi-nodejs:dynamic:Resource"))
							optionalInput := res.Inputs["optionalInput"]
							assert.NotNil(t, optionalInput)
							assert.Equal(t, "stackDefault", optionalInput.(string))
						}
						// "res4" is impacted by both a global stack transformation which sets
						// optionalDefault to "stackDefault" and then two component parent
						// transformations which set optionalDefault to "default1" and then finally
						// "default2".  The end result should be "default2".
						if res.URN.Name() == "res4-child" {
							foundRes4Child = true
							assert.Equal(t, res.Type, tokens.Type("pulumi-nodejs:dynamic:Resource"))
							assert.Equal(t, res.Parent.Type(), tokens.Type("my:component:MyComponent"))
							optionalInput := res.Inputs["optionalInput"]
							assert.NotNil(t, optionalInput)
							assert.Equal(t, "default2", optionalInput.(string))
						}
						// "res5" modifies one of its children to depend on another of its children.
						if res.URN.Name() == "res5-child1" {
							foundRes5Child = true
							assert.Equal(t, res.Type, tokens.Type("pulumi-nodejs:dynamic:Resource"))
							assert.Equal(t, res.Parent.Type(), tokens.Type("my:component:MyComponent"))
							// TODO[pulumi/pulumi#3282] Due to this bug, the dependency information
							// will not be correctly recorded in the state file, and so cannot be
							// verified here.
							//
							// assert.Len(t, res.PropertyDependencies, 1)
							input := res.Inputs["input"]
							assert.NotNil(t, input)
							assert.Equal(t, "b", input.(string))
						}
					}
					assert.True(t, foundRes1)
					assert.True(t, foundRes2Child)
					assert.True(t, foundRes3)
					assert.True(t, foundRes4Child)
					assert.True(t, foundRes5Child)
				},
			})
		})
	}
}
