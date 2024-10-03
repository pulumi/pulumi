// Copyright 2016-2022, Pulumi Corporation.
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

package apitype

import (
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// PulumiOperation describes what operation to perform on the
// stack as defined in the Job spec.
type PulumiOperation string

// The possible operations we can deploy.
const (
	Update         PulumiOperation = "update"
	Preview        PulumiOperation = "preview"
	Destroy        PulumiOperation = "destroy"
	Refresh        PulumiOperation = "refresh"
	DetectDrift    PulumiOperation = "detect-drift"
	RemediateDrift PulumiOperation = "remediate-drift"
)

func ParsePulumiOperation(o string) (PulumiOperation, error) {
	switch o {
	case "update":
		return Update, nil
	case "preview":
		return Preview, nil
	case "destroy":
		return Destroy, nil
	case "refresh":
		return Refresh, nil
	case "detect-drift":
		return DetectDrift, nil
	case "remediate-drift":
		return RemediateDrift, nil
	default:
		return "", fmt.Errorf("invalid pulumi operation; %q", o)
	}
}

// DeploymentDuration, wrapper over time.Duration to properly marshall
// time durations according to pulumi cloud spec.
type DeploymentDuration time.Duration

func (v DeploymentDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(v).String())
}

func (v *DeploymentDuration) UnmarshalJSON(bytes []byte) error {
	var s string
	if err := json.Unmarshal(bytes, &s); err != nil {
		return err
	}
	if s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			return err
		}
		*v = DeploymentDuration(d)
	}
	return nil
}

func (v DeploymentDuration) MarshalYAML() (any, error) {
	return time.Duration(v).String(), nil
}

func (v *DeploymentDuration) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return err
	}
	if s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			return err
		}
		*v = DeploymentDuration(d)
	}
	return nil
}

// CreateDeploymentRequest defines the request payload that is expected when
// creating a new deployment.
type CreateDeploymentRequest struct {
	// Op
	Op PulumiOperation `json:"operation"`

	// InheritSettings is a flag that indicates whether the deployment should inherit
	// deployment settings from the stack.
	InheritSettings bool `json:"inheritSettings"`

	// Executor defines options that the executor is going to use to run the job.
	Executor *ExecutorContext `json:"executorContext,omitempty"`

	// Source defines how the source code to the Pulumi program will be gathered.
	Source *SourceContext `json:"sourceContext,omitempty"`

	// Operation defines the options that the executor will use to run the Pulumi commands.
	Operation *OperationContext `json:"operationContext,omitempty"`

	AgentPoolID *AgentPoolIDMarshaller `json:"agentPoolID,omitempty"`
}

type AgentPoolIDMarshaller string

func (d *AgentPoolIDMarshaller) MarshalJSON() ([]byte, error) {
	if *d != "" {
		return json.Marshal(*d)
	}
	return json.Marshal(nil)
}

type GitHubAppIntegration struct {
	// Whether the app is installed for this org.
	Installed bool `json:"installed"`
}

type DeploymentSettings struct {
	Tag           string                    `json:"tag,omitempty" yaml:"tag,omitempty"`
	Executor      *ExecutorContext          `json:"executorContext,omitempty" yaml:"executorContext,omitempty"`
	SourceContext *SourceContext            `json:"sourceContext,omitempty" yaml:"sourceContext,omitempty"`
	GitHub        *DeploymentSettingsGitHub `json:"gitHub,omitempty" yaml:"gitHub,omitempty"`
	Operation     *OperationContext         `json:"operationContext,omitempty" yaml:"operationContext,omitempty"`
	AgentPoolID   *string                   `json:"agentPoolID,omitempty" yaml:"agentPoolID,omitempty"`
}

type DeploymentSettingsGitHub struct {
	Repository          string   `json:"repository,omitempty" yaml:"repository,omitempty"`
	PullRequestTemplate bool     `json:"pullRequestTemplate,omitempty" yaml:"pullRequestTemplate,omitempty"`
	DeployCommits       bool     `json:"deployCommits,omitempty" yaml:"deployCommits,omitempty"`
	PreviewPullRequests bool     `json:"previewPullRequests,omitempty" yaml:"previewPullRequests,omitempty"`
	DeployPullRequest   *int64   `json:"deployPullRequest,omitempty" yaml:"deployPullRequest,omitempty"`
	Paths               []string `json:"paths,omitempty" yaml:"paths,omitempty"`
}

type ExecutorContext struct {
	// WorkingDirectory defines the path where the work should be done when executing.
	WorkingDirectory string `json:"workingDirectory" yaml:"workingDirectory,omitempty"`

	// Defines the image that the pulumi operations should run in.
	ExecutorImage *DockerImage `json:"executorImage,omitempty" yaml:"executorImage,omitempty"`
}

// A DockerImage describes a Docker image reference + optional credentials for use with a job definition.
type DockerImage struct {
	Reference   string                  `json:"reference" yaml:"reference"`
	Credentials *DockerImageCredentials `json:"credentials,omitempty" yaml:"credentials,omitempty"`
}

type dockerImageJSON struct {
	Reference   string                  `json:"reference" yaml:"reference"`
	Credentials *DockerImageCredentials `json:"credentials,omitempty" yaml:"credentials,omitempty"`
}

func (d *DockerImage) MarshalJSON() ([]byte, error) {
	if d.Credentials != nil {
		return json.Marshal(dockerImageJSON{
			Reference:   d.Reference,
			Credentials: d.Credentials,
		})
	}
	return json.Marshal(d.Reference)
}

func (d *DockerImage) UnmarshalJSON(bytes []byte) error {
	var image dockerImageJSON
	if err := json.Unmarshal(bytes, &image); err == nil {
		d.Reference, d.Credentials = image.Reference, image.Credentials
		return nil
	}

	var reference string
	if err := json.Unmarshal(bytes, &reference); err != nil {
		return err
	}
	d.Reference, d.Credentials = reference, nil
	return nil
}

// DockerImageCredentials describes the credentials needed to access a Docker repository.
type DockerImageCredentials struct {
	Username string      `json:"username" yaml:"username"`
	Password SecretValue `json:"password" yaml:"password"`
}

// SourceContext describes some source code, and how to obtain it.
type SourceContext struct {
	Git *SourceContextGit `json:"git,omitempty" yaml:"git,omitempty"`
}

type SourceContextGit struct {
	RepoURL string `json:"repoUrl" yaml:"repoUrl,omitempty"`

	Branch string `json:"branch" yaml:"branch"`

	// (optional) RepoDir is the directory to work from in the project's source repository
	// where Pulumi.yaml is located. It is used in case Pulumi.yaml is not
	// in the project source root.
	RepoDir string `json:"repoDir,omitempty" yaml:"repoDir,omitempty"`

	// (optional) Commit is the hash of the commit to deploy. If used, HEAD will be in detached mode. This
	// is mutually exclusive with the Branch setting. Either value needs to be specified.
	Commit string `json:"commit,omitempty" yaml:"commit,omitempty"`

	// (optional) GitAuth allows configuring git authentication options
	// There are 3 different authentication options:
	//   * SSH private key (and its optional password)
	//   * Personal access token
	//   * Basic auth username and password
	// Only one authentication mode will be considered if more than one option is specified,
	// with ssh private key/password preferred first, then personal access token, and finally
	// basic auth credentials.
	GitAuth *GitAuthConfig `json:"gitAuth,omitempty" yaml:"gitAuth,omitempty"`
}

// GitAuthConfig specifies git authentication configuration options.
// There are 3 different authentication options:
//   - Personal access token
//   - SSH private key (and its optional password)
//   - Basic auth username and password
//
// Only 1 authentication mode is valid.
type GitAuthConfig struct {
	PersonalAccessToken *SecretValue `json:"accessToken,omitempty" yaml:"accessToken,omitempty"`
	SSHAuth             *SSHAuth     `json:"sshAuth,omitempty" yaml:"sshAuth,omitempty"`
	BasicAuth           *BasicAuth   `json:"basicAuth,omitempty" yaml:"basicAuth,omitempty"`
}

// SSHAuth configures ssh-based auth for git authentication.
// SSHPrivateKey is required but password is optional.
type SSHAuth struct {
	SSHPrivateKey SecretValue  `json:"sshPrivateKey" yaml:"sshPrivateKey"`
	Password      *SecretValue `json:"password,omitempty" yaml:"password,omitempty"`
}

// BasicAuth configures git authentication through basic auth —
// i.e. username and password. Both UserName and Password are required.
type BasicAuth struct {
	UserName SecretValue `json:"userName" yaml:"userName"`
	Password SecretValue `json:"password" yaml:"password"`
}

// OperationContext describes what to do.
type OperationContext struct {
	// OIDC contains the OIDC configuration for the operation.
	OIDC *OperationContextOIDCConfiguration `json:"oidc,omitempty" yaml:"oidc,omitempty"`
	// PreRunCommands is an optional list of arbitrary commands to run before Pulumi
	// is invoked.
	// ref: https://github.com/pulumi/pulumi/issues/9397
	PreRunCommands []string `json:"preRunCommands" yaml:"preRunCommands,omitempty"`

	// Operation is what we plan on doing.
	Operation PulumiOperation `json:"operation" yaml:"-"`

	// EnvironmentVariables contains environment variables to be applied during the execution.
	EnvironmentVariables map[string]SecretValue `json:"environmentVariables" yaml:"environmentVariables,omitempty"`

	// Options is a bag of settings to specify or override default behavior
	Options *OperationContextOptions `json:"options,omitempty" yaml:"options,omitempty"`
}

type OperationContextOIDCConfiguration struct {
	// AWS contains AWS-specific configuration.
	AWS *OperationContextAWSOIDCConfiguration `json:"aws,omitempty" yaml:"aws,omitempty"`
	// Azure contains Azure-specific configuration.
	Azure *OperationContextAzureOIDCConfiguration `json:"azure,omitempty" yaml:"azure,omitempty"`
	// GCP contains GCP-specific configuration.
	GCP *OperationContextGCPOIDCConfiguration `json:"gcp,omitempty" yaml:"gcp,omitempty"`
}

type OperationContextAWSOIDCConfiguration struct {
	// Duration is the duration of the assume-role session.
	Duration DeploymentDuration `json:"duration,omitempty" yaml:"duration,omitempty"`
	// PolicyARNs is an optional set of IAM policy ARNs that further restrict the assume-role session.
	PolicyARNs []string `json:"policyArns,omitempty" yaml:"policyArns,omitempty"`
	// The ARN of the role to assume using the OIDC token.
	RoleARN string `json:"roleArn" yaml:"roleArn"`
	// The name of the assume-role session.
	SessionName string `json:"sessionName" yaml:"sessionName"`
}

type OperationContextAzureOIDCConfiguration struct {
	// ClientID is the client ID of the federated workload identity.
	ClientID string `json:"clientId,omitempty" yaml:"clientId,omitempty"`
	// TenantID is the tenant ID of the federated workload identity.
	TenantID string `json:"tenantId,omitempty" yaml:"tenantId,omitempty"`
	// SubscriptionID is the subscription ID of the federated workload identity.
	SubscriptionID string `json:"subscriptionId,omitempty" yaml:"subscriptionId,omitempty"`
}

type OperationContextGCPOIDCConfiguration struct {
	// ProjectID is the numerical ID of the GCP project.
	ProjectID string `json:"projectId" yaml:"projectId"`
	// Region is the region of the GCP project.
	Region string `json:"region,omitempty" yaml:"region,omitempty"`
	// WorkloadPoolID is the ID of the workload pool to use.
	WorkloadPoolID string `json:"workloadPoolId" yaml:"workloadPoolId"`
	// ProviderID is the ID of the identity provider associated with the workload pool.
	ProviderID string `json:"providerId" yaml:"providerId"`
	// ServiceAccount is the email address of the service account to use.
	ServiceAccount string `json:"serviceAccount" yaml:"serviceAccount"`
	// TokenLifetime is the lifetime of the temporary credentials.
	TokenLifetime DeploymentDuration `json:"tokenLifetime,omitempty" yaml:"tokenLifetime,omitempty"`
}

// OperationContextOptions is a bag of settings to specify or override default behavior in a deployment
type OperationContextOptions struct {
	SkipInstallDependencies     bool   `json:"skipInstallDependencies" yaml:"skipInstallDependencies"`
	SkipIntermediateDeployments bool   `json:"skipIntermediateDeployments" yaml:"skipIntermediateDeployments"`
	Shell                       string `json:"shell" yaml:"shell,omitempty"`
	DeleteAfterDestroy          bool   `json:"deleteAfterDestroy" yaml:"deleteAfterDestroy"`
	RemediateIfDriftDetected    bool   `json:"remediateIfDriftDetected" yaml:"remediateIfDriftDetected"`
}

// CreateDeploymentResponse defines the response given when a new Deployment is created.
type CreateDeploymentResponse struct {
	// ID represents the generated Deployment ID.
	ID string `json:"id"`
	// ConsoleURL is the Console URL for the deployment.
	ConsoleURL string `json:"consoleUrl"`
}

type DeploymentLogLine struct {
	Header    string    `json:"header,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
	Line      string    `json:"line,omitempty"`
}

type DeploymentLogs struct {
	Lines     []DeploymentLogLine `json:"lines,omitempty"`
	NextToken string              `json:"nextToken,omitempty"`
}

// A SecretValue describes a possibly-secret value.
type SecretValue struct {
	Value      string
	Ciphertext string
	Secret     bool
}

type secretWorkflowValue struct {
	Secret     string `json:"secret,omitempty" yaml:"secret,omitempty"`
	Ciphertext string `json:"ciphertext,omitempty" yaml:"ciphertext,omitempty"`
}

func (v SecretValue) MarshalJSON() ([]byte, error) {
	switch {
	case len(v.Ciphertext) != 0:
		return json.Marshal(secretWorkflowValue{Ciphertext: v.Ciphertext})
	case v.Secret:
		return json.Marshal(secretWorkflowValue{Secret: v.Value})
	default:
		return json.Marshal(v.Value)
	}
}

func (v *SecretValue) UnmarshalJSON(bytes []byte) error {
	var secret secretWorkflowValue
	if err := json.Unmarshal(bytes, &secret); err == nil {
		v.Value, v.Ciphertext, v.Secret = secret.Secret, secret.Ciphertext, true
		return nil
	}

	var plaintext string
	if err := json.Unmarshal(bytes, &plaintext); err != nil {
		return err
	}
	v.Value, v.Secret = plaintext, false
	return nil
}

func (v SecretValue) MarshalYAML() (any, error) {
	switch {
	case len(v.Ciphertext) != 0:
		return secretWorkflowValue{Ciphertext: v.Ciphertext}, nil
	case v.Secret:
		return secretWorkflowValue{Secret: v.Value}, nil
	default:
		return v.Value, nil
	}
}

func (v *SecretValue) UnmarshalYAML(node *yaml.Node) error {
	var secret secretWorkflowValue
	if err := node.Decode(&secret); err == nil {
		v.Value, v.Ciphertext, v.Secret = secret.Secret, secret.Ciphertext, true
		return nil
	}

	var plaintext string
	if err := node.Decode(&plaintext); err != nil {
		return err
	}
	v.Value, v.Secret = plaintext, false
	return nil
}

type GetDeploymentUpdatesUpdateInfo struct {
	// UpdateID is the underlying Update's ID on the PPC.
	UpdateID string `json:"updateID"`

	// Version of the stack that this UpdateInfo describe.
	Version int `json:"version"`
	// LatestVersion of the stack in general. i.e. the latest when Version == LatestVersion.
	LatestVersion int `json:"latestVersion"`
}
