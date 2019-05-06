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
package ciutil

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestDetectVars(t *testing.T) {
	buildID := "123"
	systemAndEnvVars := map[System]map[string]string{
		AzurePipelines: {
			"TF_BUILD":      "true",
			"BUILD_BUILDID": buildID,
		},
		CircleCI: {
			"CIRCLECI":         "true",
			"CIRCLE_BUILD_NUM": buildID,
		},
		GitLab: {
			"GITLAB_CI": "true",
			"CI_JOB_ID": buildID,
		},
		Travis: {
			"TRAVIS":        "true",
			"TRAVIS_JOB_ID": buildID,
		},
	}

	for system := range systemAndEnvVars {
		t.Run(fmt.Sprintf("Test_%v_Detection", system), func(t *testing.T) {
			envVars := systemAndEnvVars[system]
			for envVar := range envVars {
				os.Setenv(envVar, envVars[envVar])
			}
			vars := DetectVars()
			assert.Equal(t,
				buildID, vars.BuildID,
				"%v did not set the expected build ID %v in the Vars struct.", system, buildID)
			for envVar := range envVars {
				os.Setenv(envVar, "")
			}
		})
	}
}

func TestDetectVarsDisableCIDetection(t *testing.T) {
	os.Setenv("PULUMI_DISABLE_CI_DETECTION", "nonEmptyString")
	os.Setenv("TRAVIS", "true")
	os.Setenv("TRAVIS_JOB_ID", "1234")

	v := DetectVars()
	assert.Equal(t, "", v.BuildID)

	os.Setenv("PULUMI_DISABLE_CI_DETECTION", "")
	os.Setenv("TRAVIS", "")
	os.Setenv("TRAVIS_JOB_ID", "")
}
