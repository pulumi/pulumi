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

// CI system constants.
const (
	AppVeyor                    System = "AppVeyor"
	AWSCodeBuild                System = "AWS CodeBuild"
	AtlassianBamboo             System = "Atlassian Bamboo"
	AtlassianBitbucketPipelines System = "Atlassian Bitbucket Pipelines"
	AzurePipelines              System = "Azure Pipelines"
	Buildkite                   System = "Buildkite"
	CircleCI                    System = "CircleCI"
	Codeship                    System = "Codeship"
	Drone                       System = "Drone"
	GitHub                      System = "GitHub"
	GitLab                      System = "GitLab"
	GoCD                        System = "GoCD"
	Hudson                      System = "Hudson"
	Jenkins                     System = "Jenkins"
	MagnumCI                    System = "Magnum CI"
	PulumiCI                    System = "Pulumi CI"
	Semaphore                   System = "Semaphore"
	TaskCluster                 System = "TaskCluster"
	TeamCity                    System = "TeamCity"
	Travis                      System = "Travis CI"
)

// System is a recognized CI system.
type System string
