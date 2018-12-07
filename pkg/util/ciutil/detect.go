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

package ciutil

import (
	"os"
)

// detectors contains environment variable names and their values, if applicable, for detecting when we're running in
// CI. See https://github.com/watson/ci-info/blob/master/index.js
var detectors = map[System]detector{
	AppVeyor:                    envVarDetector{envVar: []string{"APPVEYOR"}},
	AzurePipelines:              envVarDetector{envVar: []string{"System.TeamProjectId"}},
	AWSCodeBuild:                envVarDetector{envVar: []string{"CODEBUILD_BUILD_ARN"}},
	AtlassianBamboo:             envVarDetector{envVar: []string{"bamboo_planKey"}},
	AtlassianBitbucketPipelines: envVarDetector{envVar: []string{"BITBUCKET_COMMIT"}},
	Buildkite:                   envVarDetector{envVar: []string{"BUILDKITE"}},
	CircleCI:                    envVarDetector{envVar: []string{"CIRCLECI"}},
	Codeship:                    envValueDetector{envMap: map[string]string{"CI_NAME": "codeship"}},
	Drone:                       envVarDetector{envVar: []string{"DRONE"}},
	GitHub:                      envVarDetector{envVar: []string{"GITHUB_WORKFLOW"}},
	GitLab:                      envVarDetector{envVar: []string{"GITLAB_CI"}},
	GoCD:                        envVarDetector{envVar: []string{"GO_PIPELINE_LABEL"}},
	Hudson:                      envVarDetector{envVar: []string{"HUDSON_URL"}},
	Jenkins:                     envVarDetector{envVar: []string{"JENKINS_URL", "BUILD_ID"}},
	MagnumCI:                    envVarDetector{envVar: []string{"MAGNUM"}},
	MicrosoftTFS:                envVarDetector{envVar: []string{"TF_BUILD"}},
	Semaphore:                   envVarDetector{envVar: []string{"SEMAPHORE"}},
	TaskCluster:                 envVarDetector{envVar: []string{"TASK_ID", "RUN_ID"}},
	TeamCity:                    envVarDetector{envVar: []string{"TEAMCITY_VERSION"}},
	Travis:                      envVarDetector{envVar: []string{"TRAVIS"}},
}

// detector detects whether we're running in a particular CI system.
type detector interface {
	// IsCI returns true if we are currently running in this particular CI system.
	IsCI() bool
}

// envVarDetector is a detector that uses the existence of a set of environment variables to determine if the system
// is a CI system. All variaibles must be present.
type envVarDetector struct {
	envVar []string
}

// IsCI returns true if any of the detector's associated environment variables are set.
func (c envVarDetector) IsCI() bool {
	for _, e := range c.envVar {
		if os.Getenv(e) == "" {
			return false
		}
	}
	return true
}

// envValueDetector is a detector that uses the existence of a set of environment variables to determine if the system
// is a CI system. All variaibles must be present and their values must match expected data.
type envValueDetector struct {
	envMap map[string]string
}

// IsCI returns true if the detector's required environment variables in the underlying map all match.
func (c envValueDetector) IsCI() bool {
	for k, v := range c.envMap {
		if os.Getenv(k) != v {
			return false
		}
	}
	return true
}

// IsCI returns true if we are running in a known CI system.
func IsCI() bool {
	return DetectSystem() != ""
}

// DetectSystem returns a CI system name when the current system looks like a CI system. Detection is based on
// environment variables that CI vendors we know about set.
func DetectSystem() System {
	// Provide a way to disable CI/CD detection, as it can interfere with the ability to test.
	if os.Getenv("PULUMI_DISABLE_CI_DETECTION") != "" {
		return ""
	}

	for sys, d := range detectors {
		if d.IsCI() {
			return sys
		}
	}
	return ""
}
