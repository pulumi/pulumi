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
// CI. See https://github.com/watson/ci-info/blob/master/index.js.
// For any CI system for which we detect additional env vars, the type of `System` is that is
// specific to that CI system. The rest, even though we detect if it is that CI system, may not have an
// implementation that detects all useful env vars, and hence just uses the `baseCI` struct type.
var detectors = map[SystemName]system{
	AppVeyor: baseCI{
		Name:            AppVeyor,
		EnvVarsToDetect: []string{"APPVEYOR"},
	},
	AWSCodeBuild: baseCI{
		Name:            AWSCodeBuild,
		EnvVarsToDetect: []string{"CODEBUILD_BUILD_ARN"},
	},
	AtlassianBamboo: baseCI{
		Name:            AtlassianBamboo,
		EnvVarsToDetect: []string{"bamboo_planKey"},
	},
	AtlassianBitbucketPipelines: bitbucketPipelinesCI{
		baseCI: baseCI{
			Name:            AtlassianBitbucketPipelines,
			EnvVarsToDetect: []string{"BITBUCKET_COMMIT"},
		},
	},
	AzurePipelines: azurePipelinesCI{
		baseCI: baseCI{
			Name:            AzurePipelines,
			EnvVarsToDetect: []string{"TF_BUILD"},
		},
	},
	Buildkite: baseCI{
		Name:            Buildkite,
		EnvVarsToDetect: []string{"BUILDKITE"},
	},
	CircleCI: circleCICI{
		baseCI: baseCI{
			Name:            CircleCI,
			EnvVarsToDetect: []string{"CIRCLECI"},
		},
	},
	Codefresh: codefreshCI{
		baseCI: baseCI{
			Name:            Codefresh,
			EnvVarsToDetect: []string{"CF_BUILD_URL"},
		},
	},
	Codeship: baseCI{
		Name:              Codeship,
		EnvValuesToDetect: map[string]string{"CI_NAME": "codeship"},
	},
	Drone: baseCI{
		Name:            Drone,
		EnvVarsToDetect: []string{"DRONE"},
	},

	// GenericCI is used when a CI system in which the CLI is being run,
	// is not recognized by it. Users can set the relevant env vars
	// as a fallback so that the CLI would still pick-up the metadata related
	// to their CI build.
	GenericCI: genericCICI{
		baseCI: baseCI{
			Name:            SystemName(os.Getenv("PULUMI_CI_SYSTEM")),
			EnvVarsToDetect: []string{"PULUMI_CI_SYSTEM"},
		},
	},

	GitHubActions: githubActionsCI{
		baseCI{
			Name:            GitHubActions,
			EnvVarsToDetect: []string{"GITHUB_ACTIONS"},
		},
	},
	GitLab: gitlabCI{
		baseCI: baseCI{
			Name:            GitLab,
			EnvVarsToDetect: []string{"GITLAB_CI"},
		},
	},
	GoCD: baseCI{
		Name:            GoCD,
		EnvVarsToDetect: []string{"GO_PIPELINE_LABEL"},
	},
	Hudson: baseCI{
		Name:            Hudson,
		EnvVarsToDetect: []string{"HUDSON_URL"},
	},
	Jenkins: jenkinsCI{
		baseCI: baseCI{
			Name:            Jenkins,
			EnvVarsToDetect: []string{"JENKINS_URL"},
		},
	},
	MagnumCI: baseCI{
		Name:            MagnumCI,
		EnvVarsToDetect: []string{"MAGNUM"},
	},
	Semaphore: baseCI{
		Name:            Semaphore,
		EnvVarsToDetect: []string{"SEMAPHORE"},
	},
	TaskCluster: baseCI{
		Name:            TaskCluster,
		EnvVarsToDetect: []string{"TASK_ID", "RUN_ID"},
	},
	TeamCity: baseCI{
		Name:            TeamCity,
		EnvVarsToDetect: []string{"TEAMCITY_VERSION"},
	},
	Travis: travisCI{
		baseCI: baseCI{
			Name:            Travis,
			EnvVarsToDetect: []string{"TRAVIS"},
		},
	},
}

// IsCI returns true if we are running in a known CI system.
func IsCI() bool {
	return detectSystem() != nil
}

// detectSystem returns a CI system name when the current system looks like a CI system.
// Detection is based on environment variables that CI vendors, we know about, set.
func detectSystem() system {
	// Provide a way to disable CI/CD detection, as it can interfere with the ability to test.
	if os.Getenv("PULUMI_DISABLE_CI_DETECTION") != "" {
		return nil
	}

	for _, system := range detectors {
		if system.IsCI() {
			return system
		}
	}
	return nil
}
