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

package initcmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

// NewInitCmd creates a command that initializes a minimal Pulumi project file.
func NewInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "init [name]",
		Short:  "Create a minimal Pulumi project file",
		Long: "Create a minimal Pulumi project file.\n" +
			"\n" +
			"This command writes a Pulumi.yaml file containing only the project name. " +
			"If no name is provided, the current directory name is used.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInitCmd(cmd.OutOrStdout(), args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "name"}},
		Required:  0,
	})

	return cmd
}

func runInitCmd(stdout io.Writer, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting the working directory: %w", err)
	}

	name := filepath.Base(cwd)
	if len(args) > 0 {
		name = args[0]
	} else {
		name = pkgWorkspace.ValueOrSanitizedDefaultProjectName("", "${PROJECT}", name)
	}

	if err := pkgWorkspace.ValidateProjectName(name); err != nil {
		return fmt.Errorf("'%s' is not a valid project name: %w", name, err)
	}

	path := filepath.Join(cwd, "Pulumi.yaml")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("checking for existing Pulumi.yaml: %w", err)
	}

	// Write the YAML raw, because the go marshaller always writes `Runtime: null` if the runtime is empty when if we
	// use a `workspace.Project`.
	proj := struct {
		Name string `yaml:"name"`
	}{
		Name: name,
	}

	contents, err := yaml.Marshal(proj)
	if err != nil {
		return fmt.Errorf("marshalling Pulumi.yaml: %w", err)
	}

	if err := os.WriteFile(path, contents, 0o600); err != nil {
		return fmt.Errorf("writing Pulumi.yaml: %w", err)
	}

	fmt.Fprintf(stdout, "Created Pulumi.yaml\n")
	return nil
}
