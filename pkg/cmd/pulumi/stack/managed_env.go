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

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

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
	if loc.ManagedEnvironmentMissing {
		return nil, fmt.Errorf("environment %s managed by stack %s was deleted. %s",
			loc.ManagedEnvironment, stackName, ManagedEnvironmentDeletedHint(loc.ManagedEnvironment, stackName))
	}

	node, err := ps.Environment.DefinitionNode()
	if err == nil {
		return node, nil
	}
	stackFile := stackConfigFilePath(stackName, configFile)
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
		"  environment:\n"+
		"    values:\n"+
		"      pulumiConfig: {}",
		org, managedEnv, stackConfigFilePath(stack, configFile))
}

// ManagedEnvironmentNotSupportedError is the error for a command or flag that does not work on a stack with an
// ESC-managed environment.
func ManagedEnvironmentNotSupportedError(what string) error {
	return fmt.Errorf("%s is not supported on stacks with an ESC-managed environment yet", what)
}

// ManagedEnvironmentDeletedHint tells the user how to recover a managed stack whose environment was deleted.
func ManagedEnvironmentDeletedHint(managedEnv string, stack string) string {
	return fmt.Sprintf("Restore it in ESC (Enterprise). A new environment named %s can't decrypt "+
		"the existing secrets in stack %s's config.", managedEnv, stack)
}

func stackConfigFilePath(stack string, configFile string) string {
	if configFile != "" {
		return configFile
	}
	if _, path, err := workspace.DetectProjectStackPath(tokens.QName(stack)); err == nil {
		return path
	}
	return fmt.Sprintf("Pulumi.%s.yaml", stack)
}

// A JSON stack file whose environment is a list or absent doesn't yield ErrJSONManagedDefinition.
func isJSONStackFile(path string, definitionErr error) bool {
	return errors.Is(definitionErr, workspace.ErrJSONManagedDefinition) || filepath.Ext(path) == encoding.JSONExt
}
