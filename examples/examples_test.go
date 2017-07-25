// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package examples

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/lumi/pkg/integrationtesting"
)

func Test_Examples(t *testing.T) {
	var examples []string
	region := os.Getenv("AWS_REGION")
	if region == "" {
		t.Skipf("Skipping test due to missing AWS_REGION environment variable")
	}
	cwd, err := os.Getwd()
	if !assert.NoError(t, err, "expected a valid working directory: %v", err) {
		return
	}
	if testing.Short() {
		examples = []string{
			path.Join(cwd, "scenarios", "aws", "serverless"),
		}
	} else {
		examples = []string{
			path.Join(cwd, "scenarios", "aws", "serverless-raw"),
			path.Join(cwd, "scenarios", "aws", "serverless"),
			path.Join(cwd, "scenarios", "aws", "webserver"),
			path.Join(cwd, "scenarios", "aws", "webserver-comp"),
			path.Join(cwd, "scenarios", "aws", "beanstalk"),
			path.Join(cwd, "scenarios", "aws", "minimal"),
		}
	}
	options := integrationtesting.LumiProgramTestOptions{
		Config: map[string]string{
			"aws:config:region": region,
		},
		Dependencies: []string{
			"@lumi/lumirt",
			"@lumi/lumi",
			"@lumi/aws",
		},
	}
	for _, ex := range examples {
		example := ex
		t.Run(example, func(t *testing.T) {
			integrationtesting.LumiProgramTest(t, example, options)
		})
	}
}
