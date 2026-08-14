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

package selfupdate

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/blang/semver"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
)

func NewSelfUpdateCmd() *cobra.Command {
	var targetVersion string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "self-update",
		Short: "Update the Pulumi CLI to the latest version",
		Long: "Update the Pulumi CLI to the latest version.\n" +
			"\n" +
			"This replaces the running CLI, and the language plugins bundled with it, with the newest released\n" +
			"version. The release archive is checked against its published SHA256 checksum before anything is\n" +
			"replaced, and the previous binaries are only discarded once the new ones are in place.\n" +
			"\n" +
			"This only applies to installs produced by https://get.pulumi.com. If the CLI came from a package\n" +
			"manager such as Homebrew, upgrade it with that package manager instead.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return selfUpdate(cmd.Context(), cmd.OutOrStdout(), targetVersion, dryRun)
		},
	}

	cmd.PersistentFlags().StringVar(&targetVersion, "version", "",
		"Install this version instead of the latest release")
	cmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false,
		"Report what would be installed without changing anything")

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	return cmd
}

func selfUpdate(ctx context.Context, out io.Writer, targetVersion string, dryRun bool) error {
	installDir, err := detectInstallDir()
	if err != nil {
		return err
	}

	current, err := semver.ParseTolerant(version.Version)
	if err != nil {
		return fmt.Errorf("could not parse the running version %q: %w", version.Version, err)
	}

	return update(ctx, out, installDir, current, targetVersion, dryRun)
}

// update installs targetVersion into installDir, replacing the binaries of the currently running version.
func update(
	ctx context.Context, out io.Writer, installDir string, current semver.Version, targetVersion string, dryRun bool,
) error {
	// Windows cannot delete the executable it is running, and may briefly hold a handle to a freshly written one, so
	// the previous update can leave both its binary and its staging directory behind. Clear them here rather than
	// only during a swap, so that a run which has nothing to install still tidies up.
	removeStale(installDir)
	removeOrphanedStaging(filepath.Dir(installDir))

	target, err := resolveTarget(ctx, targetVersion)
	if err != nil {
		return err
	}

	if current.EQ(target) {
		fmt.Fprintf(out, "Pulumi v%s is already up to date.\n", current)
		return nil
	}

	artifact, err := artifactName(target)
	if err != nil {
		return err
	}

	if dryRun {
		fmt.Fprintf(out, "Would update Pulumi v%s to v%s in %s, using %s.\n", current, target, installDir, artifact)
		return nil
	}

	// Stage alongside the install directory so that the binaries can be moved into place with a rename rather than
	// a copy, which keeps the swap quick and avoids a partially written executable.
	staging, err := os.MkdirTemp(filepath.Dir(installDir), stagingPrefix+"*")
	if err != nil {
		return fmt.Errorf("could not create a staging directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(staging)
	}()

	fmt.Fprintf(out, "Updating Pulumi v%s to v%s...\n", current, target)

	downloaded, err := downloadVerified(ctx, target, artifact, staging)
	if err != nil {
		return err
	}

	staged, err := extract(downloaded, staging)
	if err != nil {
		return err
	}

	if err := swapIn(staged, installDir); err != nil {
		return err
	}

	fmt.Fprintf(out, "Pulumi is now at v%s.\n", target)
	return nil
}

func resolveTarget(ctx context.Context, targetVersion string) (semver.Version, error) {
	if targetVersion == "" {
		return latestVersion(ctx)
	}

	v, err := semver.ParseTolerant(targetVersion)
	if err != nil {
		return semver.Version{}, fmt.Errorf("could not parse the requested version %q: %w", targetVersion, err)
	}
	return v, nil
}
