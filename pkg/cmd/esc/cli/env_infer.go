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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// dotESC is the parsed shape of a .esc.yaml file. See `esc env --help` for the user-facing schema.
type dotESC struct {
	Environment *dotESCEnv `yaml:"environment,omitempty"`
}

// dotESCEnv decodes from a string or an organization+imports object. The UnmarshalYAML method
// picks the form based on the shape of the value; exactly one of the two fields will be set.
type dotESCEnv struct {
	Environment string
	Imports     *dotESCImports
}

type dotESCImports struct {
	Organization string   `yaml:"organization"`
	Imports      []string `yaml:"imports"`
}

func (e *dotESCEnv) UnmarshalYAML(n *yaml.Node) error {
	stringErr := n.Decode(&e.Environment)
	if stringErr == nil {
		return nil
	}

	var imports dotESCImports
	importsErr := n.Decode(&imports)
	if importsErr == nil && (imports.Organization != "" || len(imports.Imports) != 0) {
		e.Imports = &imports
		return nil
	}

	return errors.Join(stringErr, importsErr)
}

// inferFSEnv walks up from startDir looking for a .esc.yaml that names a default environment.
// A file that has not been accepted by the user is ignored (see checkDotESCTrust). The returned
// source is the path of the .esc.yaml the default came from, relative to startDir.
func (cmd *envCommand) inferFSEnv(startDir string) (environmentDesc, string, error) {
	contents, path, err := cmd.findDotESC(startDir)
	if err != nil || contents == nil {
		return nil, "", err
	}

	var dotESC dotESC
	if err := yaml.Unmarshal(contents, &dotESC); err != nil {
		return nil, "", fmt.Errorf("decoding %v: %w", path, err)
	}
	if dotESC.Environment == nil {
		return nil, "", nil
	}

	source := relativePath(startDir, path)
	if ok, err := cmd.checkDotESCTrust(path, source, contents); err != nil || !ok {
		return nil, "", err
	}

	switch {
	case dotESC.Environment.Environment != "":
		return cmd.parseRef(dotESC.Environment.Environment), source, nil
	case dotESC.Environment.Imports != nil:
		return importList{
			orgName: dotESC.Environment.Imports.Organization,
			imports: dotESC.Environment.Imports.Imports,
		}, source, nil
	default:
		return nil, "", nil
	}
}

// findDotESC walks up from startDir and returns the contents and path of the first .esc.yaml it
// finds.
func (cmd *envCommand) findDotESC(startDir string) ([]byte, string, error) {
	dir := startDir
	for {
		path := filepath.Join(dir, ".esc.yaml")
		contents, err := cmd.esc.fs.ReadFile(path)
		if err == nil {
			return contents, path, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, "", fmt.Errorf("reading %v: %w", path, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, "", nil
		}
		dir = parent
	}
}

// relativePath returns path relative to base for use in messages, falling back to path itself.
func relativePath(base, path string) string {
	if rel, err := filepath.Rel(base, path); err == nil {
		return rel
	}
	return path
}

// inferPulumiIaCEnv resolves the imports of the currently-selected Pulumi IaC stack as an
// anonymous environment. Absent context--no project, no selected stack, no imports--resolves to
// nil rather than an error; malformed project or stack files are errors. The returned source
// names the stack and its config file.
func (cmd *envCommand) inferPulumiIaCEnv(cwd string) (environmentDesc, string, error) {
	project, root, err := cmd.esc.ws.ReadProject(cwd)
	if err != nil {
		if errors.Is(err, workspace.ErrProjectNotFound) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("reading Pulumi project: %w", err)
	}

	fullStackName, err := state.CurrentStackName(cmd.esc.ws, cwd)
	if err != nil {
		return nil, "", fmt.Errorf("reading current stack: %w", err)
	}
	if fullStackName == "" {
		return nil, "", nil
	}

	// The stored stack name is fully qualified (see state.SetCurrentStack), but settings written
	// by older CLIs may elide the owner. The CLI proper qualifies such names against a
	// backend.Backend (see state.CurrentStackAt); we have no backend, so fall back to the
	// account's default org, which lookupDefaultOrg resolves the same way.
	orgName, stackName := cmd.esc.account.DefaultOrg, fullStackName
	if parts := strings.Split(fullStackName, "/"); len(parts) > 1 {
		orgName, stackName = parts[0], parts[len(parts)-1]
	}
	if orgName == "" {
		return nil, "", fmt.Errorf("could not determine organization for stack %q", stackName)
	}

	sink := diag.DefaultSink(io.Discard, cmd.esc.stderr, diag.FormatOptions{Color: cmd.esc.colors})
	ps, configPath, err := cmd.esc.ws.ReadProjectStack(sink, project, root, stackName)
	if err != nil {
		return nil, "", fmt.Errorf("loading stack config: %w", err)
	}
	if ps.Environment == nil {
		return nil, "", nil
	}

	// Imports appends a "yaml" sentinel when the environment block has inline values; it is not
	// an importable environment.
	imports := slices.DeleteFunc(ps.Environment.Imports(), func(s string) bool { return s == "yaml" })
	if len(imports) == 0 {
		return nil, "", nil
	}
	source := fmt.Sprintf("Pulumi stack %v (%v)", fullStackName, relativePath(cwd, configPath))
	return importList{orgName: orgName, imports: imports}, source, nil
}

// inferDefaultEnv resolves the default environment from a .esc.yaml in the working directory or
// any parent, falling back to the imports of the currently-selected Pulumi IaC stack. The
// returned source describes where the default came from.
func (cmd *envCommand) inferDefaultEnv() (environmentDesc, string, error) {
	dir, err := cmd.esc.workingDir()
	if err != nil {
		return nil, "", err
	}

	env, source, err := cmd.inferFSEnv(dir)
	if err != nil || env != nil {
		return env, source, err
	}
	return cmd.inferPulumiIaCEnv(dir)
}
