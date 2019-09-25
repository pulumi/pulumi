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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectVars(t *testing.T) {
	buildID := "123"
	systemAndEnvVars := map[SystemName]map[string]string{
		// Since the `pulumi/pulumi` repo runs on Travis,
		// we set the TRAVIS env var to an empty string for all test cases
		// except for the Travis one itself.
		// This way when the unit test runs on Travis, we don't pick-up Travis env vars.
		AzurePipelines: {
			"TRAVIS":        "",
			"TF_BUILD":      "true",
			"BUILD_BUILDID": buildID,
		},
		CircleCI: {
			"TRAVIS":           "",
			"CIRCLECI":         "true",
			"CIRCLE_BUILD_NUM": buildID,
		},
		Codefresh: {
			"TRAVIS":       "",
			"CF_BUILD_URL": "https://g.codefresh.io/build/99f5d825577e23c56f8c6b2a",
			"CF_BUILD_ID":  buildID,
		},
		GenericCI: {
			"TRAVIS":             "",
			"GENERIC_CI_SYSTEM":  "true",
			"PULUMI_CI_BUILD_ID": buildID,
		},
		GitLab: {
			"TRAVIS":    "",
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
			originalEnvVars := make(map[string]string)
			for envVar := range envVars {
				// Save the original env value
				if value, isSet := os.LookupEnv(envVar); isSet {
					originalEnvVars[envVar] = value
				}

				os.Setenv(envVar, envVars[envVar])
			}
			vars := DetectVars()
			assert.Equal(t,
				buildID, vars.BuildID,
				"%v did not set the expected build ID %v in the Vars struct.", system, buildID)

			// Restore any modified env vars back to their original value
			// if we previously saved it. Otherwise, just unset it.
			for envVar := range envVars {
				if val, ok := originalEnvVars[envVar]; ok {
					os.Setenv(envVar, val)
				} else {
					os.Unsetenv(envVar)
				}
			}
		})
	}
}

func TestDetectVarsBaseCI(t *testing.T) {
	systemAndEnvVars := map[SystemName]map[string]string{
		// Since the `pulumi/pulumi` repo runs on Travis,
		// we set the TRAVIS env var to an empty string for all test cases
		// except for the Travis one itself.
		// This way when the unit test runs on Travis, we don't pick-up Travis env vars.
		AppVeyor: {
			"TRAVIS":   "",
			"APPVEYOR": "true",
		},
		Codeship: {
			"TRAVIS":  "",
			"CI_NAME": "codeship",
		},
	}

	for system := range systemAndEnvVars {
		t.Run(fmt.Sprintf("Test_%v_Detection", system), func(t *testing.T) {
			envVars := systemAndEnvVars[system]
			originalEnvVars := make(map[string]string)
			for envVar := range envVars {
				// Save the original env value
				if value, isSet := os.LookupEnv(envVar); isSet {
					originalEnvVars[envVar] = value
				}

				os.Setenv(envVar, envVars[envVar])
			}
			vars := DetectVars()
			assert.Equal(t,
				string(system), string(vars.Name),
				"%v did not set the expected CI system name in the Vars struct.", system)

			// Restore any modified env vars back to their original value
			// if we previously saved it. Otherwise, just unset it.
			for envVar := range envVars {
				if val, ok := originalEnvVars[envVar]; ok {
					os.Setenv(envVar, val)
				} else {
					os.Unsetenv(envVar)
				}
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
