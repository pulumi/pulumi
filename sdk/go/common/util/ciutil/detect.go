// Copyright 2016-2025, Pulumi Corporation.
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
//
// For any CI system for which we detect additional env vars, the type of `System` is specific to that CI system. The
// rest, even though we detect if it is that CI system, may not have an implementation that detects all useful env vars,
// and hence just uses the `baseCI` struct type.
var detectors = []system{
	// GenericCI picks up a set of environment variables that users may define explicitly when using a CI system that we
	// would not otherwise detect. It is deliberately placed first in this list so that it takes precedence over any other
	// CI system that we may detect.
	genericCICI{
		baseCI: baseCI{
			Name:            SystemName(os.Getenv("PULUMI_CI_SYSTEM")),
			EnvVarsToDetect: []string{"PULUMI_CI_SYSTEM"},
		},
	},

	// Supported CI systems, in alphabetical order. We expect (rather, support) exactly one of these matching, so their
	// order should not really matter.
	baseCI{
		Name:            AppVeyor,
		EnvVarsToDetect: []string{"APPVEYOR"},
	},
	baseCI{
		Name:            AWSCodeBuild,
		EnvVarsToDetect: []string{"CODEBUILD_BUILD_ARN"},
	},
	baseCI{
		Name:            AtlassianBamboo,
		EnvVarsToDetect: []string{"bamboo_planKey"},
	},
	bitbucketPipelinesCI{
		baseCI: baseCI{
			Name:            AtlassianBitbucketPipelines,
			EnvVarsToDetect: []string{"BITBUCKET_COMMIT"},
		},
	},
	azurePipelinesCI{
		baseCI: baseCI{
			Name:            AzurePipelines,
			EnvVarsToDetect: []string{"TF_BUILD"},
		},
	},
	buildkiteCI{
		baseCI: baseCI{
			Name:            Buildkite,
			EnvVarsToDetect: []string{"BUILDKITE"},
		},
	},
	circleCICI{
		baseCI: baseCI{
			Name:            CircleCI,
			EnvVarsToDetect: []string{"CIRCLECI"},
		},
	},
	codefreshCI{
		baseCI: baseCI{
			Name:            Codefresh,
			EnvVarsToDetect: []string{"CF_BUILD_URL"},
		},
	},
	baseCI{
		Name:              Codeship,
		EnvValuesToDetect: map[string]string{"CI_NAME": "codeship"},
	},
	baseCI{
		Name:            Drone,
		EnvVarsToDetect: []string{"DRONE"},
	},

	githubActionsCI{
		baseCI{
			Name:            GitHubActions,
			EnvVarsToDetect: []string{"GITHUB_ACTIONS"},
		},
	},
	gitlabCI{
		baseCI: baseCI{
			Name:            GitLab,
			EnvVarsToDetect: []string{"GITLAB_CI"},
		},
	},
	baseCI{
		Name:            GoCD,
		EnvVarsToDetect: []string{"GO_PIPELINE_LABEL"},
	},
	baseCI{
		Name:            Hudson,
		EnvVarsToDetect: []string{"HUDSON_URL"},
	},
	jenkinsCI{
		baseCI: baseCI{
			Name:            Jenkins,
			EnvVarsToDetect: []string{"JENKINS_URL"},
		},
	},
	baseCI{
		Name:            MagnumCI,
		EnvVarsToDetect: []string{"MAGNUM"},
	},
	baseCI{
		Name:            Semaphore,
		EnvVarsToDetect: []string{"SEMAPHORE"},
	},
	baseCI{
		Name: Spacelift,
		EnvVarsToDetect: []string{
			"SPACELIFT_MAX_REQUESTS_BURST", "TF_VAR_spacelift_run_trigger", "SPACELIFT_STORE_HOOKS_ENV_VARS",
			"TF_VAR_spacelift_commit_branch", "SPACELIFT_WORKER_TRACING_ENABLED",
		},
	},
	baseCI{
		Name:            TaskCluster,
		EnvVarsToDetect: []string{"TASK_ID", "RUN_ID"},
	},
	baseCI{
		Name:            TeamCity,
		EnvVarsToDetect: []string{"TEAMCITY_VERSION"},
	},
	travisCI{
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
