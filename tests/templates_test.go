// Copyright 2016-2019, Pulumi Corporation.
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
	"fmt"
	"os"
	"strings"
	"testing"

	ptesting "github.com/pulumi/pulumi/pkg/testing"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

func TestTemplates(t *testing.T) {
	awsRegion := os.Getenv("AWS_REGION")
	if awsRegion == "" {
		awsRegion = "us-west-1"
		fmt.Println("Defaulting AWS_REGION to 'us-west-1'.  You can override using the AWS_REGION environment variable")
	}
	azureEnviron := os.Getenv("ARM_ENVIRONMENT")
	if azureEnviron == "" {
		azureEnviron = "public"
		fmt.Println("Defaulting ARM_ENVIRONMENT to 'public'.  You can override using the ARM_ENVIRONMENT variable")
	}
	azureLocation := os.Getenv("ARM_LOCATION")
	if azureLocation == "" {
		azureLocation = "westus"
		fmt.Println("Defaulting ARM_LOCATION to 'westus'.  You can override using the ARM_LOCATION variable")
	}
	gcpProject := os.Getenv("GOOGLE_PROJECT")
	if gcpProject == "" {
		gcpProject = "pulumi-ci-gcp-provider"
		fmt.Println("Defaulting GOOGLE_PROJECT to 'pulumi-ci-gcp-provider'." +
			"You can override using the GOOGLE_PROJECT variable")
	}
	gcpRegion := os.Getenv("GOOGLE_REGION")
	if gcpRegion == "" {
		gcpRegion = "us-central1"
		fmt.Println("Defaulting GOOGLE_REGION to 'us-central1'.  You can override using the GOOGLE_REGION variable")
	}
	gcpZone := os.Getenv("GOOGLE_ZONE")
	if gcpZone == "" {
		gcpZone = "us-central1-a"
		fmt.Println("Defaulting GOOGLE_ZONE to 'us-central1-a'.  You can override using the GOOGLE_ZONE variable")
	}
	overrides, err := integration.DecodeMapString(os.Getenv("PULUMI_TEST_NODE_OVERRIDES"))
	if !assert.NoError(t, err, "expected valid override map: %v", err) {
		return
	}

	base := integration.ProgramTestOptions{
		Tracing:              "https://tracing.pulumi-engineering.com/collector/api/v1/spans",
		ExpectRefreshChanges: true,
		Overrides:            overrides,
		Quick:                true,
		SkipRefresh:          true,
	}

	// Retrieve the template repo.
	repo, err := workspace.RetrieveTemplates("", false /*offline*/)
	assert.NoError(t, err)
	defer assert.NoError(t, repo.Delete())

	// List the templates from the repo.
	templates, err := repo.Templates()
	assert.NoError(t, err)

	for _, template := range templates {
		// Skip packet tests for now
		if strings.Contains(template.Name, "packet") {
			continue
		}
		// Skip go tests for now
		if strings.Contains(template.Name, "go") {
			continue
		}

		t.Run(template.Name, func(t *testing.T) {
			e := ptesting.NewEnvironment(t)
			defer deleteIfNotFailed(e)

			e.RunCommand("pulumi", "new", template.Name, "-f")

			path, err := workspace.DetectProjectPathFrom(e.RootPath)
			assert.NoError(t, err)
			_, err = workspace.LoadProject(path)
			assert.NoError(t, err)

			example := base.With(integration.ProgramTestOptions{
				Dir: e.RootPath,
				Config: map[string]string{
					"aws:region":        awsRegion,
					"azure:environment": azureEnviron,
					"azure:location":    azureLocation,
					"gcp:project":       gcpProject,
					"gcp:region":        gcpRegion,
					"gcp:zone":          gcpZone,
					"cloud:provider":    "aws",
				},
			})

			integration.ProgramTest(t, &example)
		})
	}
}
