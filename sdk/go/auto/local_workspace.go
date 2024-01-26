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

package auto

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/sdk/v3/go/auto/optremove"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
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
	workDir                       string
	pulumiHome                    string
	program                       pulumi.RunFunc
	envvars                       map[string]string
	secretsProvider               string
	repo                          *GitRepo
	remote                        bool
	remoteEnvVars                 map[string]EnvVarValue
	preRunCommands                []string
	remoteSkipInstallDependencies bool
	pulumiCommand                 PulumiCommand
}

var settingsExtensions = []string{".yaml", ".yml", ".json"}

// ProjectSettings returns the settings object for the current project if any
// LocalWorkspace reads settings from the Pulumi.yaml in the workspace.
// A workspace can contain only a single project at a time.
func (l *LocalWorkspace) ProjectSettings(ctx context.Context) (*workspace.Project, error) {
	return readProjectSettingsFromDir(ctx, l.WorkDir())
}

// SaveProjectSettings overwrites the settings object in the current project.
// There can only be a single project per workspace. Fails is new project name does not match old.
// LocalWorkspace writes this value to a Pulumi.yaml file in Workspace.WorkDir().
func (l *LocalWorkspace) SaveProjectSettings(ctx context.Context, settings *workspace.Project) error {
	pulumiYamlPath := filepath.Join(l.WorkDir(), "Pulumi.yaml")
	return settings.Save(pulumiYamlPath)
}

// StackSettings returns the settings object for the stack matching the specified stack name if any.
// LocalWorkspace reads this from a Pulumi.<stack>.yaml file in Workspace.WorkDir().
func (l *LocalWorkspace) StackSettings(ctx context.Context, stackName string) (*workspace.ProjectStack, error) {
	project, err := l.ProjectSettings(ctx)
	if err != nil {
		return nil, err
	}

	name := getStackSettingsName(stackName)
	for _, ext := range settingsExtensions {
		stackPath := filepath.Join(l.WorkDir(), fmt.Sprintf("Pulumi.%s%s", name, ext))
		if _, err := os.Stat(stackPath); err == nil {
			proj, err := workspace.LoadProjectStack(project, stackPath)
			if err != nil {
				return nil, fmt.Errorf("found stack settings, but failed to load: %w", err)
			}
			return proj, nil
		}
	}
	return nil, fmt.Errorf("unable to find stack settings in workspace for %s", stackName)
}

// SaveStackSettings overwrites the settings object for the stack matching the specified stack name.
// LocalWorkspace writes this value to a Pulumi.<stack>.yaml file in Workspace.WorkDir()
func (l *LocalWorkspace) SaveStackSettings(
	ctx context.Context,
	stackName string,
	settings *workspace.ProjectStack,
) error {
	name := getStackSettingsName(stackName)
	stackYamlPath := filepath.Join(l.WorkDir(), fmt.Sprintf("Pulumi.%s.yaml", name))
	err := settings.Save(stackYamlPath)
	if err != nil {
		return fmt.Errorf("failed to save stack setttings for %s: %w", stackName, err)
	}
	return nil
}

// SerializeArgsForOp is hook to provide additional args to every CLI commands before they are executed.
// Provided with stack name,
// returns a list of args to append to an invoked command ["--config=...", ]
// LocalWorkspace does not utilize this extensibility point.
func (l *LocalWorkspace) SerializeArgsForOp(ctx context.Context, stackName string) ([]string, error) {
	// not utilized for LocalWorkspace
	return nil, nil
}

// PostCommandCallback is a hook executed after every command. Called with the stack name.
// An extensibility point to perform workspace cleanup (CLI operations may create/modify a Pulumi.stack.yaml)
// LocalWorkspace does not utilize this extensibility point.
func (l *LocalWorkspace) PostCommandCallback(ctx context.Context, stackName string) error {
	// not utilized for LocalWorkspace
	return nil
}

// AddEnvironments adds environments to the end of a stack's import list. Imported environments are merged in order
// per the ESC merge rules. The list of environments behaves as if it were the import list in an anonymous
// environment.
func (l *LocalWorkspace) AddEnvironments(ctx context.Context, stackName string, envs ...string) error {
	// 3.95 added this command (https://github.com/pulumi/pulumi/releases/tag/v3.95.0)
	if l.pulumiCommand.Version().LT(semver.Version{Major: 3, Minor: 95}) {
		return fmt.Errorf("AddEnvironments requires Pulumi CLI version >= 3.95.0")
	}
	args := []string{"config", "env", "add"}
	args = append(args, envs...)
	args = append(args, "--yes", "--stack", stackName)
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, args...)
	if err != nil {
		return newAutoError(fmt.Errorf("unable to add environments: %w", err), stdout, stderr, errCode)
	}
	return nil
}

// ListEnvironments returns the list of environments from the provided stack's configuration.
func (l *LocalWorkspace) ListEnvironments(ctx context.Context, stackName string) ([]string, error) {
	// 3.99 added this command (https://github.com/pulumi/pulumi/releases/tag/v3.99.0)
	if l.pulumiCommand.Version().LT(semver.Version{Major: 3, Minor: 99}) {
		return nil, fmt.Errorf("ListEnvironments requires Pulumi CLI version >= 3.99.0")
	}
	args := []string{"config", "env", "ls", "--stack", stackName, "--json"}
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, args...)
	if err != nil {
		return nil, newAutoError(fmt.Errorf("unable to list environments: %w", err), stdout, stderr, errCode)
	}
	var envs []string
	err = json.Unmarshal([]byte(stdout), &envs)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal environments: %w", err)
	}
	return envs, nil
}

// RemoveEnvironment removes an environment from a stack's configuration.
func (l *LocalWorkspace) RemoveEnvironment(ctx context.Context, stackName string, env string) error {
	// 3.95 added this command (https://github.com/pulumi/pulumi/releases/tag/v3.95.0)
	if l.pulumiCommand.Version().LT(semver.Version{Major: 3, Minor: 95}) {
		return fmt.Errorf("RemoveEnvironments requires Pulumi CLI version >= 3.95.0")
	}
	args := []string{"config", "env", "rm", env, "--yes", "--stack", stackName}
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, args...)
	if err != nil {
		return newAutoError(fmt.Errorf("unable to remove environment: %w", err), stdout, stderr, errCode)
	}
	return nil
}

// GetConfig returns the value associated with the specified stack name and key,
// scoped to the current workspace. LocalWorkspace reads this config from the matching Pulumi.stack.yaml file.
func (l *LocalWorkspace) GetConfig(ctx context.Context, stackName string, key string) (ConfigValue, error) {
	return l.GetConfigWithOptions(ctx, stackName, key, nil)
}

// GetConfigWithOptions returns the value associated with the specified stack name and key,
// using the optional ConfigOptions, scoped to the current workspace.
// LocalWorkspace reads this config from the matching Pulumi.stack.yaml file.
func (l *LocalWorkspace) GetConfigWithOptions(
	ctx context.Context, stackName string, key string, opts *ConfigOptions,
) (ConfigValue, error) {
	var val ConfigValue
	args := []string{"config", "get"}
	if opts != nil {
		if opts.Path {
			args = append(args, "--path")
		}
	}
	args = append(args, key, "--json", "--stack", stackName)
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, args...)
	if err != nil {
		return val, newAutoError(fmt.Errorf("unable to read config: %w", err), stdout, stderr, errCode)
	}
	err = json.Unmarshal([]byte(stdout), &val)
	if err != nil {
		return val, fmt.Errorf("unable to unmarshal config value: %w", err)
	}
	return val, nil
}

// GetAllConfig returns the config map for the specified stack name, scoped to the current workspace.
// LocalWorkspace reads this config from the matching Pulumi.stack.yaml file.
func (l *LocalWorkspace) GetAllConfig(ctx context.Context, stackName string) (ConfigMap, error) {
	var val ConfigMap
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "config", "--show-secrets", "--json", "--stack", stackName)
	if err != nil {
		return val, newAutoError(fmt.Errorf("unable to read config: %w", err), stdout, stderr, errCode)
	}
	err = json.Unmarshal([]byte(stdout), &val)
	if err != nil {
		return val, fmt.Errorf("unable to unmarshal config value: %w", err)
	}
	return val, nil
}

// SetConfig sets the specified key-value pair on the provided stack name.
// LocalWorkspace writes this value to the matching Pulumi.<stack>.yaml file in Workspace.WorkDir().
func (l *LocalWorkspace) SetConfig(ctx context.Context, stackName string, key string, val ConfigValue) error {
	return l.SetConfigWithOptions(ctx, stackName, key, val, nil)
}

// SetConfigWithOptions sets the specified key-value pair on the provided stack name using the optional ConfigOptions.
// LocalWorkspace writes this value to the matching Pulumi.<stack>.yaml file in Workspace.WorkDir().
func (l *LocalWorkspace) SetConfigWithOptions(
	ctx context.Context, stackName string, key string, val ConfigValue, opts *ConfigOptions,
) error {
	args := []string{"config", "set", "--stack", stackName}
	if opts != nil {
		if opts.Path {
			args = append(args, "--path")
		}
	}
	secretArg := "--plaintext"
	if val.Secret {
		secretArg = "--secret"
	}
	args = append(args, key, secretArg, "--non-interactive", "--", val.Value)

	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, args...)
	if err != nil {
		return newAutoError(fmt.Errorf("unable to set config: %w", err), stdout, stderr, errCode)
	}
	return nil
}

// SetAllConfig sets all values in the provided config map for the specified stack name.
// LocalWorkspace writes the config to the matching Pulumi.<stack>.yaml file in Workspace.WorkDir().
func (l *LocalWorkspace) SetAllConfig(ctx context.Context, stackName string, config ConfigMap) error {
	return l.SetAllConfigWithOptions(ctx, stackName, config, nil)
}

// SetAllConfigWithOptions sets all values in the provided config map for the specified stack name
// using the optional ConfigOptions.
// LocalWorkspace writes the config to the matching Pulumi.<stack>.yaml file in Workspace.WorkDir().
func (l *LocalWorkspace) SetAllConfigWithOptions(
	ctx context.Context, stackName string, config ConfigMap, opts *ConfigOptions,
) error {
	args := []string{"config", "set-all", "--stack", stackName}
	if opts != nil {
		if opts.Path {
			args = append(args, "--path")
		}
	}
	for k, v := range config {
		secretArg := "--plaintext"
		if v.Secret {
			secretArg = "--secret"
		}
		args = append(args, secretArg, fmt.Sprintf("%s=%s", k, v.Value))
	}

	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, args...)
	if err != nil {
		return newAutoError(fmt.Errorf("unable to set config: %w", err), stdout, stderr, errCode)
	}
	return nil
}

// RemoveConfig removes the specified key-value pair on the provided stack name.
// It will remove any matching values in the Pulumi.<stack>.yaml file in Workspace.WorkDir().
func (l *LocalWorkspace) RemoveConfig(ctx context.Context, stackName string, key string) error {
	return l.RemoveConfigWithOptions(ctx, stackName, key, nil)
}

// RemoveConfigWithOptions removes the specified key-value pair on the provided stack name.
// It will remove any matching values in the Pulumi.<stack>.yaml file in Workspace.WorkDir().
func (l *LocalWorkspace) RemoveConfigWithOptions(
	ctx context.Context, stackName string, key string, opts *ConfigOptions,
) error {
	args := []string{"config", "rm"}
	if opts != nil {
		if opts.Path {
			args = append(args, "--path")
		}
	}
	args = append(args, key, "--stack", stackName)
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, args...)
	if err != nil {
		return newAutoError(fmt.Errorf("could not remove config: %w", err), stdout, stderr, errCode)
	}
	return nil
}

// RemoveAllConfig removes all values in the provided key list for the specified stack name
// It will remove any matching values in the Pulumi.<stack>.yaml file in Workspace.WorkDir().
func (l *LocalWorkspace) RemoveAllConfig(ctx context.Context, stackName string, keys []string) error {
	return l.RemoveAllConfigWithOptions(ctx, stackName, keys, nil)
}

// RemoveAllConfigWithOptions removes all values in the provided key list for the specified stack name
// using the optional ConfigOptions
// It will remove any matching values in the Pulumi.<stack>.yaml file in Workspace.WorkDir().
func (l *LocalWorkspace) RemoveAllConfigWithOptions(
	ctx context.Context, stackName string, keys []string, opts *ConfigOptions,
) error {
	args := []string{"config", "rm-all", "--stack", stackName}
	if opts != nil {
		if opts.Path {
			args = append(args, "--path")
		}
	}
	args = append(args, keys...)
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, args...)
	if err != nil {
		return newAutoError(fmt.Errorf("unable to set config: %w", err), stdout, stderr, errCode)
	}
	return nil
}

// RefreshConfig gets and sets the config map used with the last Update for Stack matching stack name.
// It will overwrite all configuration in the Pulumi.<stack>.yaml file in Workspace.WorkDir().
func (l *LocalWorkspace) RefreshConfig(ctx context.Context, stackName string) (ConfigMap, error) {
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "config", "refresh", "--force", "--stack", stackName)
	if err != nil {
		return nil, newAutoError(fmt.Errorf("could not refresh config: %w", err), stdout, stderr, errCode)
	}

	cfg, err := l.GetAllConfig(ctx, stackName)
	if err != nil {
		return nil, fmt.Errorf("could not fetch config after refresh: %w", err)
	}
	return cfg, nil
}

// GetTag returns the value associated with the specified stack name and key.
func (l *LocalWorkspace) GetTag(ctx context.Context, stackName string, key string) (string, error) {
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "stack", "tag", "get", key, "--stack", stackName)
	if err != nil {
		return stdout, newAutoError(fmt.Errorf("unable to read tag: %w", err), stdout, stderr, errCode)
	}
	return strings.TrimSpace(stdout), nil
}

// SetTag sets the specified key-value pair on the provided stack name.
func (l *LocalWorkspace) SetTag(ctx context.Context, stackName string, key string, value string) error {
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "stack", "tag", "set", key, value, "--stack", stackName)
	if err != nil {
		return newAutoError(fmt.Errorf("unable to set tag: %w", err), stdout, stderr, errCode)
	}
	return nil
}

// RemoveTag removes the specified key-value pair on the provided stack name.
func (l *LocalWorkspace) RemoveTag(ctx context.Context, stackName string, key string) error {
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "stack", "tag", "rm", key, "--stack", stackName)
	if err != nil {
		return newAutoError(fmt.Errorf("could not remove tag: %w", err), stdout, stderr, errCode)
	}
	return nil
}

// ListTags Returns the tag map for the specified stack name.
func (l *LocalWorkspace) ListTags(ctx context.Context, stackName string) (map[string]string, error) {
	var vals map[string]string
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "stack", "tag", "ls", "--json", "--stack", stackName)
	if err != nil {
		return vals, newAutoError(fmt.Errorf("unable to read tags: %w", err), stdout, stderr, errCode)
	}
	err = json.Unmarshal([]byte(stdout), &vals)
	if err != nil {
		return vals, fmt.Errorf("unable to unmarshal tag values: %w", err)
	}
	return vals, nil
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
	return setEnvVars(l, envvars)
}

func setEnvVars(l *LocalWorkspace, envvars map[string]string) error {
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

// PulumiCommand returns the PulumiCommand instance that is used to execute commands.
func (l *LocalWorkspace) PulumiCommand() PulumiCommand {
	return l.pulumiCommand
}

// PulumiHome returns the directory override for CLI metadata if set.
// This customizes the location of $PULUMI_HOME where metadata is stored and plugins are installed.
func (l *LocalWorkspace) PulumiHome() string {
	return l.pulumiHome
}

// PulumiVersion returns the version of the underlying Pulumi CLI/Engine.
func (l *LocalWorkspace) PulumiVersion() string {
	return l.pulumiCommand.Version().String()
}

// WhoAmI returns the currently authenticated user
func (l *LocalWorkspace) WhoAmI(ctx context.Context) (string, error) {
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "whoami")
	if err != nil {
		return "", newAutoError(fmt.Errorf("could not determine authenticated user: %w", err), stdout, stderr, errCode)
	}
	return strings.TrimSpace(stdout), nil
}

// WhoAmIDetails returns detailed information about the currently
// logged-in Pulumi identity.
func (l *LocalWorkspace) WhoAmIDetails(ctx context.Context) (WhoAmIResult, error) {
	// 3.58 added the --json flag (https://github.com/pulumi/pulumi/releases/tag/v3.58.0)
	if l.pulumiCommand.Version().GTE(semver.Version{Major: 3, Minor: 58}) {
		var whoAmIDetailedInfo WhoAmIResult
		stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "whoami", "--json")
		if err != nil {
			return whoAmIDetailedInfo, newAutoError(
				fmt.Errorf("could not retrieve WhoAmIDetailedInfo: %w", err), stdout, stderr, errCode)
		}
		err = json.Unmarshal([]byte(stdout), &whoAmIDetailedInfo)
		if err != nil {
			return whoAmIDetailedInfo, fmt.Errorf("unable to unmarshal WhoAmIDetailedInfo: %w", err)
		}
		return whoAmIDetailedInfo, nil
	}

	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "whoami")
	if err != nil {
		return WhoAmIResult{}, newAutoError(
			fmt.Errorf("could not determine authenticated user: %w", err), stdout, stderr, errCode)
	}
	return WhoAmIResult{User: strings.TrimSpace(stdout)}, nil
}

// Stack returns a summary of the currently selected stack, if any.
func (l *LocalWorkspace) Stack(ctx context.Context) (*StackSummary, error) {
	stacks, err := l.ListStacks(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not determine selected stack: %w", err)
	}
	for _, s := range stacks {
		if s.Current {
			return &s, nil
		}
	}
	return nil, nil
}

// ChangeStackSecretsProvider edits the secrets provider for the given stack.
func (l *LocalWorkspace) ChangeStackSecretsProvider(
	ctx context.Context, stackName, newSecretsProvider string, opts *ChangeSecretsProviderOptions,
) error {
	args := []string{"stack", "change-secrets-provider", "--stack", stackName, newSecretsProvider}

	var reader io.Reader
	if newSecretsProvider == "passphrase" {
		if opts == nil || opts.NewPassphrase == nil {
			return fmt.Errorf("new passphrase must be provided")
		}
		reader = strings.NewReader(*opts.NewPassphrase)
	}
	stdout, stderr, errCode, err := l.runPulumiInputCmdSync(ctx, reader, args...)
	if err != nil {
		return newAutoError(fmt.Errorf("failed to change secrets provider: %w", err), stdout, stderr, errCode)
	}
	return nil
}

// CreateStack creates and sets a new stack with the stack name, failing if one already exists.
func (l *LocalWorkspace) CreateStack(ctx context.Context, stackName string) error {
	args := []string{"stack", "init", stackName}
	if l.secretsProvider != "" {
		args = append(args, "--secrets-provider", l.secretsProvider)
	}
	if l.remote {
		args = append(args, "--no-select")
	}
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, args...)
	if err != nil {
		return newAutoError(fmt.Errorf("failed to create stack: %w", err), stdout, stderr, errCode)
	}

	return nil
}

// SelectStack selects and sets an existing stack matching the stack name, failing if none exists.
func (l *LocalWorkspace) SelectStack(ctx context.Context, stackName string) error {
	// If this is a remote workspace, we don't want to actually select the stack (which would modify global state);
	// but we will ensure the stack exists by calling `pulumi stack`.
	args := []string{"stack"}
	if !l.remote {
		args = append(args, "select")
	}
	args = append(args, "--stack", stackName)

	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, args...)
	if err != nil {
		return newAutoError(fmt.Errorf("failed to select stack: %w", err), stdout, stderr, errCode)
	}

	return nil
}

// RemoveStack deletes the stack and all associated configuration and history.
func (l *LocalWorkspace) RemoveStack(ctx context.Context, stackName string, opts ...optremove.Option) error {
	args := []string{"stack", "rm", "--yes", stackName}

	optRemoveOpts := &optremove.Options{}
	for _, o := range opts {
		o.ApplyOption(optRemoveOpts)
	}

	if optRemoveOpts.Force {
		args = append(args, "--force")
	}

	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, args...)
	if err != nil {
		return newAutoError(fmt.Errorf("failed to remove stack: %w", err), stdout, stderr, errCode)
	}
	return nil
}

// ListStacks returns all Stacks created under the current Project.
// This queries underlying backend and may return stacks not present in the Workspace (as Pulumi.<stack>.yaml files).
func (l *LocalWorkspace) ListStacks(ctx context.Context) ([]StackSummary, error) {
	var stacks []StackSummary
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "stack", "ls", "--json")
	if err != nil {
		return stacks, newAutoError(fmt.Errorf("could not list stacks: %w", err), stdout, stderr, errCode)
	}
	err = json.Unmarshal([]byte(stdout), &stacks)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal config value: %w", err)
	}
	return stacks, nil
}

// InstallPlugin acquires the plugin matching the specified name and version.
func (l *LocalWorkspace) InstallPlugin(ctx context.Context, name string, version string) error {
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "plugin", "install", "resource", name, version)
	if err != nil {
		return newAutoError(fmt.Errorf("failed to install plugin: %w", err), stdout, stderr, errCode)
	}
	return nil
}

// InstallPluginFromServer acquires the plugin matching the specified name and version from a third party server.
func (l *LocalWorkspace) InstallPluginFromServer(
	ctx context.Context,
	name string,
	version string,
	server string,
) error {
	stdout, stderr, errCode, err := l.runPulumiCmdSync(
		ctx, "plugin", "install", "resource", name, version, "--server", server)
	if err != nil {
		return newAutoError(fmt.Errorf("failed to install plugin: %w", err), stdout, stderr, errCode)
	}
	return nil
}

// RemovePlugin deletes the plugin matching the specified name and verision.
func (l *LocalWorkspace) RemovePlugin(ctx context.Context, name string, version string) error {
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "plugin", "rm", "resource", name, version, "--yes")
	if err != nil {
		return newAutoError(fmt.Errorf("failed to remove plugin: %w", err), stdout, stderr, errCode)
	}
	return nil
}

// ListPlugins lists all installed plugins.
func (l *LocalWorkspace) ListPlugins(ctx context.Context) ([]workspace.PluginInfo, error) {
	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "plugin", "ls", "--json")
	if err != nil {
		return nil, newAutoError(fmt.Errorf("could not list list: %w", err), stdout, stderr, errCode)
	}
	var plugins []workspace.PluginInfo
	err = json.Unmarshal([]byte(stdout), &plugins)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal plugin response: %w", err)
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

// ExportStack exports the deployment state of the stack matching the given name.
// This can be combined with ImportStack to edit a stack's state (such as recovery from failed deployments).
func (l *LocalWorkspace) ExportStack(ctx context.Context, stackName string) (apitype.UntypedDeployment, error) {
	var state apitype.UntypedDeployment

	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "stack", "export", "--show-secrets", "--stack", stackName)
	if err != nil {
		return state, newAutoError(fmt.Errorf("could not export stack: %w", err), stdout, stderr, errCode)
	}

	err = json.Unmarshal([]byte(stdout), &state)
	if err != nil {
		return state, newAutoError(
			fmt.Errorf("failed to export stack, unable to unmarshall stack state: %w", err), stdout, stderr, errCode,
		)
	}

	return state, nil
}

// ImportStack imports the specified deployment state into a pre-existing stack.
// This can be combined with ExportStack to edit a stack's state (such as recovery from failed deployments).
func (l *LocalWorkspace) ImportStack(ctx context.Context, stackName string, state apitype.UntypedDeployment) error {
	f, err := os.CreateTemp(os.TempDir(), "")
	if err != nil {
		return fmt.Errorf("could not import stack. failed to allocate temp file: %w", err)
	}
	defer func() { contract.IgnoreError(os.Remove(f.Name())) }()

	bytes, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("could not import stack, failed to marshal stack state: %w", err)
	}

	_, err = f.Write(bytes)
	if err != nil {
		return fmt.Errorf("could not import stack. failed to write out stack intermediate: %w", err)
	}

	stdout, stderr, errCode, err := l.runPulumiCmdSync(ctx, "stack", "import", "--file", f.Name(), "--stack", stackName)
	if err != nil {
		return newAutoError(fmt.Errorf("could not import stack: %w", err), stdout, stderr, errCode)
	}

	return nil
}

// StackOutputs gets the current set of Stack outputs from the last Stack.Up().
func (l *LocalWorkspace) StackOutputs(ctx context.Context, stackName string) (OutputMap, error) {
	// standard outputs
	outStdout, outStderr, code, err := l.runPulumiCmdSync(ctx, "stack", "output", "--json", "--stack", stackName)
	if err != nil {
		return nil, newAutoError(fmt.Errorf("could not get outputs: %w", err), outStdout, outStderr, code)
	}

	// secret outputs
	secretStdout, secretStderr, code, err := l.runPulumiCmdSync(ctx,
		"stack", "output", "--json", "--show-secrets", "--stack", stackName,
	)
	if err != nil {
		return nil, newAutoError(fmt.Errorf("could not get secret outputs: %w", err), outStdout, outStderr, code)
	}

	var outputs map[string]interface{}
	var secrets map[string]interface{}

	if err = json.Unmarshal([]byte(outStdout), &outputs); err != nil {
		return nil, fmt.Errorf("error unmarshalling outputs: %s: %w", secretStderr, err)
	}

	if err = json.Unmarshal([]byte(secretStdout), &secrets); err != nil {
		return nil, fmt.Errorf("error unmarshalling secret outputs: %s: %w", secretStderr, err)
	}

	res := make(OutputMap)
	for k, v := range secrets {
		raw, err := json.Marshal(outputs[k])
		if err != nil {
			return nil, fmt.Errorf("error determining secretness: %s: %w", secretStderr, err)
		}
		rawString := string(raw)
		isSecret := strings.Contains(rawString, secretSentinel)
		res[k] = OutputValue{
			Value:  v,
			Secret: isSecret,
		}
	}

	return res, nil
}

func (l *LocalWorkspace) runPulumiInputCmdSync(
	ctx context.Context,
	stdin io.Reader,
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
	return l.PulumiCommand().Run(ctx,
		l.WorkDir(),
		stdin,
		nil, /* additionalOutputs */
		nil, /* additionalErrorOutputs */
		env,
		args...,
	)
}

func (l *LocalWorkspace) runPulumiCmdSync(
	ctx context.Context,
	args ...string,
) (string, string, int, error) {
	return l.runPulumiInputCmdSync(ctx, nil, args...)
}

// supportsPulumiCmdFlag runs a command with `--help` to see if the specified flag is found within the resulting
// output, in which case we assume the flag is supported.
func (l *LocalWorkspace) supportsPulumiCmdFlag(ctx context.Context, flag string, args ...string) (bool, error) {
	env := []string{
		"PULUMI_DEBUG_COMMANDS=true",
		"PULUMI_EXPERIMENTAL=true",
	}

	// Run the command with `--help`, and then we'll look for the flag in the output.
	stdout, _, _, err := l.PulumiCommand().Run(ctx, l.WorkDir(), nil, nil, nil, env, append(args, "--help")...)
	if err != nil {
		return false, err
	}

	// Does the help test in stdout mention the flag? If so, assume it's supported.
	if strings.Contains(stdout, flag) {
		return true, nil
	}

	return false, nil
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
		dir, err := os.MkdirTemp("", "pulumi_auto")
		if err != nil {
			return nil, fmt.Errorf("unable to create tmp directory for workspace: %w", err)
		}
		workDir = dir
	}

	if lwOpts.Repo != nil && !lwOpts.Remote {
		// now do the git clone
		projDir, err := setupGitRepo(ctx, workDir, lwOpts.Repo)
		if err != nil {
			return nil, fmt.Errorf("failed to create workspace, unable to enlist in git repo: %w", err)
		}
		workDir = projDir
	}

	optOut := env.SkipVersionCheck.Value()
	if val, ok := lwOpts.EnvVars[env.SkipVersionCheck.Var().Name()]; ok {
		optOut = optOut || cmdutil.IsTruthy(val)
	}

	var pulumiCommand PulumiCommand
	if lwOpts.PulumiCommand != nil {
		pulumiCommand = lwOpts.PulumiCommand
	} else {
		p, err := NewPulumiCommand(&PulumiCommandOptions{SkipVersionCheck: optOut})
		if err != nil {
			return nil, err
		}
		pulumiCommand = p
	}

	var program pulumi.RunFunc
	if lwOpts.Program != nil {
		program = lwOpts.Program
	}

	l := &LocalWorkspace{
		workDir:                       workDir,
		preRunCommands:                lwOpts.PreRunCommands,
		program:                       program,
		pulumiHome:                    lwOpts.PulumiHome,
		remote:                        lwOpts.Remote,
		remoteEnvVars:                 lwOpts.RemoteEnvVars,
		remoteSkipInstallDependencies: lwOpts.RemoteSkipInstallDependencies,
		repo:                          lwOpts.Repo,
		pulumiCommand:                 pulumiCommand,
	}

	// If remote was specified, ensure the CLI supports it.
	if !optOut && l.remote {
		// See if `--remote` is present in `pulumi preview --help`'s output.
		supportsRemote, err := l.supportsPulumiCmdFlag(ctx, "--remote", "preview")
		if err != nil {
			return nil, err
		}
		if !supportsRemote {
			return nil, errors.New("Pulumi CLI does not support remote operations; please upgrade")
		}
	}

	if lwOpts.Project != nil {
		err := l.SaveProjectSettings(ctx, lwOpts.Project)
		if err != nil {
			return nil, fmt.Errorf("failed to create workspace, unable to save project settings: %w", err)
		}
	}

	for stackName := range lwOpts.Stacks {
		s := lwOpts.Stacks[stackName]
		err := l.SaveStackSettings(ctx, stackName, &s)
		if err != nil {
			return nil, fmt.Errorf("failed to create workspace: %w", err)
		}
	}

	// setup
	if !lwOpts.Remote && lwOpts.Repo != nil && lwOpts.Repo.Setup != nil {
		err := lwOpts.Repo.Setup(ctx, l)
		if err != nil {
			return nil, fmt.Errorf("error while running setup function: %w", err)
		}
	}

	// Secrets providers
	if lwOpts.SecretsProvider != "" {
		l.secretsProvider = lwOpts.SecretsProvider
	}

	// Environment values
	if lwOpts.EnvVars != nil {
		if err := setEnvVars(l, lwOpts.EnvVars); err != nil {
			return nil, fmt.Errorf("failed to set environment values: %w", err)
		}
	}

	return l, nil
}

// EnvVarValue represents the value of an envvar. A value can be a secret, which is passed along
// to remote operations when used with remote workspaces, otherwise, it has no affect.
type EnvVarValue struct {
	Value  string
	Secret bool
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
	// PulumiCommand is the PulumiCommand instance to use. If none is
	// supplied, the workspace will create an instance using the PulumiCommand
	// CLI found in $PATH.
	PulumiCommand PulumiCommand
	// Project is the project settings for the workspace.
	Project *workspace.Project
	// Stacks is a map of [stackName -> stack settings objects] to seed the workspace.
	Stacks map[string]workspace.ProjectStack
	// Repo is a git repo with a Pulumi Project to clone into the WorkDir.
	Repo *GitRepo
	// Secrets Provider to use with the current Stack
	SecretsProvider string
	// EnvVars is a map of environment values scoped to the workspace.
	// These values will be passed to all Workspace and Stack level commands.
	EnvVars map[string]string
	// Whether the workspace represents a remote workspace.
	Remote bool
	// Remote environment variables to be passed to the remote Pulumi operation.
	RemoteEnvVars map[string]EnvVarValue
	// PreRunCommands is an optional list of arbitrary commands to run before the remote Pulumi operation is invoked.
	PreRunCommands []string
	// RemoteSkipInstallDependencies sets whether to skip the default dependency installation step
	RemoteSkipInstallDependencies bool
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
	// Specifying this option will update the Workspace's WorkDir accordingly.
	ProjectPath string
	// Optional branch to checkout.
	Branch string
	// Optional commit to checkout.
	CommitHash string
	// Optional function to execute after enlisting in the specified repo.
	Setup SetupFn
	// GitAuth is the different Authentication options for the Git repository
	Auth *GitAuth
	// Shallow disables fetching the repo's entire history.
	Shallow bool
}

// GitAuth is the authentication details that can be specified for a private Git repo.
// There are 3 different authentication paths:
// * PersonalAccessToken
// * SSHPrivateKeyPath (and it's potential password)
// * Username and Password
// Only 1 authentication path is valid. If more than 1 is specified it will result in an error
type GitAuth struct {
	// The absolute path to a private key for access to the git repo
	// When using `SSHPrivateKeyPath`, the URL of the repository must be in the format
	// git@github.com:org/repository.git - if the url is not in this format, then an error
	// `unable to clone repo: invalid auth method` will be returned
	SSHPrivateKeyPath string
	// The (contents) private key for access to the git repo.
	// When using `SSHPrivateKey`, the URL of the repository must be in the format
	// git@github.com:org/repository.git - if the url is not in this format, then an error
	// `unable to clone repo: invalid auth method` will be returned
	SSHPrivateKey string
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

// PulumiCommand is the PulumiCommand instance to use. If none is
// supplied, the workspace will create an instance using the PulumiCommand
// CLI found in $PATH.
func Pulumi(pulumi PulumiCommand) LocalWorkspaceOption {
	return localWorkspaceOption(func(lo *localWorkspaceOptions) {
		lo.PulumiCommand = pulumi
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

// EnvVars is a map of environment values scoped to the workspace.
// These values will be passed to all Workspace and Stack level commands.
func EnvVars(envvars map[string]string) LocalWorkspaceOption {
	return localWorkspaceOption(func(lo *localWorkspaceOptions) {
		lo.EnvVars = envvars
	})
}

// remoteEnvVars is a map of environment values scoped to the workspace.
// These values will be passed to the remote Pulumi operation.
func remoteEnvVars(envvars map[string]EnvVarValue) LocalWorkspaceOption {
	return localWorkspaceOption(func(lo *localWorkspaceOptions) {
		lo.RemoteEnvVars = envvars
	})
}

// remote is set on the local workspace to indicate it is actually a remote workspace.
func remote(remote bool) LocalWorkspaceOption {
	return localWorkspaceOption(func(lo *localWorkspaceOptions) {
		lo.Remote = remote
	})
}

// preRunCommands is an optional list of arbitrary commands to run before the remote Pulumi operation is invoked.
func preRunCommands(commands ...string) LocalWorkspaceOption {
	return localWorkspaceOption(func(lo *localWorkspaceOptions) {
		lo.PreRunCommands = commands
	})
}

// remoteSkipInstallDependencies sets whether to skip the default dependency installation step.
func remoteSkipInstallDependencies(skipInstallDependencies bool) LocalWorkspaceOption {
	return localWorkspaceOption(func(lo *localWorkspaceOptions) {
		lo.RemoteSkipInstallDependencies = skipInstallDependencies
	})
}

// NewStackLocalSource creates a Stack backed by a LocalWorkspace created on behalf of the user,
// from the specified WorkDir. This Workspace will pick up
// any available Settings files (Pulumi.yaml, Pulumi.<stack>.yaml).
func NewStackLocalSource(ctx context.Context, stackName, workDir string, opts ...LocalWorkspaceOption) (Stack, error) {
	opts = append(opts, WorkDir(workDir))
	w, err := NewLocalWorkspace(ctx, opts...)
	var stack Stack
	if err != nil {
		return stack, fmt.Errorf("failed to create stack: %w", err)
	}

	return NewStack(ctx, stackName, w)
}

// UpsertStackLocalSource creates a Stack backed by a LocalWorkspace created on behalf of the user,
// from the specified WorkDir. If the Stack already exists, it will not error
// and proceed to selecting the Stack.This Workspace will pick up any available
// Settings files (Pulumi.yaml, Pulumi.<stack>.yaml).
func UpsertStackLocalSource(
	ctx context.Context,
	stackName,
	workDir string,
	opts ...LocalWorkspaceOption,
) (Stack, error) {
	opts = append(opts, WorkDir(workDir))
	w, err := NewLocalWorkspace(ctx, opts...)
	var stack Stack
	if err != nil {
		return stack, fmt.Errorf("failed to create stack: %w", err)
	}

	return UpsertStack(ctx, stackName, w)
}

// SelectStackLocalSource selects an existing Stack backed by a LocalWorkspace created on behalf of the user,
// from the specified WorkDir. This Workspace will pick up
// any available Settings files (Pulumi.yaml, Pulumi.<stack>.yaml).
func SelectStackLocalSource(
	ctx context.Context,
	stackName,
	workDir string,
	opts ...LocalWorkspaceOption,
) (Stack, error) {
	opts = append(opts, WorkDir(workDir))
	w, err := NewLocalWorkspace(ctx, opts...)
	var stack Stack
	if err != nil {
		return stack, fmt.Errorf("failed to select stack: %w", err)
	}

	return SelectStack(ctx, stackName, w)
}

// NewStackRemoteSource creates a Stack backed by a LocalWorkspace created on behalf of the user,
// with source code cloned from the specified GitRepo. This Workspace will pick up
// any available Settings files (Pulumi.yaml, Pulumi.<stack>.yaml) that are cloned into the Workspace.
// Unless a WorkDir option is specified, the GitRepo will be clone into a new temporary directory provided by the OS.
func NewStackRemoteSource(
	ctx context.Context,
	stackName string,
	repo GitRepo,
	opts ...LocalWorkspaceOption,
) (Stack, error) {
	opts = append(opts, Repo(repo))
	w, err := NewLocalWorkspace(ctx, opts...)
	var stack Stack
	if err != nil {
		return stack, fmt.Errorf("failed to create stack: %w", err)
	}

	return NewStack(ctx, stackName, w)
}

// UpsertStackRemoteSource creates a Stack backed by a LocalWorkspace created on behalf of the user,
// with source code cloned from the specified GitRepo. If the Stack already exists,
// it will not error and proceed to selecting the Stack. This Workspace will pick up
// any available Settings files (Pulumi.yaml, Pulumi.<stack>.yaml) that are cloned
// into the Workspace. Unless a WorkDir option is specified, the GitRepo will be clone
// into a new temporary directory provided by the OS.
func UpsertStackRemoteSource(
	ctx context.Context, stackName string, repo GitRepo, opts ...LocalWorkspaceOption,
) (Stack, error) {
	opts = append(opts, Repo(repo))
	w, err := NewLocalWorkspace(ctx, opts...)
	var stack Stack
	if err != nil {
		return stack, fmt.Errorf("failed to create stack: %w", err)
	}

	return UpsertStack(ctx, stackName, w)
}

// SelectStackRemoteSource selects an existing Stack backed by a LocalWorkspace created on behalf of the user,
// with source code cloned from the specified GitRepo. This Workspace will pick up
// any available Settings files (Pulumi.yaml, Pulumi.<stack>.yaml) that are cloned into the Workspace.
// Unless a WorkDir option is specified, the GitRepo will be clone into a new temporary directory provided by the OS.
func SelectStackRemoteSource(
	ctx context.Context,
	stackName string, repo GitRepo,
	opts ...LocalWorkspaceOption,
) (Stack, error) {
	opts = append(opts, Repo(repo))
	w, err := NewLocalWorkspace(ctx, opts...)
	var stack Stack
	if err != nil {
		return stack, fmt.Errorf("failed to select stack: %w", err)
	}

	return SelectStack(ctx, stackName, w)
}

// NewStackInlineSource creates a Stack backed by a LocalWorkspace created on behalf of the user,
// with the specified program. If no Project option is specified, default project settings will be created
// on behalf of the user. Similarly, unless a WorkDir option is specified, the working directory will default
// to a new temporary directory provided by the OS.
func NewStackInlineSource(
	ctx context.Context,
	stackName string,
	projectName string,
	program pulumi.RunFunc,
	opts ...LocalWorkspaceOption,
) (Stack, error) {
	var stack Stack
	opts = append(opts, Program(program))

	proj, err := getProjectSettings(ctx, projectName, opts)
	if err != nil {
		return stack, err
	}
	if proj != nil {
		opts = append(opts, Project(*proj))
	}

	w, err := NewLocalWorkspace(ctx, opts...)
	if err != nil {
		return stack, fmt.Errorf("failed to create stack: %w", err)
	}

	return NewStack(ctx, stackName, w)
}

// UpsertStackInlineSource creates a Stack backed by a LocalWorkspace created on behalf of the user,
// with the specified program. If the Stack already exists, it will not error and
// proceed to selecting the Stack. If no Project option is specified, default project
// settings will be created on behalf of the user. Similarly, unless a WorkDir option
// is specified, the working directory will default to a new temporary directory provided by the OS.
func UpsertStackInlineSource(
	ctx context.Context,
	stackName string,
	projectName string,
	program pulumi.RunFunc,
	opts ...LocalWorkspaceOption,
) (Stack, error) {
	var stack Stack
	opts = append(opts, Program(program))

	proj, err := getProjectSettings(ctx, projectName, opts)
	if err != nil {
		return stack, err
	}
	if proj != nil {
		opts = append(opts, Project(*proj))
	}

	w, err := NewLocalWorkspace(ctx, opts...)
	if err != nil {
		return stack, fmt.Errorf("failed to create stack: %w", err)
	}

	return UpsertStack(ctx, stackName, w)
}

// SelectStackInlineSource selects an existing Stack backed by a new LocalWorkspace created on behalf of the user,
// with the specified program. If no Project option is specified, default project settings will be created
// on behalf of the user. Similarly, unless a WorkDir option is specified, the working directory will default
// to a new temporary directory provided by the OS.
func SelectStackInlineSource(
	ctx context.Context,
	stackName string,
	projectName string,
	program pulumi.RunFunc,
	opts ...LocalWorkspaceOption,
) (Stack, error) {
	var stack Stack
	opts = append(opts, Program(program))

	proj, err := getProjectSettings(ctx, projectName, opts)
	if err != nil {
		return stack, err
	}
	if proj != nil {
		opts = append(opts, Project(*proj))
	}

	w, err := NewLocalWorkspace(ctx, opts...)
	if err != nil {
		return stack, fmt.Errorf("failed to select stack: %w", err)
	}

	return SelectStack(ctx, stackName, w)
}

func defaultInlineProject(projectName string) (workspace.Project, error) {
	var proj workspace.Project
	cwd, err := os.Getwd()
	if err != nil {
		return proj, err
	}
	proj = workspace.Project{
		Name:    tokens.PackageName(projectName),
		Runtime: workspace.NewProjectRuntimeInfo("go", nil),
		Main:    cwd,
	}

	return proj, nil
}

// stack names come in many forms:
// s, o/p/s, u/p/s o/s
// so just return the last chunk which is what will be used in pulumi.<stack>.yaml
func getStackSettingsName(stackName string) string {
	parts := strings.Split(stackName, "/")
	if len(parts) < 1 {
		return stackName
	}
	return parts[len(parts)-1]
}

const pulumiHomeEnv = "PULUMI_HOME"

func readProjectSettingsFromDir(ctx context.Context, workDir string) (*workspace.Project, error) {
	for _, ext := range settingsExtensions {
		projectPath := filepath.Join(workDir, "Pulumi"+ext)
		if _, err := os.Stat(projectPath); err == nil {
			proj, err := workspace.LoadProject(projectPath)
			if err != nil {
				return nil, fmt.Errorf("found project settings, but failed to load: %w", err)
			}
			return proj, nil
		}
	}
	return nil, errors.New("unable to find project settings in workspace")
}

func getProjectSettings(
	ctx context.Context,
	projectName string,
	opts []LocalWorkspaceOption,
) (*workspace.Project, error) {
	var optsBag localWorkspaceOptions
	for _, opt := range opts {
		opt.applyLocalWorkspaceOption(&optsBag)
	}

	// If the Project is included in the opts, just use that.
	if optsBag.Project != nil {
		return optsBag.Project, nil
	}

	// If WorkDir is specified, try to read any existing project settings before resorting to
	// creating a default project.
	if optsBag.WorkDir != "" {
		_, err := readProjectSettingsFromDir(ctx, optsBag.WorkDir)
		if err == nil {
			return nil, nil
		}

		if err.Error() == "unable to find project settings in workspace" {
			proj, err := defaultInlineProject(projectName)
			if err != nil {
				return nil, fmt.Errorf("failed to create default project: %w", err)
			}
			return &proj, nil
		}

		return nil, fmt.Errorf("failed to load project settings: %w", err)
	}

	// If there was no workdir specified, create the default project.
	proj, err := defaultInlineProject(projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to create default project: %w", err)
	}
	return &proj, nil
}
