// Copyright 2025, Pulumi Corporation.
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

package packagecmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/blang/semver"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// packageDeleteCmd holds the state for the delete command. This struct exists
// primarily for testability.
type packageDeleteCmd struct {
	yes bool
}

func newPackageDeleteCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <source>/<publisher>/<name>@<version>",
		Args:  cmdutil.ExactArgs(1),
		Short: "Delete a package version from the registry",
		Long: `Delete a package version from the Pulumi Registry.

The package version must be specified in the format:
  <source>/<publisher>/<name>@<version>

Example:
  pulumi package delete private/myorg/my-package@1.0.0

Warning: If this is the only version of the package, the entire package
will be removed. This action cannot be undone.

You must have publish permissions for the package to delete it.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			yes = yes || env.SkipConfirmations.Value()

			deleteCmd := &packageDeleteCmd{
				yes: yes,
			}

			return deleteCmd.Run(ctx, args[0])
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Skip confirmation prompts, and proceed with deletion anyway")

	return cmd
}

func (cmd *packageDeleteCmd) Run(ctx context.Context, packageVersion string) error {
	// Parse package version first to fail fast on invalid input
	source, publisher, name, version, err := parsePackageVersion(packageVersion)
	if err != nil {
		return err
	}

	opts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}

	// Check non-interactive mode before doing any backend work
	if !cmdutil.Interactive() && !cmd.yes {
		return errors.New("non-interactive mode requires --yes flag")
	}

	// Confirm deletion if not using --yes
	prompt := fmt.Sprintf("This will permanently delete package version '%s'!", packageVersion)
	if !cmd.yes && !ui.ConfirmPrompt(prompt, packageVersion, opts) {
		return result.FprintBailf(os.Stdout, "confirmation declined")
	}

	// Get backend and registry
	project, _, err := pkgWorkspace.Instance.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return fmt.Errorf("failed to determine current project: %w", err)
	}

	b, err := login(ctx, project)
	if err != nil {
		return err
	}

	registry, err := b.GetCloudRegistry()
	if err != nil {
		return fmt.Errorf("failed to get the registry backend: %w", err)
	}

	// Delete the package version
	if err := registry.DeletePackageVersion(ctx, source, publisher, name, version); err != nil {
		return fmt.Errorf("failed to delete package version: %w", err)
	}

	msg := fmt.Sprintf("%sPackage version '%s' has been deleted!%s",
		colors.SpecAttention, packageVersion, colors.Reset)
	fmt.Println(opts.Color.Colorize(msg))
	return nil
}

// parsePackageVersion parses a package version string in the format
// <source>/<publisher>/<name>@<version> and returns its components.
func parsePackageVersion(input string) (source, publisher, name string, version semver.Version, err error) {
	// Split by @ to separate name from version
	parts := strings.SplitN(input, "@", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", "", "", semver.Version{}, errors.New("invalid package version format\n" +
			"  Expected format: <source>/<publisher>/<name>@<version>\n" +
			"  Example: private/myorg/my-package@1.0.0")
	}

	// Split the name part by /
	nameParts := strings.Split(parts[0], "/")
	if len(nameParts) != 3 {
		return "", "", "", semver.Version{}, errors.New("invalid package name format\n" +
			"  Expected format: <source>/<publisher>/<name>@<version>\n" +
			"  Example: private/myorg/my-package@1.0.0")
	}

	source = nameParts[0]
	publisher = nameParts[1]
	name = nameParts[2]

	// Validate that none of the parts are empty
	if source == "" || publisher == "" || name == "" {
		return "", "", "", semver.Version{}, errors.New(
			"invalid package version format: source, publisher, and name cannot be empty\n" +
				"  Expected format: <source>/<publisher>/<name>@<version>\n" +
				"  Example: private/myorg/my-package@1.0.0")
	}

	// Validate semantic version
	version, err = semver.Parse(parts[1])
	if err != nil {
		return "", "", "", semver.Version{}, fmt.Errorf(
			"invalid semantic version: %q\n"+
				"  Version must follow semantic versioning (e.g., 1.0.0, 2.1.3)",
			parts[1])
	}

	return source, publisher, name, version, nil
}
