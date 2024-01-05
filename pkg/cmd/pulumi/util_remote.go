// Copyright 2016-2023, Pulumi Corporation.
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

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// This is a variable instead of a constant so it can be set in certain builds of the CLI that do not
// support remote deployments.
var disableRemote bool

// remoteSupported returns true if the CLI supports remote deployments.
func remoteSupported() bool {
	return !disableRemote && hasExperimentalCommands()
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

// validateUnsupportedRemoteFlags returns an error if any unsupported flags are set when --remote is set.
func validateUnsupportedRemoteFlags(
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
	replaces []string,
	targetReplaces []string,
	targetDependents bool,
	planFilePath string,
	stackConfigFile string,
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

	return nil
}

// Flags for remote operations.
type RemoteArgs struct {
	remote                   bool
	envVars                  []string
	secretEnvVars            []string
	preRunCommands           []string
	skipInstallDependencies  bool
	gitBranch                string
	gitCommit                string
	gitRepoDir               string
	gitAuthAccessToken       string
	gitAuthSSHPrivateKey     string
	gitAuthSSHPrivateKeyPath string
	gitAuthPassword          string
	gitAuthUsername          string
}

// Add flags to support remote operations
func (r *RemoteArgs) applyFlags(cmd *cobra.Command) {
	if !remoteSupported() {
		return
	}

	cmd.PersistentFlags().BoolVar(
		&r.remote, "remote", false,
		"[EXPERIMENTAL] Run the operation remotely")
	cmd.PersistentFlags().StringArrayVar(
		&r.envVars, "remote-env", []string{},
		"[EXPERIMENTAL] Environment variables to use in the remote operation of the form NAME=value "+
			"(e.g. `--remote-env FOO=bar`)")
	cmd.PersistentFlags().StringArrayVar(
		&r.secretEnvVars, "remote-env-secret", []string{},
		"[EXPERIMENTAL] Environment variables with secret values to use in the remote operation of the form "+
			"NAME=secretvalue (e.g. `--remote-env FOO=secret`)")
	cmd.PersistentFlags().StringArrayVar(
		&r.preRunCommands, "remote-pre-run-command", []string{},
		"[EXPERIMENTAL] Commands to run before the remote operation")
	cmd.PersistentFlags().BoolVar(
		&r.skipInstallDependencies, "remote-skip-install-dependencies", false,
		"[EXPERIMENTAL] Whether to skip the default dependency installation step")
	cmd.PersistentFlags().StringVar(
		&r.gitBranch, "remote-git-branch", "",
		"[EXPERIMENTAL] Git branch to deploy; this is mutually exclusive with --remote-git-commit; "+
			"either value needs to be specified")
	cmd.PersistentFlags().StringVar(
		&r.gitCommit, "remote-git-commit", "",
		"[EXPERIMENTAL] Git commit hash of the commit to deploy (if used, HEAD will be in detached mode); "+
			"this is mutually exclusive with --remote-git-branch; either value needs to be specified")
	cmd.PersistentFlags().StringVar(
		&r.gitRepoDir, "remote-git-repo-dir", "",
		"[EXPERIMENTAL] The directory to work from in the project's source repository "+
			"where Pulumi.yaml is located; used when Pulumi.yaml is not in the project source root")
	cmd.PersistentFlags().StringVar(
		&r.gitAuthAccessToken, "remote-git-auth-access-token", "",
		"[EXPERIMENTAL] Git personal access token")
	cmd.PersistentFlags().StringVar(
		&r.gitAuthSSHPrivateKey, "remote-git-auth-ssh-private-key", "",
		"[EXPERIMENTAL] Git SSH private key; use --remote-git-auth-password for the password, if needed")
	cmd.PersistentFlags().StringVar(
		&r.gitAuthSSHPrivateKeyPath, "remote-git-auth-ssh-private-key-path", "",
		"[EXPERIMENTAL] Git SSH private key path; use --remote-git-auth-password for the password, if needed")
	cmd.PersistentFlags().StringVar(
		&r.gitAuthPassword, "remote-git-auth-password", "",
		"[EXPERIMENTAL] Git password; for use with username or with an SSH private key")
	cmd.PersistentFlags().StringVar(
		&r.gitAuthUsername, "remote-git-auth-username", "",
		"[EXPERIMENTAL] Git username")
}

// runDeployment kicks off a remote deployment.
func runDeployment(ctx context.Context, opts display.Options, operation apitype.PulumiOperation, stack, url string,
	args RemoteArgs,
) result.Result {
	// Validate args.
	if url == "" {
		return result.FromError(errors.New("the url arg must be specified"))
	}
	if args.gitBranch != "" && args.gitCommit != "" {
		return result.FromError(errors.New("`--remote-git-branch` and `--remote-git-commit` cannot both be specified"))
	}
	if args.gitBranch == "" && args.gitCommit == "" {
		return result.FromError(errors.New("either `--remote-git-branch` or `--remote-git-commit` is required"))
	}
	if args.gitAuthSSHPrivateKey != "" && args.gitAuthSSHPrivateKeyPath != "" {
		return result.FromError(errors.New("`--remote-git-auth-ssh-private-key` and " +
			"`--remote-git-auth-ssh-private-key-path` cannot both be specified"))
	}

	// Parse and validate the environment args.
	env := map[string]apitype.SecretValue{}
	for i, e := range append(args.envVars, args.secretEnvVars...) {
		name, value, err := parseEnv(e)
		if err != nil {
			return result.FromError(err)
		}
		env[name] = apitype.SecretValue{
			Value:  value,
			Secret: i >= len(args.envVars),
		}
	}

	// Read the SSH Private Key from the path, if necessary.
	sshPrivateKey := args.gitAuthSSHPrivateKey
	if args.gitAuthSSHPrivateKeyPath != "" {
		key, err := os.ReadFile(args.gitAuthSSHPrivateKeyPath)
		if err != nil {
			return result.FromError(fmt.Errorf(
				"reading SSH private key path %q: %w", args.gitAuthSSHPrivateKeyPath, err))
		}
		sshPrivateKey = string(key)
	}

	// Try to read the current project
	project, _, err := readProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return result.FromError(err)
	}

	b, err := currentBackend(ctx, project, opts)
	if err != nil {
		return result.FromError(err)
	}

	// Ensure the cloud backend is being used.
	cb, isCloud := b.(httpstate.Backend)
	if !isCloud {
		return result.FromError(errors.New("the Pulumi Cloud backend must be used for remote operations; " +
			"use `pulumi login` without arguments to log into the Pulumi Cloud backend"))
	}

	stackRef, err := b.ParseStackReference(stack)
	if err != nil {
		return result.FromError(err)
	}

	var gitAuth *apitype.GitAuthConfig
	if args.gitAuthAccessToken != "" || sshPrivateKey != "" || args.gitAuthPassword != "" ||
		args.gitAuthUsername != "" {

		gitAuth = &apitype.GitAuthConfig{}
		switch {
		case args.gitAuthAccessToken != "":
			gitAuth.PersonalAccessToken = &apitype.SecretValue{Value: args.gitAuthAccessToken, Secret: true}

		case sshPrivateKey != "":
			sshAuth := &apitype.SSHAuth{
				SSHPrivateKey: apitype.SecretValue{Value: sshPrivateKey, Secret: true},
			}
			if args.gitAuthPassword != "" {
				sshAuth.Password = &apitype.SecretValue{Value: args.gitAuthPassword, Secret: true}
			}
			gitAuth.SSHAuth = sshAuth

		case args.gitAuthUsername != "":
			basicAuth := &apitype.BasicAuth{UserName: apitype.SecretValue{Value: args.gitAuthUsername, Secret: true}}
			if args.gitAuthPassword != "" {
				basicAuth.Password = apitype.SecretValue{Value: args.gitAuthPassword, Secret: true}
			}
			gitAuth.BasicAuth = basicAuth
		}
	}

	var operationOptions *apitype.OperationContextOptions
	if args.skipInstallDependencies {
		operationOptions = &apitype.OperationContextOptions{
			SkipInstallDependencies: args.skipInstallDependencies,
		}
	}

	req := apitype.CreateDeploymentRequest{
		Source: &apitype.SourceContext{
			Git: &apitype.SourceContextGit{
				RepoURL: url,
				Branch:  args.gitBranch,
				Commit:  args.gitCommit,
				RepoDir: args.gitRepoDir,
				GitAuth: gitAuth,
			},
		},
		Operation: &apitype.OperationContext{
			Operation:            operation,
			PreRunCommands:       args.preRunCommands,
			EnvironmentVariables: env,
			Options:              operationOptions,
		},
	}
	err = cb.RunDeployment(ctx, stackRef, req, opts)
	if err != nil {
		return result.FromError(err)
	}

	return nil
}
