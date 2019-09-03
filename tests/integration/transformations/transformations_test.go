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
					for _, res := range stack.Deployment.Resources {
						// "res1" has a transformation which adds additionalSecretOutputs
						if res.URN.Name() == "res1" {
							assert.Equal(t, res.Type, tokens.Type("pulumi-nodejs:dynamic:Resource"))
							assert.Contains(t, res.AdditionalSecretOutputs, resource.PropertyKey("output"))
						}
						// "res2" has a transformation which adds additionalSecretOutputs to it's
						// "child"
						if res.URN.Name() == "child" {
							assert.Equal(t, res.Type, tokens.Type("pulumi-nodejs:dynamic:Resource"))
							assert.Equal(t, res.Parent.Type(), tokens.Type("my:component:MyComponent"))
							assert.Contains(t, res.AdditionalSecretOutputs, resource.PropertyKey("output"))
						}
						// "res3" is impacted by a global stack transformation which sets
						// optionalDefault to "stackDefault"
						if res.URN.Name() == "res3" {
							assert.Equal(t, res.Type, tokens.Type("pulumi-nodejs:dynamic:Resource"))
							optionalInput := res.Inputs["optionalInput"]
							assert.NotNil(t, optionalInput)
							assert.Equal(t, optionalInput.(string), "stackDefault")
						}
					}
				},
			})
		})
	}
}
