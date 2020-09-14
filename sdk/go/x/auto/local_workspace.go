// Copyright 2016-2020, Pulumi Corporation.
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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

// LocalWorkspace is a default implementation of the Workspace interface.
// A Workspace is the execution context containing a single Pulumi project, a program,
// and multiple stacks. Workspaces are used to manage the execution environment,
// providing various utilities such as plugin installation, environment configuration
// ($PULUMI_HOME), and creation, deletion, and listing of Stacks.
// LocalWorkspace relies on Pulumi.yaml and Pulumi.<stack>.yaml as the intermediate format
// for Project and Stack settings. Modifying ProjectSettings will
// alter the Workspace Pulumi.yaml file, and setting config on a Stack will modify the Pulumi.<stack>.yaml file.
// This is identical to the behavior of Pulumi CLI driven workspaces.
type LocalWorkspace struct {
	workDir         string
	pulumiHome      string
	program         pulumi.RunFunc
	envvars         map[string]string
	secretsProvider string
}

var settingsExtensions = []string{".yaml", ".yml", ".json"}

// ProjectSettings returns the settings object for the current project if any
// LocalWorkspace reads settings from the Pulumi.yaml in the workspace.
// A workspace can contain only a single project at a time.
func (l *LocalWorkspace) ProjectSettings(ctx context.Context) (*workspace.Project, error) {
	for _, ext := range settingsExtensions {
		projectPath := filepath.Join(l.WorkDir(), fmt.Sprintf("Pulumi%s", ext))
		if _, err := os.Stat(projectPath); err == nil {
			proj, err := workspace.LoadProject(projectPath)
			if err != nil {
				return nil, errors.Wrap(err, "found project settings, but failed to load")
			}
			return proj, nil
		}
	}
	return nil, errors.New("unable to find project settings in workspace")
}

// SaveProjectSettings overwrites the settings object in the current project.
// There can only be a single project per workspace. Fails is new project name does not match old.
// LocalWorkspace writes this value to a Pulumi.yaml file in Workspace.WorkDir().
func (l *LocalWorkspace) SaveProjectSettings(ctx context.Context, settings *workspace.Project) error {
	pulumiYamlPath := filepath.Join(l.WorkDir(), "Pulumi.yaml")
	return settings.Save(pulumiYamlPath)
}

// StackSettings returns the settings object for the stack matching the specified fullyQualifiedStackName if any.
// LocalWorkspace reads this from a Pulumi.<stack>.yaml file in Workspace.WorkDir().
func (l *LocalWorkspace) StackSettings(ctx context.Context, fqsn string) (*workspace.ProjectStack, error) {
	name, err := getStackFromFQSN(fqsn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load stack settings, invalid stack name")
	}
	for _, ext := range settingsExtensions {
		stackPath := filepath.Join(l.WorkDir(), fmt.Sprintf("pulumi.%s%s", name, ext))
		if _, err := os.Stat(stackPath); err != nil {
			proj, err := workspace.LoadProjectStack(stackPath)
			if err != nil {
				return nil, errors.Wrap(err, "found stack settings, but failed to load")
			}
			return proj, nil
		}
	}
	return nil, errors.Errorf("unable to find stack settings in workspace for %s", fqsn)
}

// SaveStackSettings overwrites the settings object for the stack matching the specified fullyQualifiedStackName.
// LocalWorkspace writes this value to a Pulumi.<stack>.yaml file in Workspace.WorkDir()
func (l *LocalWorkspace) SaveStackSettings(
	ctx context.Context,
	fqsn string,
	settings *workspace.ProjectStack,
) error {
	name, err := getStackFromFQSN(fqsn)
	if err != nil {
		return errors.Wrap(err, "failed to save stack settings, invalid stack name")
	}
	stackYamlPath := filepath.Join(l.WorkDir(), fmt.Sprintf("pulumi.%s.yaml:", name))
	err = settings.Save(stackYamlPath)
	if err != nil {
		return errors.Wrapf(err, "failed to save stack setttings for %s", fqsn)
	}
	return nil
}

// SerializeArgsForOp is hook to provide additional args to every CLI commands before they are executed.
// Provided with fullyQualifiedStackName,
// returns a list of args to append to an invoked command ["--config=...", ]
// LocalWorkspace does not utilize this extensibility point.
func (l *LocalWorkspace) SerializeArgsForOp(ctx context.Context, fqsn string) ([]string, error) {
	// not utilized for LocalWorkspace
	return nil, nil
}

// PostCommandCallback is a hook executed after every command. Called with the fullyQualifiedStackName.
// An extensibility point to perform workspace cleanup (CLI operations may create/modify a Pulumi.stack.yaml)
// LocalWorkspace does not utilize this extensibility point.
func (l *LocalWorkspace) PostCommandCallback(ctx context.Context, fqsn string) error {
	// not utilized for LocalWorkspace
	return nil
}

// GetConfig returns the value associated with the specified fullyQualifiedStackName and key,
// scoped to the current workspace. LocalWorkspace reads this config from the matching Pulumi.stack.yaml file.
func (l *LocalWorkspace) GetConfig(ctx context.Context, fqsn string, key string) (ConfigValue, error) {
	var val ConfigValue
	err := l.SelectStack(ctx, fqsn)
	if err != nil {
		return val, errors.Wrapf(err, "could not get config, unable to select stack %s", fqsn)
	}
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "config", "get", key, "--json")
	if err != nil {
		return val, newAutoError(errors.Wrap(err, "unable to read config"), stdout, stderr, errCode)
	}
	err = json.Unmarshal([]byte(stdout), &val)
	if err != nil {
		return val, errors.Wrap(err, "unable to unmarshal config value")
	}
	return val, nil
}

// GetAllConfig returns the config map for the specified fullyQualifiedStackName, scoped to the current workspace.
// LocalWorkspace reads this config from the matching Pulumi.stack.yaml file.
func (l *LocalWorkspace) GetAllConfig(ctx context.Context, fqsn string) (ConfigMap, error) {
	var val ConfigMap
	err := l.SelectStack(ctx, fqsn)
	if err != nil {
		return val, errors.Wrapf(err, "could not get config, unable to select stack %s", fqsn)
	}
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "config", "--show-secrets", "--json")
	if err != nil {
		return val, newAutoError(errors.Wrap(err, "unable read config"), stdout, stderr, errCode)
	}
	err = json.Unmarshal([]byte(stdout), &val)
	if err != nil {
		return val, errors.Wrap(err, "unable to unmarshal config value")
	}
	return val, nil
}

// SetConfig sets the specified key-value pair on the provided fullyQualifiedStackName.
// LocalWorkspace writes this value to the matching Pulumi.<stack>.yaml file in Workspace.WorkDir().
func (l *LocalWorkspace) SetConfig(ctx context.Context, fqsn string, key string, val ConfigValue) error {
	err := l.SelectStack(ctx, fqsn)
	if err != nil {
		return errors.Wrapf(err, "could not set config, unable to select stack %s", fqsn)
	}

	secretArg := "--plaintext"
	if val.Secret {
		secretArg = "--secret"
	}

	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "config", "set", key, val.Value, secretArg)
	if err != nil {
		return newAutoError(errors.Wrap(err, "unable set config"), stdout, stderr, errCode)
	}
	return nil
}

// SetAllConfig sets all values in the provided config map for the specified fullyQualifiedStackName.
// LocalWorkspace writes the config to the matching Pulumi.<stack>.yaml file in Workspace.WorkDir().
func (l *LocalWorkspace) SetAllConfig(ctx context.Context, fqsn string, config ConfigMap) error {
	for k, v := range config {
		err := l.SetConfig(ctx, fqsn, k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveConfig removes the specified key-value pair on the provided fullyQualifiedStackName.
// It will remove any matching values in the Pulumi.<stack>.yaml file in Workspace.WorkDir().
func (l *LocalWorkspace) RemoveConfig(ctx context.Context, fqsn string, key string) error {
	err := l.SelectStack(ctx, fqsn)
	if err != nil {
		return errors.Wrapf(err, "could not remove config, unable to select stack %s", fqsn)
	}

	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "config", "rm", key)
	if err != nil {
		return newAutoError(errors.Wrap(err, "could not remove config"), stdout, stderr, errCode)
	}
	return nil
}

// RemoveAllConfig removes all values in the provided key list for the specified fullyQualifiedStackName
// It will remove any matching values in the Pulumi.<stack>.yaml file in Workspace.WorkDir().
func (l *LocalWorkspace) RemoveAllConfig(ctx context.Context, fqsn string, keys []string) error {
	for _, k := range keys {
		err := l.RemoveConfig(ctx, fqsn, k)
		if err != nil {
			return err
		}
	}
	return nil
}

// RefreshConfig gets and sets the config map used with the last Update for Stack matching fullyQualifiedStackName.
// It will overwrite all configuration in the Pulumi.<stack>.yaml file in Workspace.WorkDir().
func (l *LocalWorkspace) RefreshConfig(ctx context.Context, fqsn string) (ConfigMap, error) {
	err := l.SelectStack(ctx, fqsn)
	if err != nil {
		return nil, errors.Wrapf(err, "could not refresh config, unable to select stack %s", fqsn)
	}

	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "config", "refresh", "--force")
	if err != nil {
		return nil, newAutoError(errors.Wrap(err, "could not refresh config"), stdout, stderr, errCode)
	}

	cfg, err := l.GetAllConfig(ctx, fqsn)
	if err != nil {
		return nil, errors.Wrap(err, "could not fetch config after refresh")
	}
	return cfg, nil
}

// GetEnvVars returns the environment values scoped to the current workspace.
func (l *LocalWorkspace) GetEnvVars() map[string]string {
	if l.envvars == nil {
		return nil
	}
	return l.envvars
}

// SetEnvVars sets the specified map of environment values scoped to the current workspace.
// These values will be passed to all Workspace and Stack level commands.
func (l *LocalWorkspace) SetEnvVars(envvars map[string]string) error {
	if envvars == nil {
		return errors.New("unable to set nil environment values")
	}
	if l.envvars == nil {
		l.envvars = map[string]string{}
	}
	for k, v := range envvars {
		l.envvars[k] = v
	}
	return nil
}

// SetEnvVar sets the specified environment value scoped to the current workspace.
// This value will be passed to all Workspace and Stack level commands.
func (l *LocalWorkspace) SetEnvVar(key, value string) {
	if l.envvars == nil {
		l.envvars = map[string]string{}
	}
	l.envvars[key] = value
}

// UnsetEnvVar unsets the specified environment value scoped to the current workspace.
// This value will be removed from all Workspace and Stack level commands.
func (l *LocalWorkspace) UnsetEnvVar(key string) {
	if l.envvars == nil {
		return
	}
	delete(l.envvars, key)
}

// WorkDir returns the working directory to run Pulumi CLI commands.
// LocalWorkspace expects that this directory contains a Pulumi.yaml file.
// For "Inline" Pulumi programs created from NewStackInlineSource, a Pulumi.yaml
// is created on behalf of the user if none is specified.
func (l *LocalWorkspace) WorkDir() string {
	return l.workDir
}

// PulumiHome returns the directory override for CLI metadata if set.
// This customizes the location of $PULUMI_HOME where metadata is stored and plugins are installed.
func (l *LocalWorkspace) PulumiHome() string {
	return l.pulumiHome
}

// WhoAmI returns the currently authenticated user
func (l *LocalWorkspace) WhoAmI(ctx context.Context) (string, error) {
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "whoami")
	if err != nil {
		return "", newAutoError(errors.Wrap(err, "could not determine authenticated user"), stdout, stderr, errCode)
	}
	return strings.TrimSpace(stdout), nil
}

// Stack returns a summary of the currently selected stack, if any.
func (l *LocalWorkspace) Stack(ctx context.Context) (*StackSummary, error) {
	stacks, err := l.ListStacks(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not determine selected stack")
	}
	for _, s := range stacks {
		if s.Current {
			return &s, nil
		}
	}
	return nil, nil
}

// CreateStack creates and sets a new stack with the fullyQualifiedStackName, failing if one already exists.
func (l *LocalWorkspace) CreateStack(ctx context.Context, fqsn string) error {
	err := ValidateFullyQualifiedStackName(fqsn)
	if err != nil {
		return errors.Wrap(err, "failed to create stack")
	}

	args := []string{"stack", "init", fqsn}
	if l.secretsProvider != "" {
		args = append(args, fmt.Sprintf("--secrets-provider=%s", l.secretsProvider))
	}
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, args...)
	if err != nil {
		return newAutoError(errors.Wrap(err, "failed to create stack"), stdout, stderr, errCode)
	}

	return nil
}

// SelectStack selects and sets an existing stack matching the fullyQualifiedStackName, failing if none exists.
func (l *LocalWorkspace) SelectStack(ctx context.Context, fqsn string) error {
	err := ValidateFullyQualifiedStackName(fqsn)
	if err != nil {
		return errors.Wrap(err, "failed to select stack")
	}

	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "stack", "select", fqsn)
	if err != nil {
		return newAutoError(errors.Wrap(err, "failed to select stack"), stdout, stderr, errCode)
	}

	return nil
}

// RemoveStack deletes the stack and all associated configuration and history.
func (l *LocalWorkspace) RemoveStack(ctx context.Context, fqsn string) error {
	err := ValidateFullyQualifiedStackName(fqsn)
	if err != nil {
		return errors.Wrap(err, "failed to remove stack")
	}

	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "stack", "rm", "--yes", fqsn)
	if err != nil {
		return newAutoError(errors.Wrap(err, "failed to remove stack"), stdout, stderr, errCode)
	}
	return nil
}

// ListStacks returns all Stacks created under the current Project.
// This queries underlying backend and may return stacks not present in the Workspace (as Pulumi.<stack>.yaml files).
func (l *LocalWorkspace) ListStacks(ctx context.Context) ([]StackSummary, error) {
	user, err := l.WhoAmI(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not list stacks")
	}

	proj, err := l.ProjectSettings(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not list stacks")
	}

	var stacks []StackSummary
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "stack", "ls", "--json")
	if err != nil {
		return stacks, newAutoError(errors.Wrap(err, "could not list stacks"), stdout, stderr, errCode)
	}
	err = json.Unmarshal([]byte(stdout), &stacks)
	if err != nil {
		return nil, errors.Wrap(err, "unable to unmarshal config value")
	}
	for _, s := range stacks {
		nameParts := strings.Split(s.Name, "/")
		if len(nameParts) == 1 {
			s.Name = fmt.Sprintf("%s/%s/%s", user, proj.Name.String(), s.Name)
		} else {
			s.Name = fmt.Sprintf("%s/%s/%s", nameParts[0], proj.Name.String(), nameParts[1])
		}
	}
	return stacks, nil
}

// InstallPlugin acquires the plugin matching the specified name and version.
func (l *LocalWorkspace) InstallPlugin(ctx context.Context, name string, version string) error {
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "plugin", "install", "resource", name, version)
	if err != nil {
		return newAutoError(errors.Wrap(err, "failed to install plugin"), stdout, stderr, errCode)
	}
	return nil
}

// RemovePlugin deletes the plugin matching the specified name and verision.
func (l *LocalWorkspace) RemovePlugin(ctx context.Context, name string, version string) error {
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "plugin", "rm", "resource", name, version)
	if err != nil {
		return newAutoError(errors.Wrap(err, "failed to remove plugin"), stdout, stderr, errCode)
	}
	return nil
}

// ListPlugins lists all installed plugins.
func (l *LocalWorkspace) ListPlugins(ctx context.Context) ([]workspace.PluginInfo, error) {
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "plugin", "ls", "--json")
	if err != nil {
		return nil, newAutoError(errors.Wrap(err, "could not list list"), stdout, stderr, errCode)
	}
	var plugins []workspace.PluginInfo
	err = json.Unmarshal([]byte(stdout), &plugins)
	if err != nil {
		return nil, errors.Wrap(err, "unable to unmarshal plugin response")
	}
	return plugins, nil
}

// Program returns the program `pulumi.RunFunc` to be used for Preview/Update if any.
// If none is specified, the stack will refer to ProjectSettings for this information.
func (l *LocalWorkspace) Program() pulumi.RunFunc {
	return l.program
}

// SetProgram sets the program associated with the Workspace to the specified `pulumi.RunFunc`.
func (l *LocalWorkspace) SetProgram(fn pulumi.RunFunc) {
	l.program = fn
}

func (l *LocalWorkspace) runPulumiCmdSync(
	ctx context.Context,
	args ...string,
) (string, string, int, error) {
	var env []string
	if l.PulumiHome() != "" {
		homeEnv := fmt.Sprintf("%s=%s", pulumiHomeEnv, l.PulumiHome())
		env = append(env, homeEnv)
	}
	if envvars := l.GetEnvVars(); envvars != nil {
		for k, v := range envvars {
			e := []string{k, v}
			env = append(env, strings.Join(e, "="))
		}
	}
	return runPulumiCommandSync(ctx, l.WorkDir(), env, args...)
}

// NewLocalWorkspace creates and configures a LocalWorkspace. LocalWorkspaceOptions can be used to
// configure things like the working directory, the program to execute, and to seed the directory with source code
// from a git repository.
func NewLocalWorkspace(ctx context.Context, opts ...LocalWorkspaceOption) (Workspace, error) {
	lwOpts := &localWorkspaceOptions{}
	// for merging options, last specified value wins
	for _, opt := range opts {
		opt.applyLocalWorkspaceOption(lwOpts)
	}

	var workDir string

	if lwOpts.WorkDir != "" {
		workDir = lwOpts.WorkDir
	} else {
		dir, err := ioutil.TempDir("", "pulumi_auto")
		if err != nil {
			return nil, errors.Wrap(err, "unable to create tmp directory for workspace")
		}
		workDir = dir
	}

	if lwOpts.Repo != nil {
		// now do the git clone
		projDir, err := setupGitRepo(ctx, workDir, lwOpts.Repo)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create workspace, unable to enlist in git repo")
		}
		workDir = projDir
	}

	var program pulumi.RunFunc
	if lwOpts.Program != nil {
		program = lwOpts.Program
	}

	l := &LocalWorkspace{
		workDir:    workDir,
		program:    program,
		pulumiHome: lwOpts.PulumiHome,
	}

	if lwOpts.Project != nil {
		err := l.SaveProjectSettings(ctx, lwOpts.Project)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create workspace, unable to save project settings")
		}
	}

	for fqsn := range lwOpts.Stacks {
		s := lwOpts.Stacks[fqsn]
		err := l.SaveStackSettings(ctx, fqsn, &s)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create workspace")
		}
	}

	// setup
	if lwOpts.Repo != nil && lwOpts.Repo.Setup != nil {
		err := lwOpts.Repo.Setup(ctx, l)
		if err != nil {
			return nil, errors.Wrap(err, "error while running setup function")
		}
	}

	// Secrets providers
	if lwOpts.SecretsProvider != "" {
		l.secretsProvider = lwOpts.SecretsProvider
	}

	return l, nil
}

type localWorkspaceOptions struct {
	// WorkDir is the directory to execute commands from and store state.
	// Defaults to a tmp dir.
	WorkDir string
	// Program is the Pulumi Program to execute. If none is supplied,
	// the program identified in $WORKDIR/Pulumi.yaml will be used instead.
	Program pulumi.RunFunc
	// PulumiHome overrides the metadata directory for pulumi commands.
	// This customizes the location of $PULUMI_HOME where metadata is stored and plugins are installed.
	PulumiHome string
	// Project is the project settings for the workspace.
	Project *workspace.Project
	// Stacks is a map of [fqsn -> stack settings objects] to seed the workspace.
	Stacks map[string]workspace.ProjectStack
	// Repo is a git repo with a Pulumi Project to clone into the WorkDir.
	Repo *GitRepo
	// Secrets Provider to use with the current Stack
	SecretsProvider string
}

// LocalWorkspaceOption is used to customize and configure a LocalWorkspace at initialization time.
// See Workdir, Program, PulumiHome, Project, Stacks, and Repo for concrete options.
type LocalWorkspaceOption interface {
	applyLocalWorkspaceOption(*localWorkspaceOptions)
}

type localWorkspaceOption func(*localWorkspaceOptions)

func (o localWorkspaceOption) applyLocalWorkspaceOption(opts *localWorkspaceOptions) {
	o(opts)
}

// GitRepo contains info to acquire and setup a Pulumi program from a git repository.
type GitRepo struct {
	// URL to clone git repo
	URL string
	// Optional path relative to the repo root specifying location of the pulumi program.
	// Specifying this option will update the Worspace's WorkDir accordingly.
	ProjectPath string
	// Optional branch to checkout.
	Branch string
	// Optional commit to checkout.
	CommitHash string
	// Optional function to execute after enlisting in the specified repo.
	Setup SetupFn
	// GitAuth is the different Authentication options for the Git repository
	Auth *GitAuth
}

// GitAuth is the authentication details that can be specified for a private Git repo.
// There are 3 different authentication paths:
// * PersonalAccessToken
// * SSHPrivateKeyPath (and it's potential password)
// * Username and Password
// Only 1 authentication path is valid. If more than 1 is specified it will result in an error
type GitAuth struct {
	// The absolute path to a private key for access to the git repo
	SSHPrivateKeyPath string
	// The password that pairs with a username or as part of an SSH Private Key
	Password string
	// PersonalAccessToken is a Git personal access token in replacement of your password
	PersonalAccessToken string
	// Username is the username to use when authenticating to a git repository
	Username string
}

// SetupFn is a function to execute after enlisting in a git repo.
// It is called with the workspace after all other options have been processed.
type SetupFn func(context.Context, Workspace) error

// WorkDir is the directory to execute commands from and store state.
func WorkDir(workDir string) LocalWorkspaceOption {
	return localWorkspaceOption(func(lo *localWorkspaceOptions) {
		lo.WorkDir = workDir
	})
}

// Program is the Pulumi Program to execute. If none is supplied,
// the program identified in $WORKDIR/Pulumi.yaml will be used instead.
func Program(program pulumi.RunFunc) LocalWorkspaceOption {
	return localWorkspaceOption(func(lo *localWorkspaceOptions) {
		lo.Program = program
	})
}

// PulumiHome overrides the metadata directory for pulumi commands.
func PulumiHome(dir string) LocalWorkspaceOption {
	return localWorkspaceOption(func(lo *localWorkspaceOptions) {
		lo.PulumiHome = dir
	})
}

// Project sets project settings for the workspace.
func Project(settings workspace.Project) LocalWorkspaceOption {
	return localWorkspaceOption(func(lo *localWorkspaceOptions) {
		lo.Project = &settings
	})
}

// Stacks is a list of stack settings objects to seed the workspace.
func Stacks(settings map[string]workspace.ProjectStack) LocalWorkspaceOption {
	return localWorkspaceOption(func(lo *localWorkspaceOptions) {
		lo.Stacks = settings
	})
}

// Repo is a git repo with a Pulumi Project to clone into the WorkDir.
func Repo(gitRepo GitRepo) LocalWorkspaceOption {
	return localWorkspaceOption(func(lo *localWorkspaceOptions) {
		lo.Repo = &gitRepo
	})
}

// SecretsProvider is the secrets provider to use with the current
// workspace when interacting with a stack
func SecretsProvider(secretsProvider string) LocalWorkspaceOption {
	return localWorkspaceOption(func(lo *localWorkspaceOptions) {
		lo.SecretsProvider = secretsProvider
	})
}

// ValidateFullyQualifiedStackName validates that the fqsn is in the form "org/project/name".
func ValidateFullyQualifiedStackName(fqsn string) error {
	parts := strings.Split(fqsn, "/")
	if len(parts) != 3 {
		return errors.Errorf(
			"invalid fully qualified stack name: %s, expected in the form 'org/project/stack'",
			fqsn,
		)
	}
	return nil
}

// NewStackLocalSource creates a Stack backed by a LocalWorkspace created on behalf of the user,
// from the specified WorkDir. This Workspace will pick up
// any available Settings files (Pulumi.yaml, Pulumi.<stack>.yaml).
func NewStackLocalSource(ctx context.Context, fqsn, workDir string, opts ...LocalWorkspaceOption) (Stack, error) {
	opts = append(opts, WorkDir(workDir))
	w, err := NewLocalWorkspace(ctx, opts...)
	var stack Stack
	if err != nil {
		return stack, errors.Wrap(err, "failed to create stack")
	}

	return NewStack(ctx, fqsn, w)
}

// UpsertStackLocalSource creates a Stack backed by a LocalWorkspace created on behalf of the user,
// from the specified WorkDir. If the Stack already exists, it will not error
// and proceed to selecting the Stack.This Workspace will pick up any available
// Settings files (Pulumi.yaml, Pulumi.<stack>.yaml).
func UpsertStackLocalSource(ctx context.Context, fqsn, workDir string, opts ...LocalWorkspaceOption) (Stack, error) {
	opts = append(opts, WorkDir(workDir))
	w, err := NewLocalWorkspace(ctx, opts...)
	var stack Stack
	if err != nil {
		return stack, errors.Wrap(err, "failed to create stack")
	}

	return UpsertStack(ctx, fqsn, w)
}

// SelectStackLocalSource selects an existing Stack backed by a LocalWorkspace created on behalf of the user,
// from the specified WorkDir. This Workspace will pick up
// any available Settings files (Pulumi.yaml, Pulumi.<stack>.yaml).
func SelectStackLocalSource(ctx context.Context, fqsn, workDir string, opts ...LocalWorkspaceOption) (Stack, error) {
	opts = append(opts, WorkDir(workDir))
	w, err := NewLocalWorkspace(ctx, opts...)
	var stack Stack
	if err != nil {
		return stack, errors.Wrap(err, "failed to select stack")
	}

	return SelectStack(ctx, fqsn, w)
}

// NewStackRemoteSource creates a Stack backed by a LocalWorkspace created on behalf of the user,
// with source code cloned from the specified GitRepo. This Workspace will pick up
// any available Settings files (Pulumi.yaml, Pulumi.<stack>.yaml) that are cloned into the Workspace.
// Unless a WorkDir option is specified, the GitRepo will be clone into a new temporary directory provided by the OS.
func NewStackRemoteSource(ctx context.Context, fqsn string, repo GitRepo, opts ...LocalWorkspaceOption) (Stack, error) {
	opts = append(opts, Repo(repo))
	w, err := NewLocalWorkspace(ctx, opts...)
	var stack Stack
	if err != nil {
		return stack, errors.Wrap(err, "failed to create stack")
	}

	return NewStack(ctx, fqsn, w)
}

// UpsertStackRemoteSource creates a Stack backed by a LocalWorkspace created on behalf of the user,
// with source code cloned from the specified GitRepo. If the Stack already exists,
// it will not error and proceed to selecting the Stack. This Workspace will pick up
// any available Settings files (Pulumi.yaml, Pulumi.<stack>.yaml) that are cloned
// into the Workspace. Unless a WorkDir option is specified, the GitRepo will be clone
// into a new temporary directory provided by the OS.
func UpsertStackRemoteSource(
	ctx context.Context, fqsn string, repo GitRepo, opts ...LocalWorkspaceOption) (Stack, error) {
	opts = append(opts, Repo(repo))
	w, err := NewLocalWorkspace(ctx, opts...)
	var stack Stack
	if err != nil {
		return stack, errors.Wrap(err, "failed to create stack")
	}

	return UpsertStack(ctx, fqsn, w)
}

// SelectStackRemoteSource selects an existing Stack backed by a LocalWorkspace created on behalf of the user,
// with source code cloned from the specified GitRepo. This Workspace will pick up
// any available Settings files (Pulumi.yaml, Pulumi.<stack>.yaml) that are cloned into the Workspace.
// Unless a WorkDir option is specified, the GitRepo will be clone into a new temporary directory provided by the OS.
func SelectStackRemoteSource(
	ctx context.Context,
	fqsn string, repo GitRepo,
	opts ...LocalWorkspaceOption,
) (Stack, error) {
	opts = append(opts, Repo(repo))
	w, err := NewLocalWorkspace(ctx, opts...)
	var stack Stack
	if err != nil {
		return stack, errors.Wrap(err, "failed to select stack")
	}

	return SelectStack(ctx, fqsn, w)
}

// NewStackInlineSource creates a Stack backed by a LocalWorkspace created on behalf of the user,
// with the specified program. If no Project option is specified, default project settings will be created
// on behalf of the user. Similarly, unless a WorkDir option is specified, the working directory will default
// to a new temporary directory provided by the OS.
func NewStackInlineSource(
	ctx context.Context,
	fqsn string,
	program pulumi.RunFunc,
	opts ...LocalWorkspaceOption,
) (Stack, error) {
	var stack Stack
	opts = append(opts, Program(program))
	proj, err := defaultInlineProject(fqsn)
	if err != nil {
		return stack, errors.Wrap(err, "failed to create stack")
	}
	// as we implictly create project on behalf of the user, prepend to opts in case the user specifies one.
	opts = append([]LocalWorkspaceOption{Project(proj)}, opts...)
	w, err := NewLocalWorkspace(ctx, opts...)
	if err != nil {
		return stack, errors.Wrap(err, "failed to create stack")
	}

	return NewStack(ctx, fqsn, w)
}

// UpsertStackInlineSource creates a Stack backed by a LocalWorkspace created on behalf of the user,
// with the specified program. If the Stack already exists, it will not error and
// proceed to selecting the Stack. If no Project option is specified, default project
// settings will be created on behalf of the user. Similarly, unless a WorkDir option
// is specified, the working directory will default to a new temporary directory provided by the OS.
func UpsertStackInlineSource(
	ctx context.Context,
	fqsn string,
	program pulumi.RunFunc,
	opts ...LocalWorkspaceOption,
) (Stack, error) {
	var stack Stack
	opts = append(opts, Program(program))
	proj, err := defaultInlineProject(fqsn)
	if err != nil {
		return stack, errors.Wrap(err, "failed to create stack")
	}
	// as we implictly create project on behalf of the user, prepend to opts in case the user specifies one.
	opts = append([]LocalWorkspaceOption{Project(proj)}, opts...)
	w, err := NewLocalWorkspace(ctx, opts...)
	if err != nil {
		return stack, errors.Wrap(err, "failed to create stack")
	}

	return UpsertStack(ctx, fqsn, w)
}

// SelectStackInlineSource selects an existing Stack backed by a new LocalWorkspace created on behalf of the user,
// with the specified program. If no Project option is specified, default project settings will be created
// on behalf of the user. Similarly, unless a WorkDir option is specified, the working directory will default
// to a new temporary directory provided by the OS.
func SelectStackInlineSource(
	ctx context.Context,
	fqsn string,
	program pulumi.RunFunc,
	opts ...LocalWorkspaceOption,
) (Stack, error) {
	var stack Stack
	opts = append(opts, Program(program))
	proj, err := defaultInlineProject(fqsn)
	if err != nil {
		return stack, errors.Wrap(err, "failed to select stack")
	}
	// as we implictly create project on behalf of the user, prepend to opts in case the user specifies one
	opts = append([]LocalWorkspaceOption{Project(proj)}, opts...)
	w, err := NewLocalWorkspace(ctx, opts...)
	if err != nil {
		return stack, errors.Wrap(err, "failed to select stack")
	}

	return SelectStack(ctx, fqsn, w)
}

func defaultInlineProject(fqsn string) (workspace.Project, error) {
	var proj workspace.Project
	err := ValidateFullyQualifiedStackName(fqsn)
	if err != nil {
		return proj, err
	}
	pName := strings.Split(fqsn, "/")[1]
	proj = workspace.Project{
		Name:    tokens.PackageName(pName),
		Runtime: workspace.NewProjectRuntimeInfo("go", nil),
	}

	return proj, nil
}

func getStackFromFQSN(fqsn string) (string, error) {
	if err := ValidateFullyQualifiedStackName(fqsn); err != nil {
		return "", err
	}
	return strings.Split(fqsn, "/")[2], nil
}

const pulumiHomeEnv = "PULUMI_HOME"
