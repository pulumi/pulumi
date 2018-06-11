// Copyright 2016-2018, Pulumi Corporation.
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

package testutil

import (
	"os"
)

// Forked from https://github.com/watson/ci-info/blob/master/index.js
// If a suitable go package for this can be found, it would be great to move to that.

type vendor struct {
	name                    string
	requiredEnvNames        []string
	requiredEnvNameAndValue map[string]string
}

func makeVendor(name string, requiredEnvNames []string, requiredEnvNameAndValue map[string]string) vendor {
	return vendor{name: name, requiredEnvNames: requiredEnvNames, requiredEnvNameAndValue: requiredEnvNameAndValue}
}

var vendors = []vendor{
	// Constant, Name, Envs
	makeVendor("Travis CI", []string{"TRAVIS"}, nil),
	makeVendor("CircleCI", []string{"CIRCLECI"}, nil),
	makeVendor("GitLab CI", []string{"GITLAB_CI"}, nil),
	makeVendor("AppVeyor", []string{"APPVEYOR"}, nil),
	makeVendor("CODESHIP", []string{"Codeship"}, map[string]string{"CI_NAME": "codeship"}),
	makeVendor("Drone", []string{"DRONE"}, nil),
	makeVendor("Magnum CI", []string{"MAGNUM"}, nil),
	makeVendor("Semaphore", []string{"SEMAPHORE"}, nil),
	makeVendor("Jenkins", []string{"JENKINS_URL", "BUILD_ID"}, nil),
	makeVendor("Bamboo", []string{"bamboo_planKey"}, nil),
	makeVendor("Team Foundation Server", []string{"TF_BUILD"}, nil),
	makeVendor("TeamCity", []string{"TEAMCITY_VERSION"}, nil),
	makeVendor("Buildkite", []string{"BUILDKITE"}, nil),
	makeVendor("Hudson", []string{"HUDSON_URL"}, nil),
	makeVendor("TaskCluster", []string{"TASK_ID", "RUN_ID"}, nil),
	makeVendor("GoCD", []string{"GO_PIPELINE_LABEL"}, nil),
	makeVendor("Bitbucket Pipelines", []string{"BITBUCKET_COMMIT"}, nil),
	makeVendor("AWS CodeBuild", []string{"CODEBUILD_BUILD_ARN"}, nil),
}

func IsCI() bool {
	for _, v := range vendors {
		if v.isCI() {
			return true
		}
	}

	return false
}

func (vendor vendor) isCI() bool {
	for _, n := range vendor.requiredEnvNames {
		e := os.Getenv(n)
		if e == "" {
			return false
		}
	}

	if vendor.requiredEnvNameAndValue != nil {
		for k, v := range vendor.requiredEnvNameAndValue {
			e := os.Getenv(k)
			if e == "" || e != v {
				return false
			}
		}
	}

	return true
}
