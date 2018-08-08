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

// Environment variaibles and their values from https://github.com/watson/ci-info/blob/master/index.js

type ciSystemDetector interface {
	IsCI() bool
}

// envVarDetector is a detector that uses the existence of a set of environment variables to determine if the system
// is a CI system.  All variaibles must be present.
type envVarDetector struct {
	envVar []string
}

func (c envVarDetector) IsCI() bool {
	for _, e := range c.envVar {
		if os.Getenv(e) == "" {
			return false
		}
	}

	return true
}

// envValueDetector is a detector that uses the existence of a set of environment variables to determine if the system
// is a CI system.  All variaibles must be present and their values must match expected data.
type envValueDetector struct {
	envMap map[string]string
}

func (c envValueDetector) IsCI() bool {
	for k, v := range c.envMap {
		if os.Getenv(k) != v {
			return false
		}
	}

	return true
}

var detectors = map[string]ciSystemDetector{
	"Travis CI":              envVarDetector{envVar: []string{"TRAVIS"}},
	"CircleCI":               envVarDetector{envVar: []string{"CIRCLECI"}},
	"GitLab CI":              envVarDetector{envVar: []string{"GITLAB_CI"}},
	"AppVeyor":               envVarDetector{envVar: []string{"APPVEYOR"}},
	"Drone":                  envVarDetector{envVar: []string{"DRONE"}},
	"Magnum CI":              envVarDetector{envVar: []string{"MAGNUM"}},
	"Semaphore":              envVarDetector{envVar: []string{"SEMAPHORE"}},
	"Jenkins":                envVarDetector{envVar: []string{"JENKINS_URL", "BUILD_ID"}},
	"Bamboo":                 envVarDetector{envVar: []string{"bamboo_planKey"}},
	"Team Foundation Server": envVarDetector{envVar: []string{"TF_BUILD"}},
	"TeamCity":               envVarDetector{envVar: []string{"TEAMCITY_VERSION"}},
	"Buildkite":              envVarDetector{envVar: []string{"BUILDKITE"}},
	"Hudson":                 envVarDetector{envVar: []string{"HUDSON_URL"}},
	"TaskCluster":            envVarDetector{envVar: []string{"TASK_ID", "RUN_ID"}},
	"GoCD":                   envVarDetector{envVar: []string{"GO_PIPELINE_LABEL"}},
	"Bitbucket Pipelines":    envVarDetector{envVar: []string{"BITBUCKET_COMMIT"}},
	"AWS CodeBuild":          envVarDetector{envVar: []string{"CODEBUILD_BUILD_ARN"}},
	"CODESHIP":               envValueDetector{envMap: map[string]string{"CI_NAME": "codeship"}},
}

// IsCI returns true when the current system looks like a CI system. Detection is based on environment variables
// that CI vendors we know about set.
func IsCI() bool {
	for _, v := range detectors {
		if v.IsCI() {
			return true
		}
	}

	return false
}
