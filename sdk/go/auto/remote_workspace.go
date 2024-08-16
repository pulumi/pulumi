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

package auto

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// PREVIEW: NewRemoteStackGitSource creates a Stack backed by a RemoteWorkspace with source code from the specified
// GitRepo. Pulumi operations on the stack (Preview, Update, Refresh, and Destroy) are performed remotely.
func NewRemoteStackGitSource(
	ctx context.Context,
	stackName string, repo GitRepo,
	opts ...RemoteWorkspaceOption,
) (RemoteStack, error) {
	if !isFullyQualifiedStackName(stackName) {
		return RemoteStack{}, fmt.Errorf("stack name %q must be fully qualified", stackName)
	}

	localOpts, err := remoteToLocalOptions(repo, opts...)
	if err != nil {
		return RemoteStack{}, err
	}
	w, err := NewLocalWorkspace(ctx, localOpts...)
	if err != nil {
		return RemoteStack{}, fmt.Errorf("failed to create stack: %w", err)
	}

	s, err := NewStack(ctx, stackName, w)
	if err != nil {
		return RemoteStack{}, err
	}
	return RemoteStack{stack: s}, nil
}

// PREVIEW: UpsertRemoteStackGitSource creates a Stack backed by a RemoteWorkspace with source code from the
// specified GitRepo. If the Stack already exists, it will not error and proceed with returning the Stack.
// Pulumi operations on the stack (Preview, Update, Refresh, and Destroy) are performed remotely.
func UpsertRemoteStackGitSource(
	ctx context.Context,
	stackName string, repo GitRepo,
	opts ...RemoteWorkspaceOption,
) (RemoteStack, error) {
	if !isFullyQualifiedStackName(stackName) {
		return RemoteStack{}, fmt.Errorf("stack name %q must be fully qualified", stackName)
	}

	localOpts, err := remoteToLocalOptions(repo, opts...)
	if err != nil {
		return RemoteStack{}, err
	}
	w, err := NewLocalWorkspace(ctx, localOpts...)
	if err != nil {
		return RemoteStack{}, fmt.Errorf("failed to create stack: %w", err)
	}

	s, err := UpsertStack(ctx, stackName, w)
	if err != nil {
		return RemoteStack{}, err
	}
	return RemoteStack{stack: s}, nil
}

// PREVIEW: SelectRemoteStackGitSource selects an existing Stack backed by a RemoteWorkspace with source code from the
// specified GitRepo. Pulumi operations on the stack (Preview, Update, Refresh, and Destroy) are performed remotely.
func SelectRemoteStackGitSource(
	ctx context.Context,
	stackName string, repo GitRepo,
	opts ...RemoteWorkspaceOption,
) (RemoteStack, error) {
	if !isFullyQualifiedStackName(stackName) {
		return RemoteStack{}, fmt.Errorf("stack name %q must be fully qualified", stackName)
	}

	localOpts, err := remoteToLocalOptions(repo, opts...)
	if err != nil {
		return RemoteStack{}, err
	}
	w, err := NewLocalWorkspace(ctx, localOpts...)
	if err != nil {
		return RemoteStack{}, fmt.Errorf("failed to select stack: %w", err)
	}

	s, err := SelectStack(ctx, stackName, w)
	if err != nil {
		return RemoteStack{}, err
	}
	return RemoteStack{stack: s}, nil
}

func remoteToLocalOptions(repo GitRepo, opts ...RemoteWorkspaceOption) ([]LocalWorkspaceOption, error) {
	remoteOpts := &remoteWorkspaceOptions{}
	for _, o := range opts {
		o.applyOption(remoteOpts)
	}

	if !remoteOpts.InheritSettings {
		ifNotSet := " if RemoteInheritSettings(true) is not set"
		if repo.URL == "" {
			return nil, errors.New("repo.URL is required" + ifNotSet)
		}
		if repo.Branch == "" && repo.CommitHash == "" {
			return nil, errors.New("either repo.Branch or repo.CommitHash is required" + ifNotSet)
		}
	}
	if repo.Setup != nil {
		return nil, errors.New("repo.Setup cannot be used with remote workspaces")
	}
	if repo.Branch != "" && repo.CommitHash != "" {
		return nil, errors.New("repo.Branch and repo.CommitHash cannot both be specified")
	}
	if repo.Auth != nil {
		if repo.Auth.SSHPrivateKey != "" && repo.Auth.SSHPrivateKeyPath != "" {
			return nil, errors.New("repo.Auth.SSHPrivateKey and repo.Auth.SSHPrivateKeyPath cannot both be specified")
		}
	}

	for k, v := range remoteOpts.EnvVars {
		if k == "" {
			return nil, errors.New("envvar cannot be empty")
		}
		if v.Value == "" {
			return nil, fmt.Errorf("envvar %q cannot have an empty value", k)
		}
	}

	for index, command := range remoteOpts.PreRunCommands {
		if command == "" {
			return nil, fmt.Errorf("pre run command at index %v cannot be empty", index)
		}
	}

	if remoteOpts.ExecutorImage != nil {
		if remoteOpts.ExecutorImage.Image == "" {
			return nil, errors.New("executorImage.Image cannot be empty")
		}
		if remoteOpts.ExecutorImage.Credentials != nil {
			if remoteOpts.ExecutorImage.Credentials.Username == "" {
				return nil, errors.New("executorImage.Credentials.Username cannot be empty")
			}
			if remoteOpts.ExecutorImage.Credentials.Password == "" {
				return nil, errors.New("executorImage.Credentials.Password cannot be empty")
			}
		}
	}

	localOpts := []LocalWorkspaceOption{
		remote(true),
		remoteInheritSettings(remoteOpts.InheritSettings),
		remoteEnvVars(remoteOpts.EnvVars),
		preRunCommands(remoteOpts.PreRunCommands...),
		remoteSkipInstallDependencies(remoteOpts.SkipInstallDependencies),
		Repo(repo),
		remoteExecutorImage(remoteOpts.ExecutorImage),
		remoteAgentPoolID(remoteOpts.AgentPoolID),
	}
	return localOpts, nil
}

type remoteWorkspaceOptions struct {
	// InheritSettings sets whether to inherit deployment settings from the stack.
	InheritSettings bool
	// EnvVars is a map of environment values scoped to the workspace.
	// These values will be passed to all Workspace and Stack level commands.
	EnvVars map[string]EnvVarValue
	// PreRunCommands is an optional list of arbitrary commands to run before the remote Pulumi operation is invoked.
	PreRunCommands []string
	// SkipInstallDependencies sets whether to skip the default dependency installation step. Defaults to false.
	SkipInstallDependencies bool
	// ExecutorImage is the image to use for the remote executor.
	ExecutorImage *ExecutorImage
	// AgentPoolID is the agent pool (also called deployment runner pool) to use for the remote Pulumi operation.
	AgentPoolID string
}

type ExecutorImage struct {
	Image       string
	Credentials *DockerImageCredentials
}

type DockerImageCredentials struct {
	Username string
	Password string
}

// RemoteWorkspaceOption is used to customize and configure a RemoteWorkspace at initialization time.
// See Workdir, Program, PulumiHome, Project, Stacks, and Repo for concrete options.
type RemoteWorkspaceOption interface {
	applyOption(*remoteWorkspaceOptions)
}

type remoteWorkspaceOption func(*remoteWorkspaceOptions)

func (o remoteWorkspaceOption) applyOption(opts *remoteWorkspaceOptions) {
	o(opts)
}

// RemoteEnvVars is a map of environment values scoped to the remote workspace.
// These will be passed to remote operations.
func RemoteEnvVars(envvars map[string]EnvVarValue) RemoteWorkspaceOption {
	return remoteWorkspaceOption(func(opts *remoteWorkspaceOptions) {
		opts.EnvVars = envvars
	})
}

// RemotePreRunCommands is an optional list of arbitrary commands to run before the remote Pulumi operation is invoked.
func RemotePreRunCommands(commands ...string) RemoteWorkspaceOption {
	return remoteWorkspaceOption(func(opts *remoteWorkspaceOptions) {
		opts.PreRunCommands = commands
	})
}

// RemoteSkipInstallDependencies sets whether to skip the default dependency installation step. Defaults to false.
func RemoteSkipInstallDependencies(skipInstallDependencies bool) RemoteWorkspaceOption {
	return remoteWorkspaceOption(func(opts *remoteWorkspaceOptions) {
		opts.SkipInstallDependencies = skipInstallDependencies
	})
}

// RemoteInheritSettings sets whether to inherit deployment settings from the stack.
func RemoteInheritSettings(inheritSettings bool) RemoteWorkspaceOption {
	return remoteWorkspaceOption(func(opts *remoteWorkspaceOptions) {
		opts.InheritSettings = inheritSettings
	})
}

// RemoteExecutorImage sets the image to use for the remote executor.
func RemoteExecutorImage(image *ExecutorImage) RemoteWorkspaceOption {
	return remoteWorkspaceOption(func(opts *remoteWorkspaceOptions) {
		opts.ExecutorImage = image
	})
}

// RemoteExecutorImage sets the agent pool (also called deployment runner pool) to use for the
// remote Pulumi operation.
func RemoteAgentPoolID(agentPoolID string) RemoteWorkspaceOption {
	return remoteWorkspaceOption(func(opts *remoteWorkspaceOptions) {
		opts.AgentPoolID = agentPoolID
	})
}

// isFullyQualifiedStackName returns true if the stack is fully qualified,
// i.e. has owner, project, and stack components.
func isFullyQualifiedStackName(stackName string) bool {
	split := strings.Split(stackName, "/")
	return len(split) == 3 && split[0] != "" && split[1] != "" && split[2] != ""
}
