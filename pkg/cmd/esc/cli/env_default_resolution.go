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

package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// escConfigFile is the name of the file that declares a default environment for a directory tree.
const escConfigFile = ".esc.yaml"

// defaultEnvironment describes an environment inferred from the working directory. It is either a
// single named environment (ref != nil) or an anonymous list of imports opened in orgName.
type defaultEnvironment struct {
	// ref is the referenced environment, if the default names a single environment.
	ref *environmentRef

	// orgName is the organization the anonymous import list is opened in. Only set if ref is nil.
	orgName string
	// imports is the anonymous list of imports. Only set if ref is nil.
	imports []string

	// source describes where the default came from: a path to a .esc.yaml file or "stack <name>".
	source string
}

// definition returns the YAML definition for an anonymous import list default.
func (d *defaultEnvironment) definition() []byte {
	def, err := yaml.Marshal(map[string]any{"imports": d.imports})
	if err != nil {
		// yaml.Marshal of a map of strings cannot fail.
		return nil
	}
	return def
}

// target returns the default environment as a command target.
func (d *defaultEnvironment) target() envTarget {
	if d.ref != nil {
		return envTarget{ref: *d.ref}
	}
	return envTarget{anon: d}
}

// description renders the default environment in .esc.yaml syntax.
func (d *defaultEnvironment) description() string {
	if d.ref != nil {
		return fmt.Sprintf("environment: %v\n", d.ref.String())
	}

	var s strings.Builder
	s.WriteString("environment:\n")
	if d.orgName != "" {
		fmt.Fprintf(&s, "  organization: %v\n", d.orgName)
	}
	s.WriteString("  imports:\n")
	for _, imp := range d.imports {
		fmt.Fprintf(&s, "    - %v\n", imp)
	}
	return s.String()
}

// resolveDefaultEnvironment infers the default environment for the current working directory. It
// returns (nil, nil) if no default is configured. Errors are only returned for malformed files:
// the absence of a .esc.yaml, a Pulumi project, a selected stack, or stack config is never an
// error.
func (cmd *envCommand) resolveDefaultEnvironment(ctx context.Context) (*defaultEnvironment, error) {
	if cmd.defaultEnvOnce {
		return cmd.defaultEnv, cmd.defaultEnvErr
	}
	cmd.defaultEnvOnce = true
	cmd.defaultEnv, cmd.defaultEnvErr = cmd.inferDefaultEnvironment(ctx)
	return cmd.defaultEnv, cmd.defaultEnvErr
}

func (cmd *envCommand) inferDefaultEnvironment(ctx context.Context) (*defaultEnvironment, error) {
	cwd, err := cmd.esc.wd.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}

	// The nearest .esc.yaml wins, if there is one.
	if path, contents, ok := cmd.findNearestFile(cwd, escConfigFile); ok {
		def, err := cmd.parseESCConfigFile(ctx, path, contents)
		if err != nil || def != nil {
			return def, err
		}
		// A .esc.yaml without an `environment` key is inert: fall through to the stack source.
	}

	return cmd.resolveStackDefaultEnvironment(cwd)
}

// findNearestFile searches dir and each of its parents for a file with one of the given names,
// returning the path and contents of the first one it finds.
func (cmd *envCommand) findNearestFile(dir string, names ...string) (string, []byte, bool) {
	for {
		for _, name := range names {
			path := filepath.Join(dir, name)
			if contents, ok := cmd.readFile(path); ok {
				return path, contents, true
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil, false
		}
		dir = parent
	}
}

// readFile reads a file through the injectable filesystem. Paths are normalized to forward slashes
// because io/fs (and therefore the test harness's MapFS) only understands slash-separated paths;
// os.ReadFile accepts them on every supported platform.
func (cmd *envCommand) readFile(path string) ([]byte, bool) {
	contents, err := cmd.esc.fs.ReadFile(filepath.ToSlash(path))
	if err != nil {
		return nil, false
	}
	return contents, true
}

// escConfigEnvironment is the object form of a .esc.yaml `environment` field.
type escConfigEnvironment struct {
	Organization string   `yaml:"organization,omitempty"`
	Imports      []string `yaml:"imports,omitempty"`
}

// parseESCConfigFile parses a .esc.yaml file. It returns (nil, nil) if the file does not declare
// an environment.
func (cmd *envCommand) parseESCConfigFile(
	ctx context.Context,
	path string,
	contents []byte,
) (*defaultEnvironment, error) {
	invalid := func(err error) error {
		return fmt.Errorf("invalid %v: %w", path, err)
	}

	var config struct {
		Environment yaml.Node `yaml:"environment"`
	}
	if err := yaml.Unmarshal(contents, &config); err != nil {
		return nil, invalid(err)
	}

	node := config.Environment
	switch {
	case node.Kind == 0 || node.Tag == "!!null":
		// No environment declared.
		return nil, nil
	case node.Kind == yaml.ScalarNode && node.Tag == "!!str":
		ref, err := cmd.getExistingEnvRefWithRelative(ctx, node.Value, nil)
		if err != nil {
			return nil, err
		}
		return &defaultEnvironment{ref: &ref, source: path}, nil
	case node.Kind == yaml.MappingNode:
		var env escConfigEnvironment
		if err := node.Decode(&env); err != nil {
			return nil, invalid(err)
		}
		if len(env.Imports) == 0 {
			return nil, invalid(errors.New("the 'environment' object must specify at least one import"))
		}
		orgName := env.Organization
		if orgName == "" {
			orgName = cmd.esc.account.DefaultOrg
		}
		return &defaultEnvironment{orgName: orgName, imports: env.Imports, source: path}, nil
	default:
		return nil, invalid(errors.New("the 'environment' field must be an environment reference or an " +
			"object with 'organization' and 'imports' fields"))
	}
}

// resolveStackDefaultEnvironment infers a default environment from the currently-selected Pulumi
// stack. This is best effort: no project, no selected stack, no stack config, or no environment
// imports all resolve to no default. Malformed project or stack files are errors.
//
// Note that stacks whose configuration is stored in the Pulumi Cloud (rather than in a
// Pulumi.<stack>.yaml file) do not resolve to a default: reading their linked environment would
// require Pulumi Cloud stack APIs that the ESC client does not expose.
func (cmd *envCommand) resolveStackDefaultEnvironment(cwd string) (*defaultEnvironment, error) {
	projectPath, projectBytes, ok := cmd.findNearestFile(cwd, "Pulumi.yaml", "Pulumi.yml")
	if !ok {
		return nil, nil
	}

	project, err := workspace.LoadProjectBytes(projectBytes, projectPath, encoding.YAML)
	if err != nil {
		return nil, fmt.Errorf("invalid %v: %w", projectPath, err)
	}

	projectDir := filepath.Dir(projectPath)

	// Ask the Pulumi workspace which stack is selected. A workspace that cannot be loaded (most
	// commonly because there is no project state on disk yet) means there is no selected stack,
	// which is not an error.
	w, err := cmd.esc.ws.New(projectDir)
	if err != nil {
		return nil, nil
	}
	stackName := w.Settings().Stack
	if stackName == "" {
		return nil, nil
	}

	stackPath, stackBytes, ok := cmd.findStackConfigFile(projectDir, stackName)
	if !ok {
		return nil, nil
	}

	stack, err := workspace.LoadProjectStackBytes(nil, project, stackBytes, stackPath, encoding.YAML)
	if err != nil {
		return nil, fmt.Errorf("invalid %v: %w", stackPath, err)
	}
	if stack.Environment == nil {
		return nil, nil
	}

	imports := stackEnvironmentImports(stack.Environment)
	if len(imports) == 0 {
		return nil, nil
	}

	return &defaultEnvironment{
		orgName: cmd.esc.account.DefaultOrg,
		imports: imports,
		source:  "stack " + stackName,
	}, nil
}

// findStackConfigFile returns the Pulumi.<stack>.yaml file for the given stack, if it exists. Note
// that unlike .esc.yaml and Pulumi.yaml, stack config lives next to the project file rather than
// anywhere up the directory tree.
func (cmd *envCommand) findStackConfigFile(projectDir, stackName string) (string, []byte, bool) {
	for _, ext := range []string{".yaml", ".yml"} {
		path := filepath.Join(projectDir, "Pulumi."+stackName+ext)
		if contents, ok := cmd.readFile(path); ok {
			return path, contents, true
		}
	}
	return "", nil, false
}

// stackEnvironmentImports returns the environments imported by a stack's `environment` block.
//
// Environment.Imports appends a "yaml" sentinel when the block also contains inline values.
// Projecting those inline values is out of scope, so we drop the sentinel and build the default
// from the real imports only.
func stackEnvironmentImports(env *workspace.Environment) []string {
	imports := env.Imports()
	if n := len(imports); n != 0 && imports[n-1] == "yaml" {
		imports = imports[:n-1]
	}
	return imports
}
