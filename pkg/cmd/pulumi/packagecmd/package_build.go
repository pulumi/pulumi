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

package packagecmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/oci"
	"github.com/spf13/cobra"
)

// newPackageBuildCmd builds a local package's source into a plugin image — the
// source→image seam, the dev-time twin of the registry proxy (which wraps released
// binaries as images). A package describes itself in PulumiPlugin.yaml with
// `runtime: oci` and `options.{name, version, build}`; this runs the build in a builder
// container whose image supplies the toolchain (build.image), tagging the result by the
// provider convention (pulumi-provider-<name>:v<version>). The built image lands in the
// local store and the ref is printed; `pulumi package publish` later pushes that image.
// There is deliberately no sink option — build leaves the image local, publish moves it.
//
// This is the principled home for "where does the build run?": it retires the throwaway
// `components:` block (where local-component build specs live today) in favour of each
// package describing its own build. A future change makes the language host's
// InstallDependencies a *caller* of this for local components.
func newPackageBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build [dir]",
		Short: "Build a local package's source into a plugin image",
		Long: `Build a local package's source into a plugin image.

The package directory (default: the current directory) must contain a
PulumiPlugin.yaml declaring 'runtime: oci' with options:

  runtime:
    name: oci
    options:
      name: my-component        # the package name
      version: "1.0.0"          # the package version
      build:
        image: my-builder:latest          # the build environment image
        command: docker build -q -t "$PULUMI_PACKAGE_IMAGE" .   # optional

The build runs in a container started from build.image (which supplies the
toolchain), with the target image ref available as $PULUMI_PACKAGE_IMAGE. The
command must produce that image in the local container store. The resulting ref
is printed to stdout.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			dir, err := filepath.Abs(dir)
			if err != nil {
				return err
			}

			// The build runs in a builder container that reaches the source via
			// --volumes-from the engine container, so this must run inside the engine
			// (pod mode). Unlike `package add`'s schema fetch, it needs no engine netns.
			if os.Getenv("PULUMI_POD_MODE") != "true" {
				return errors.New("pulumi package build must run inside the engine container (pod mode)")
			}

			// The same build-from-manifest logic the language host uses for local
			// components, so a package builds identically however it is reached.
			registry := os.Getenv("PULUMI_POD_PLUGIN_REGISTRY")
			ref, err := oci.BuildPackage(cmd.Context(), dir, registry, cmd.ErrOrStderr())
			if err != nil {
				return err
			}

			// The ref is the build's product — print it on stdout so a caller (or a
			// future `publish`) can consume it.
			fmt.Fprintln(cmd.OutOrStdout(), ref)
			return nil
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "dir", Usage: "[dir]"}},
	})
	return cmd
}
