// Copyright 2016-2024, Pulumi Corporation.
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

package deployment

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// This is a variable instead of a constant so it can be set in certain builds of the CLI that do not
// support remote deployments.
var disableRemote bool

// RemoteSupported returns true if the CLI supports remote deployments.
func RemoteSupported() bool {
	return !disableRemote && env.Experimental.Value()
}

// parseEnv parses a `--remote-env` flag value for `--remote`. A value should be of the form
// "NAME=value".
func parseEnv(input string) (string, string, error) {
	pair := strings.SplitN(input, "=", 2)
	if len(pair) != 2 {
		return "", "", fmt.Errorf(`expected value of the form "NAME=value": missing "=" in %q`, input)
	}
	name, value := pair[0], pair[1]

	if name == "" {
		return "", "", fmt.Errorf("expected non-empty environment name in %q", input)
	}

	return name, value, nil
}

// ValidateUnsupportedRemoteFlags returns an error if any unsupported flags are set when --remote is set.
func ValidateUnsupportedRemoteFlags(
	expectNop bool,
	configArray []string,
	configPath bool,
	client string,
	jsonDisplay bool,
	policyPackPaths []string,
	policyPackConfigPaths []string,
	refresh string,
	showConfig bool,
	showPolicyRemediations bool,
	showReplacementSteps bool,
	showSames bool,
	showReads bool,
	suppressOutputs bool,
	secretsProvider string,
	targets *[]string,
	excludes *[]string,
	replaces []string,
	targetReplaces []string,
	targetDependents bool,
	planFilePath string,
	stackConfigFile string,
	runProgram bool,
) error {
	if expectNop {
		return errors.New("--expect-no-changes is not supported with --remote")
	}
	if len(configArray) > 0 {
		return errors.New("--config is not supported with --remote")
	}
	if configPath {
		return errors.New("--config-path is not supported with --remote")
	}
	if client != "" {
		return errors.New("--client is not supported with --remote")
	}
	// We should be able to make --json work, but it doesn't work currently.
	if jsonDisplay {
		return errors.New("--json is not supported with --remote")
	}
	if len(policyPackPaths) > 0 {
		return errors.New("--policy-pack is not supported with --remote")
	}
	if len(policyPackConfigPaths) > 0 {
		return errors.New("--policy-pack-config is not supported with --remote")
	}
	if refresh != "" {
		return errors.New("--refresh is not supported with --remote")
	}
	if showConfig {
		return errors.New("--show-config is not supported with --remote")
	}
	if showPolicyRemediations {
		return errors.New("--show-policy-remediations is not supported with --remote")
	}
	if showReplacementSteps {
		return errors.New("--show-replacement-steps is not supported with --remote")
	}
	if showSames {
		return errors.New("--show-sames is not supported with --remote")
	}
	if showReads {
		return errors.New("--show-reads is not supported with --remote")
	}
	if suppressOutputs {
		return errors.New("--suppress-outputs is not supported with --remote")
	}
	if secretsProvider != "default" {
		return errors.New("--secrets-provider is not supported with --remote")
	}
	if targets != nil && len(*targets) > 0 {
		return errors.New("--target is not supported with --remote")
	}
	if excludes != nil && len(*excludes) > 0 {
		return errors.New("--exclude is not supported with --remote")
	}
	if len(replaces) > 0 {
		return errors.New("--replace is not supported with --remote")
	}
	if len(replaces) > 0 {
		return errors.New("--replace is not supported with --remote")
	}
	if len(targetReplaces) > 0 {
		return errors.New("--target-replace is not supported with --remote")
	}
	if targetDependents {
		return errors.New("--target-dependents is not supported with --remote")
	}
	if planFilePath != "" {
		return errors.New("--plan is not supported with --remote")
	}
	if stackConfigFile != "" {
		return errors.New("--config-file is not supported with --remote")
	}
	if runProgram {
		return errors.New("--run-program is not supported with --remote")
	}

	return nil
}

// Flags for remote operations.
type RemoteArgs struct {
	Remote                   bool
	InheritSettings          bool
	SuppressStreamLogs       bool
	EnvVars                  []string
	SecretEnvVars            []string
	PreRunCommands           []string
	SkipInstallDependencies  bool
	GitBranch                string
	GitCommit                string
	GitRepoDir               string
	GitAuthAccessToken       string
	GitAuthSSHPrivateKey     string
	GitAuthSSHPrivateKeyPath string
	GitAuthPassword          string
	GitAuthUsername          string
	ExecutorImage            string
	ExecutorImageUsername    string
	ExecutorImagePassword    string
	AgentPoolID              string
}

func (r *RemoteArgs) ApplyFlagsForDeploymentCommand(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolVar(
		&r.InheritSettings, "inherit-settings", true,
		"Inherit deployment settings from the current stack")
	cmd.PersistentFlags().BoolVar(
		&r.SuppressStreamLogs, "suppress-stream-logs", true,
		"Suppress log streaming of the deployment job")
	cmd.PersistentFlags().StringArrayVar(
		&r.EnvVars, "env", []string{},
		"Environment variables to use in the remote operation of the form NAME=value "+
			"(e.g. `--env FOO=bar`)")
	cmd.PersistentFlags().StringArrayVar(
		&r.SecretEnvVars, "env-secret", []string{},
		"Environment variables with secret values to use in the remote operation of the form "+
			"NAME=secretvalue (e.g. `--env FOO=secret`)")
	cmd.PersistentFlags().StringArrayVar(
		&r.PreRunCommands, "pre-run-command", []string{},
		"Commands to run before the remote operation")
	cmd.PersistentFlags().BoolVar(
		&r.SkipInstallDependencies, "skip-install-dependencies", false,
		"Whether to skip the default dependency installation step")
	cmd.PersistentFlags().StringVar(
		&r.GitBranch, "git-branch", "",
		"Git branch to deploy; this is mutually exclusive with --git-commit; "+
			"either value needs to be specified")
	cmd.PersistentFlags().StringVar(
		&r.GitCommit, "git-commit", "",
		"Git commit hash of the commit to deploy (if used, HEAD will be in detached mode); "+
			"this is mutually exclusive with --git-branch; either value needs to be specified")
	cmd.PersistentFlags().StringVar(
		&r.GitRepoDir, "git-repo-dir", "",
		"The directory to work from in the project's source repository "+
			"where Pulumi.yaml is located; used when Pulumi.yaml is not in the project source root")
	cmd.PersistentFlags().StringVar(
		&r.GitAuthAccessToken, "git-auth-access-token", "",
		"Git personal access token")
	cmd.PersistentFlags().StringVar(
		&r.GitAuthSSHPrivateKey, "git-auth-ssh-private-key", "",
		"Git SSH private key; use --git-auth-password for the password, if needed")
	cmd.PersistentFlags().StringVar(
		&r.GitAuthSSHPrivateKeyPath, "git-auth-ssh-private-key-path", "",
		"Git SSH private key path; use --git-auth-password for the password, if needed")
	cmd.PersistentFlags().StringVar(
		&r.GitAuthPassword, "git-auth-password", "",
		"Git password; for use with username or with an SSH private key")
	cmd.PersistentFlags().StringVar(
		&r.GitAuthUsername, "git-auth-username", "",
		"Git username")
	cmd.PersistentFlags().StringVar(
		&r.ExecutorImage, "executor-image", "",
		"The Docker image to use for the executor")
	cmd.PersistentFlags().StringVar(
		&r.ExecutorImageUsername, "executor-image-username", "",
		"The username for the credentials with access to the Docker image to use for the executor")
	cmd.PersistentFlags().StringVar(
		&r.ExecutorImagePassword, "executor-image-password", "",
		"The password for the credentials with access to the Docker image to use for the executor")
	cmd.PersistentFlags().StringVar(
		&r.AgentPoolID, "agent-pool-id", "",
		"The agent pool to use to run the deployment job. When empty, the Pulumi Cloud shared queue "+
			"will be used.")
}

// Add flags to support remote operations
func (r *RemoteArgs) ApplyFlags(cmd *cobra.Command) {
	if !RemoteSupported() {
		return
	}

	cmd.PersistentFlags().BoolVar(
		&r.Remote, "remote", false,
		"[EXPERIMENTAL] Run the operation remotely")
	cmd.PersistentFlags().BoolVar(
		&r.SuppressStreamLogs, "suppress-stream-logs", false,
		"[EXPERIMENTAL] Suppress log streaming of the deployment job")
	cmd.PersistentFlags().BoolVar(
		&r.InheritSettings, "remote-inherit-settings", false,
		"[EXPERIMENTAL] Inherit deployment settings from the current stack")
	cmd.PersistentFlags().StringArrayVar(
		&r.EnvVars, "remote-env", []string{},
		"[EXPERIMENTAL] Environment variables to use in the remote operation of the form NAME=value "+
			"(e.g. `--remote-env FOO=bar`)")
	cmd.PersistentFlags().StringArrayVar(
		&r.SecretEnvVars, "remote-env-secret", []string{},
		"[EXPERIMENTAL] Environment variables with secret values to use in the remote operation of the form "+
			"NAME=secretvalue (e.g. `--remote-env FOO=secret`)")
	cmd.PersistentFlags().StringArrayVar(
		&r.PreRunCommands, "remote-pre-run-command", []string{},
		"[EXPERIMENTAL] Commands to run before the remote operation")
	cmd.PersistentFlags().BoolVar(
		&r.SkipInstallDependencies, "remote-skip-install-dependencies", false,
		"[EXPERIMENTAL] Whether to skip the default dependency installation step")
	cmd.PersistentFlags().StringVar(
		&r.GitBranch, "remote-git-branch", "",
		"[EXPERIMENTAL] Git branch to deploy; this is mutually exclusive with --remote-git-commit; "+
			"either value needs to be specified")
	cmd.PersistentFlags().StringVar(
		&r.GitCommit, "remote-git-commit", "",
		"[EXPERIMENTAL] Git commit hash of the commit to deploy (if used, HEAD will be in detached mode); "+
			"this is mutually exclusive with --remote-git-branch; either value needs to be specified")
	cmd.PersistentFlags().StringVar(
		&r.GitRepoDir, "remote-git-repo-dir", "",
		"[EXPERIMENTAL] The directory to work from in the project's source repository "+
			"where Pulumi.yaml is located; used when Pulumi.yaml is not in the project source root")
	cmd.PersistentFlags().StringVar(
		&r.GitAuthAccessToken, "remote-git-auth-access-token", "",
		"[EXPERIMENTAL] Git personal access token")
	cmd.PersistentFlags().StringVar(
		&r.GitAuthSSHPrivateKey, "remote-git-auth-ssh-private-key", "",
		"[EXPERIMENTAL] Git SSH private key; use --remote-git-auth-password for the password, if needed")
	cmd.PersistentFlags().StringVar(
		&r.GitAuthSSHPrivateKeyPath, "remote-git-auth-ssh-private-key-path", "",
		"[EXPERIMENTAL] Git SSH private key path; use --remote-git-auth-password for the password, if needed")
	cmd.PersistentFlags().StringVar(
		&r.GitAuthPassword, "remote-git-auth-password", "",
		"[EXPERIMENTAL] Git password; for use with username or with an SSH private key")
	cmd.PersistentFlags().StringVar(
		&r.GitAuthUsername, "remote-git-auth-username", "",
		"[EXPERIMENTAL] Git username")
	cmd.PersistentFlags().StringVar(
		&r.ExecutorImage, "remote-executor-image", "",
		"[EXPERIMENTAL] The Docker image to use for the executor")
	cmd.PersistentFlags().StringVar(
		&r.ExecutorImageUsername, "remote-executor-image-username", "",
		"[EXPERIMENTAL] The username for the credentials with access to the Docker image to use for the executor")
	cmd.PersistentFlags().StringVar(
		&r.ExecutorImagePassword, "remote-executor-image-password", "",
		"[EXPERIMENTAL] The password for the credentials with access to the Docker image to use for the executor")
	cmd.PersistentFlags().StringVar(
		&r.AgentPoolID, "remote-agent-pool-id", "",
		"[EXPERIMENTAL] The agent pool to use to run the deployment job. When empty, the Pulumi Cloud shared queue "+
			"will be used.")
}

func ValidateRemoteDeploymentFlags(url string, args RemoteArgs) error {
	if url == "" && !args.InheritSettings {
		return errors.New("the url arg must be specified if not passing --remote-inherit-settings")
	}
	if args.GitBranch != "" && args.GitCommit != "" {
		return errors.New("`--remote-git-branch` and `--remote-git-commit` cannot both be specified")
	}
	if args.GitBranch == "" && args.GitCommit == "" && !args.InheritSettings {
		return errors.New("either `--remote-git-branch` or `--remote-git-commit` is required " +
			"if not passing --remote-inherit-settings")
	}
	if args.GitAuthSSHPrivateKey != "" && args.GitAuthSSHPrivateKeyPath != "" {
		return errors.New("`--remote-git-auth-ssh-private-key` and " +
			"`--remote-git-auth-ssh-private-key-path` cannot both be specified")
	}
	if args.ExecutorImage == "" && (args.ExecutorImageUsername != "" || args.ExecutorImagePassword != "") {
		return errors.New("`--remote-executor-image-username` and `--remote-executor-image-password` " +
			"cannot be specified without `--remote-executor-image`")
	}
	if (args.ExecutorImagePassword != "" && args.ExecutorImageUsername == "") ||
		(args.ExecutorImageUsername != "" && args.ExecutorImagePassword == "") {
		return errors.New("`--remote-executor-image-username` and `--remote-executor-image-password` " +
			"must both be specified")
	}

	return nil
}

func validateDeploymentFlags(url string, args RemoteArgs) error {
	if url == "" && !args.InheritSettings {
		return errors.New("the url arg must be specified if not passing --inherit-settings")
	}
	if args.GitBranch != "" && args.GitCommit != "" {
		return errors.New("`--git-branch` and `--git-commit` cannot both be specified")
	}
	if args.GitBranch == "" && args.GitCommit == "" && !args.InheritSettings {
		return errors.New("either `--git-branch` or `--git-commit` is required " +
			"if not passing --inherit-settings")
	}
	if args.GitAuthSSHPrivateKey != "" && args.GitAuthSSHPrivateKeyPath != "" {
		return errors.New("`--git-auth-ssh-private-key` and " +
			"`--git-auth-ssh-private-key-path` cannot both be specified")
	}
	if args.ExecutorImage == "" && (args.ExecutorImageUsername != "" || args.ExecutorImagePassword != "") {
		return errors.New("`--executor-image-username` and `--executor-image-password` " +
			"cannot be specified without `--executor-image`")
	}
	if (args.ExecutorImagePassword != "" && args.ExecutorImageUsername == "") ||
		(args.ExecutorImageUsername != "" && args.ExecutorImagePassword == "") {
		return errors.New("`--executor-image-username` and `--executor-image-password` " +
			"must both be specified")
	}

	return nil
}

// RunDeployment kicks off a remote deployment.
func RunDeployment(ctx context.Context, ws pkgWorkspace.Context, cmd *cobra.Command, opts display.Options,
	operation apitype.PulumiOperation, stack, url string, args RemoteArgs,
) error {
	// Parse and validate the environment args.
	env := map[string]apitype.SecretValue{}
	for i, e := range append(args.EnvVars, args.SecretEnvVars...) {
		name, value, err := parseEnv(e)
		if err != nil {
			return err
		}
		env[name] = apitype.SecretValue{
			Value:  value,
			Secret: i >= len(args.EnvVars),
		}
	}

	// Read the SSH Private Key from the path, if necessary.
	sshPrivateKey := args.GitAuthSSHPrivateKey
	if args.GitAuthSSHPrivateKeyPath != "" {
		key, err := os.ReadFile(args.GitAuthSSHPrivateKeyPath)
		if err != nil {
			return fmt.Errorf(
				"reading SSH private key path %q: %w", args.GitAuthSSHPrivateKeyPath, err)
		}
		sshPrivateKey = string(key)
	}

	// Try to read the current project
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return err
	}

	b, err := backend.CurrentBackend(ctx, ws, backend.DefaultLoginManager, project, opts)
	if err != nil {
		return err
	}

	// Ensure the cloud backend is being used.
	cb, isCloud := b.(httpstate.Backend)
	if !isCloud {
		return errors.New("the Pulumi Cloud backend must be used for remote operations; " +
			"use `pulumi login` without arguments to log into the Pulumi Cloud backend")
	}

	stackRef, err := b.ParseStackReference(stack)
	if err != nil {
		return err
	}

	var gitAuth *apitype.GitAuthConfig
	if args.GitAuthAccessToken != "" || sshPrivateKey != "" || args.GitAuthPassword != "" ||
		args.GitAuthUsername != "" {
		gitAuth = &apitype.GitAuthConfig{}
		switch {
		case args.GitAuthAccessToken != "":
			gitAuth.PersonalAccessToken = &apitype.SecretValue{Value: args.GitAuthAccessToken, Secret: true}

		case sshPrivateKey != "":
			sshAuth := &apitype.SSHAuth{
				SSHPrivateKey: apitype.SecretValue{Value: sshPrivateKey, Secret: true},
			}
			if args.GitAuthPassword != "" {
				sshAuth.Password = &apitype.SecretValue{Value: args.GitAuthPassword, Secret: true}
			}
			gitAuth.SSHAuth = sshAuth

		case args.GitAuthUsername != "":
			basicAuth := &apitype.BasicAuth{UserName: apitype.SecretValue{Value: args.GitAuthUsername, Secret: true}}
			if args.GitAuthPassword != "" {
				basicAuth.Password = apitype.SecretValue{Value: args.GitAuthPassword, Secret: true}
			}
			gitAuth.BasicAuth = basicAuth
		}
	}

	var executorImage *apitype.DockerImage
	if args.ExecutorImage != "" {
		executorImage = &apitype.DockerImage{
			Reference: args.ExecutorImage,
		}
	}
	if args.ExecutorImageUsername != "" && args.ExecutorImagePassword != "" {
		if executorImage.Credentials == nil {
			executorImage.Credentials = &apitype.DockerImageCredentials{}
		}
		executorImage.Credentials.Username = args.ExecutorImageUsername
		executorImage.Credentials.Password = apitype.SecretValue{Value: args.ExecutorImagePassword, Secret: true}
	}

	var operationOptions *apitype.OperationContextOptions
	if args.SkipInstallDependencies {
		operationOptions = &apitype.OperationContextOptions{
			SkipInstallDependencies: args.SkipInstallDependencies,
		}
	}

	var sourceContext *apitype.SourceContext
	if url != "" || args.GitBranch != "" || args.GitCommit != "" || args.GitRepoDir != "" || gitAuth != nil {
		sourceContext = &apitype.SourceContext{
			Git: &apitype.SourceContextGit{},
		}
		if url != "" {
			sourceContext.Git.RepoURL = url
		}
		if args.GitBranch != "" {
			sourceContext.Git.Branch = args.GitBranch
		}
		if args.GitCommit != "" {
			sourceContext.Git.Commit = args.GitCommit
		}
		if args.GitRepoDir != "" {
			sourceContext.Git.RepoDir = args.GitRepoDir
		}
		if gitAuth != nil {
			sourceContext.Git.GitAuth = gitAuth
		}
	}

	// we have a custom marshaller for CreateDeploymentRequest, to handle semantics around
	// defined/undefined/null values on AgentPoolID
	agentPoolID := apitype.AgentPoolIDMarshaller(args.AgentPoolID)

	req := apitype.CreateDeploymentRequest{
		Op:              operation,
		InheritSettings: args.InheritSettings,
		Executor: &apitype.ExecutorContext{
			ExecutorImage: executorImage,
		},
		Source: sourceContext,
		Operation: &apitype.OperationContext{
			PreRunCommands:       args.PreRunCommands,
			EnvironmentVariables: env,
			Options:              operationOptions,
		},
		AgentPoolID: &agentPoolID,
	}

	// For now, these commands are only used by automation API, so we can unilaterally set the initiator
	// to "automation-api".
	// In the future, we may want to expose initiating deployments from the CLI, in which case we would need to
	// pass this value in from the CLI as a flag or environment variable.
	err = cb.RunDeployment(ctx, stackRef, req, opts, "automation-api" /*deploymentInitiator*/, args.SuppressStreamLogs)
	if err != nil {
		return err
	}

	return nil
}
