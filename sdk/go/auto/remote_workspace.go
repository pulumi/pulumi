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
	"fmt"
	"strings"

	"github.com/pkg/errors"
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
		return RemoteStack{}, errors.Wrap(err, "failed to create stack")
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
		return RemoteStack{}, errors.Wrap(err, "failed to create stack")
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
		return RemoteStack{}, errors.Wrap(err, "failed to select stack")
	}

	s, err := SelectStack(ctx, stackName, w)
	if err != nil {
		return RemoteStack{}, err
	}
	return RemoteStack{stack: s}, nil
}

func remoteToLocalOptions(repo GitRepo, opts ...RemoteWorkspaceOption) ([]LocalWorkspaceOption, error) {
	if repo.Setup != nil {
		return nil, errors.New("repo.Setup cannot be used with remote workspaces")
	}
	if repo.URL == "" {
		return nil, errors.New("repo.URL is required")
	}
	if repo.Branch != "" && repo.CommitHash != "" {
		return nil, errors.New("repo.Branch and repo.CommitHash cannot both be specified")
	}
	if repo.Branch == "" && repo.CommitHash == "" {
		return nil, errors.New("either repo.Branch or repo.CommitHash is required")
	}
	if repo.Auth != nil {
		if repo.Auth.SSHPrivateKey != "" && repo.Auth.SSHPrivateKeyPath != "" {
			return nil, errors.New("repo.Auth.SSHPrivateKey and repo.Auth.SSHPrivateKeyPath cannot both be specified")
		}
	}

	remoteOpts := &remoteWorkspaceOptions{}
	for _, o := range opts {
		o.applyOption(remoteOpts)
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

	localOpts := []LocalWorkspaceOption{
		remote(true),
		remoteEnvVars(remoteOpts.EnvVars),
		preRunCommands(remoteOpts.PreRunCommands...),
		Repo(repo),
	}
	return localOpts, nil
}

type remoteWorkspaceOptions struct {
	// EnvVars is a map of environment values scoped to the workspace.
	// These values will be passed to all Workspace and Stack level commands.
	EnvVars map[string]EnvVarValue
	// PreRunCommands is an optional list of arbitrary commands to run before the remote Pulumi operation is invoked.
	PreRunCommands []string
}

// LocalWorkspaceOption is used to customize and configure a LocalWorkspace at initialization time.
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

// isFullyQualifiedStackName returns true if the stack is fully qualified,
// i.e. has owner, project, and stack components.
func isFullyQualifiedStackName(stackName string) bool {
	split := strings.Split(stackName, "/")
	return len(split) == 3 && split[0] != "" && split[1] != "" && split[2] != ""
}
