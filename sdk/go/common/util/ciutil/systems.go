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

// CI system constants.
const (
	AppVeyor                    SystemName = "AppVeyor"
	AWSCodeBuild                SystemName = "AWS CodeBuild"
	AtlassianBamboo             SystemName = "Atlassian Bamboo"
	AtlassianBitbucketPipelines SystemName = "Atlassian Bitbucket Pipelines"
	AzurePipelines              SystemName = "Azure Pipelines"
	Buildkite                   SystemName = "Buildkite"
	CircleCI                    SystemName = "CircleCI"
	Codefresh                   SystemName = "Codefresh"
	Codeship                    SystemName = "Codeship"
	Drone                       SystemName = "Drone"

	// GenericCI is used when a CI system in which the CLI is being run,
	// is not recognized by it. Users can set the relevant env vars
	// as a fallback so that the CLI would still pick-up the metadata related
	// to their CI build.
	GenericCI SystemName = "Generic CI"

	GitHubActions SystemName = "GitHub Actions"
	GitLab        SystemName = "GitLab CI/CD"
	GoCD          SystemName = "GoCD"
	Hudson        SystemName = "Hudson"
	Jenkins       SystemName = "Jenkins"
	MagnumCI      SystemName = "Magnum CI"
	Semaphore     SystemName = "Semaphore"
	TaskCluster   SystemName = "TaskCluster"
	TeamCity      SystemName = "TeamCity"
	Travis        SystemName = "Travis CI"
)

// SystemName is a recognized CI system.
type SystemName string

// system represents a CI/CD system.
type system interface {
	// DetectVars when called on a specific instance of a CISystem
	// detects the env vars of the corresponding CI/CD system and
	// returns `Vars` with those values.
	DetectVars() Vars
	// IsCI returns true if any of the CI systems's associated environment variables are set.
	IsCI() bool
}

// Vars contains a set of metadata variables about a CI system.
type Vars struct {
	// Name is a required friendly name of the CI system.
	Name SystemName
	// BuildID is an optional unique identifier for the current build/job.
	// In some CI systems the build ID is a system-wide unique internal ID
	// and the `BuildNumber` is the repo/project-specific unique ID.
	BuildID string
	// BuildNumber is the unique identifier of a build within a project/repository.
	// This is only set for CI systems that expose both the internal ID, as well as
	// a project/repo-specific ID.
	BuildNumber string
	// BuildType is an optional friendly type name of the build/job type.
	BuildType string
	// BuildURL is an optional URL for this build/job's webpage.
	BuildURL string
	// SHA is the SHA hash of the code repo at which this build/job is running.
	SHA string
	// BranchName is the name of the feature branch currently being built.
	BranchName string
	// CommitMessage is the full message of the Git commit being built.
	CommitMessage string
	// PRNumber is the pull-request ID/number in the source control system.
	PRNumber string
}

// baseCI implements the `System` interface with default
// implementations.
//
// When creating a new CI System implementation, implement the
// DetectVars and any other function you wish to override.
type baseCI struct {
	Name SystemName
	// EnvVarsToDetect is an array of env vars to check if any of these env vars is set,
	// which would indicate that the Pulumi CLI is running in that CI system's environment.
	EnvVarsToDetect []string
	// EnvValuesToDetect is a map of env vars and their expected values to check for,
	// in order to see if the Pulumi CLI is running inside a certain CI system's environment.
	EnvValuesToDetect map[string]string
}

// DetectVars in the base implementation returns a Vars
// struct with just the Name property of the CI system.
func (d baseCI) DetectVars() Vars {
	return Vars{Name: d.Name}
}

// IsCI returns true if a specific env var of a CI system is set.
func (d baseCI) IsCI() bool {
	for _, e := range d.EnvVarsToDetect {
		if os.Getenv(e) != "" {
			return true
		}
	}

	for k, v := range d.EnvValuesToDetect {
		if os.Getenv(k) == v {
			return true
		}
	}

	return false
}
