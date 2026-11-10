// Copyright 2026, Pulumi Corporation.
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

package stack

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// SetEachConfigValueHint tells the user how to write config values into a managed stack's environment.
const SetEachConfigValueHint = "set each value with `pulumi config set` (`--secret` for secrets)"

var errManagedEnvironmentRequiresYAML = errors.New("stack-managed environments require a YAML stack file")

// ManagedEnvironmentDefinition returns the definition node backing ps.Environment, or an error with a recovery hint.
// stackName is the short name; an empty configFile (the --config-file value) is detected from the working directory.
func ManagedEnvironmentDefinition(
	loc backend.StackConfigLocation,
	ps *workspace.ProjectStack,
	org string,
	stackName string,
	configFile string,
) (*yaml.Node, error) {
	node, err := ps.Environment.DefinitionNode()
	if err == nil && len(ps.Config) == 0 {
		return node, nil
	}
	stackFile := stackConfigFilePath(stackName, configFile)
	if err == nil {
		return nil, fmt.Errorf("%s has values under `config:`, which stacks with an ESC-managed environment "+
			"don't use. Remove the `config:` section, then %s", stackFile, SetEachConfigValueHint)
	}
	if isJSONStackFile(stackFile, err) {
		return nil, errManagedEnvironmentRequiresYAML
	}
	return nil, fmt.Errorf("%s has no definition for managed environment %s/%s. %s",
		stackFile, org, loc.ManagedEnvironment,
		ManagedEnvironmentMissingFromFileHint(org, loc.ManagedEnvironment, stackName, stackFile))
}

// ManagedEnvironmentMissingFromFileHint tells the user how to restore the definition to the stack file. configFile is
// resolved as in ManagedEnvironmentDefinition.
func ManagedEnvironmentMissingFromFileHint(
	org string,
	managedEnv string,
	stack string,
	configFile string,
) string {
	return fmt.Sprintf("Restore it with `pulumi env get %s/%s --definition` and put the output under "+
		"`environment:` in %s. "+
		"For a stack that has never been updated, this is enough:\n\n"+
		"  environment:\n%s",
		org, managedEnv, stackConfigFilePath(stack, configFile),
		"    "+strings.ReplaceAll(managedEnvironmentPlaceholder, "\n", "\n    "))
}

// RejectConfigFlagsOnManagedStack returns an error when --config or --config-file is set on a stack with an
// ESC-managed environment, whose config lives in the environment instead.
func RejectConfigFlagsOnManagedStack(s backend.Stack, configArray []string, configFile string) error {
	if s.ConfigLocation().ManagedEnvironment == "" {
		return nil
	}
	if len(configArray) != 0 {
		return ManagedEnvironmentNotSupportedError("--config")
	}
	if configFile != "" {
		return ManagedEnvironmentNotSupportedError("--config-file")
	}
	return nil
}

// RejectManagedStack returns ManagedEnvironmentNotSupportedError(what) when s has an ESC-managed environment.
func RejectManagedStack(s backend.Stack, what string) error {
	if s.ConfigLocation().ManagedEnvironment != "" {
		return ManagedEnvironmentNotSupportedError(what)
	}
	return nil
}

// ManagedEnvironmentNotSupportedError is the error for a command or flag that does not work on a stack with an
// ESC-managed environment.
func ManagedEnvironmentNotSupportedError(what string) error {
	return fmt.Errorf("%s is not supported on stacks with an ESC-managed environment yet", what)
}

func resolveStackFilePath(stack tokens.QName, configFile string) (string, error) {
	if configFile != "" {
		return configFile, nil
	}
	_, path, err := workspace.DetectProjectStackPath(stack)
	if err != nil {
		return "", fmt.Errorf("could not detect project stack path: %w", err)
	}
	return path, nil
}

func stackConfigFilePath(stack string, configFile string) string {
	if path, err := resolveStackFilePath(tokens.QName(stack), configFile); err == nil {
		return path
	}
	return fmt.Sprintf("Pulumi.%s.yaml", stack)
}

func managedStackFilePath(stackName tokens.QName, configFile string) (string, error) {
	path, err := resolveStackFilePath(stackName, configFile)
	if err != nil {
		return "", err
	}
	if ext := filepath.Ext(path); ext != encoding.YAMLExt && ext != ".yml" {
		return "", errManagedEnvironmentRequiresYAML
	}
	return path, nil
}

const managedEnvironmentPlaceholder = "values:\n  pulumiConfig: {}"

func newManagedEnvironmentPlaceholder() *workspace.Environment {
	var env workspace.Environment
	// values stays block style: yamlutil restyles only an empty flow mapping that gains content.
	err := yaml.Unmarshal([]byte(managedEnvironmentPlaceholder), &env)
	contract.AssertNoErrorf(err, "parsing the managed environment placeholder")
	return &env
}

// A JSON stack file whose environment is a list or absent doesn't yield ErrJSONManagedDefinition.
func isJSONStackFile(path string, definitionErr error) bool {
	return errors.Is(definitionErr, workspace.ErrJSONManagedDefinition) || filepath.Ext(path) == encoding.JSONExt
}
