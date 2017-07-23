// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package examples

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/lumi/pkg/integrationtesting"
)

func TestExamples(t *testing.T) {
	cwd, err := os.Getwd()
	if !assert.NoError(t, err, "expected a valid working directory: %v", err) {
		return
	}
	examples := []string{
		path.Join(cwd, "basic/minimal"),
	}
	options := integrationtesting.LumiProgramTestOptions{
		Dependencies: []string{
			"@lumi/lumirt",
			"@lumi/lumi",
		},
	}
	for _, ex := range examples {
		example := ex
		t.Run(example, func(t *testing.T) {
			integrationtesting.LumiProgramTest(t, example, options)
		})
	}
}
