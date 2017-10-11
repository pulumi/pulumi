// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package examples

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/testing/integration"
)

func TestExamples(t *testing.T) {
	cwd, err := os.Getwd()
	if !assert.NoError(t, err, "expected a valid working directory: %v", err) {
		return
	}
	examples := []integration.LumiProgramTestOptions{
		{
			Dir:          path.Join(cwd, "basic/minimal"),
			Dependencies: []string{"pulumi"},
			Config: map[string]string{
				"minimal:config:name": "Pulumi",
			},
		},
		{
			Dir:          path.Join(cwd, "test-provider/simple"),
			Dependencies: []string{"pulumi"},
			Config: map[string]string{
				"testing:providers:module": "./bin/providers.js",
				"simple:config:w": "1",
				"simple:config:x": "1",
				"simple:config:y": "1",
			},
		},
	}
	for _, ex := range examples {
		example := ex
		t.Run(example.Dir, func(t *testing.T) {
			integration.LumiProgramTest(t, example)
		})
	}
}
