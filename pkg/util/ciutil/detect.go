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

	"github.com/pulumi/pulumi/pkg/util/ciutil/systems"
)

// detectors contains environment variable names and their values, if applicable, for detecting when we're running in
// CI. See https://github.com/watson/ci-info/blob/master/index.js
var detectors = map[systems.SystemName]systems.System{
	systems.AppVeyor: systems.BaseCISystem{
		Name:            systems.AppVeyor,
		EnvVarsToDetect: []string{"APPVEYOR"},
	},
	systems.AWSCodeBuild: systems.BaseCISystem{
		Name:            systems.AWSCodeBuild,
		EnvVarsToDetect: []string{"CODEBUILD_BUILD_ARN"},
	},
	systems.AtlassianBamboo: systems.BaseCISystem{
		Name:            systems.AtlassianBamboo,
		EnvVarsToDetect: []string{"bamboo_planKey"},
	},
	systems.AtlassianBitbucketPipelines: systems.BitbucketPipelinesCISystem{
		BaseCISystem: systems.BaseCISystem{
			Name:            systems.AtlassianBitbucketPipelines,
			EnvVarsToDetect: []string{"BITBUCKET_COMMIT"},
		},
	},
	systems.AzurePipelines: systems.AzurePipelinesCISystem{
		BaseCISystem: systems.BaseCISystem{
			Name:            systems.AzurePipelines,
			EnvVarsToDetect: []string{"TF_BUILD"},
		},
	},
	systems.Buildkite: systems.BaseCISystem{
		Name:            systems.Buildkite,
		EnvVarsToDetect: []string{"BUILDKITE"},
	},
	systems.CircleCI: systems.CircleCICISystem{
		BaseCISystem: systems.BaseCISystem{
			Name:            systems.CircleCI,
			EnvVarsToDetect: []string{"CIRCLECI"},
		},
	},
	systems.Codeship: systems.BaseCISystem{
		Name:              systems.Codeship,
		EnvValuesToDetect: map[string]string{"CI_NAME": "codeship"},
	},
	systems.Drone: systems.BaseCISystem{
		Name:            systems.Drone,
		EnvVarsToDetect: []string{"DRONE"},
	},

	// GenericCI is used when a CI system in which the CLI is being run,
	// is not recognized by it. Users can set the relevant env vars
	// as a fallback so that the CLI would still pick-up the metadata related
	// to their CI build.
	systems.GenericCI: systems.GenericCISystem{
		BaseCISystem: systems.BaseCISystem{
			Name:            systems.SystemName(os.Getenv("PULUMI_CI_SYSTEM")),
			EnvVarsToDetect: []string{"GENERIC_CI_SYSTEM"},
		},
	},

	systems.GitHub: systems.BaseCISystem{
		Name:            systems.GitHub,
		EnvVarsToDetect: []string{"GITHUB_WORKFLOW"},
	},
	systems.GitLab: systems.GitLabCISystem{
		BaseCISystem: systems.BaseCISystem{
			Name:            systems.GitLab,
			EnvVarsToDetect: []string{"GITLAB_CI"},
		},
	},
	systems.GoCD: systems.BaseCISystem{
		Name:            systems.GoCD,
		EnvVarsToDetect: []string{"GO_PIPELINE_LABEL"},
	},
	systems.Hudson: systems.BaseCISystem{
		Name:            systems.Hudson,
		EnvVarsToDetect: []string{"HUDSON_URL"},
	},
	systems.Jenkins: systems.BaseCISystem{
		Name:            systems.Jenkins,
		EnvVarsToDetect: []string{"JENKINS_URL", "BUILD_ID"},
	},
	systems.MagnumCI: systems.BaseCISystem{
		Name:            systems.MagnumCI,
		EnvVarsToDetect: []string{"MAGNUM"},
	},
	systems.Semaphore: systems.BaseCISystem{
		Name:            systems.Semaphore,
		EnvVarsToDetect: []string{"SEMAPHORE"},
	},
	systems.TaskCluster: systems.BaseCISystem{
		Name:            systems.TaskCluster,
		EnvVarsToDetect: []string{"TASK_ID", "RUN_ID"},
	},
	systems.TeamCity: systems.BaseCISystem{
		Name:            systems.TeamCity,
		EnvVarsToDetect: []string{"TEAMCITY_VERSION"},
	},
	systems.Travis: systems.TravisCISystem{
		BaseCISystem: systems.BaseCISystem{
			Name:            systems.Travis,
			EnvVarsToDetect: []string{"TRAVIS"},
		},
	},
}

// IsCI returns true if we are running in a known CI system.
func IsCI() bool {
	return DetectSystem() != nil
}

// DetectSystem returns a CI system name when the current system looks like a CI system. Detection is based on
// environment variables that CI vendors we know about set.
func DetectSystem() systems.System {
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
