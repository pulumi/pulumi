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
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/oci"
	"github.com/spf13/cobra"
)

// newPackageBuildCmd builds a local package's source into a plugin image — the
// source→image seam, the dev-time twin of the registry proxy (which wraps released
// binaries as images). A package describes itself in PulumiPlugin.yaml with
// `runtime: oci` and `options.{name, version, build}`; this runs the build in a builder
// container whose image supplies the toolchain (build.image). The build emits a
// runtime-neutral OCI layout, which the engine loads into the local store under the
// provider convention ref (pulumi-provider-<name>:v<version>) and prints; `pulumi package
// publish` later pushes that image. There is deliberately no sink option — build leaves the
// image local, publish moves it.
//
// This is the principled home for "where does the build run?": it retires the throwaway
// `components:` block (where local-component build specs live today) in favour of each
// package describing its own build. A future change makes the language host's
// InstallDependencies a *caller* of this for local components.
func newPackageBuildCmd() *cobra.Command {
	// registry is the source the built image is tagged under — a location, not
	// identity. It defaults to the public convention host (so a local iteration build
	// lands where the container host resolves an unpinned package), and is set
	// explicitly when building a package destined for a private source (the same host
	// it will be published to and pulled from).
	var registry string
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
        image: my-builder:latest   # the build environment image
        # command: optional; the default builds ./Dockerfile into an OCI layout
        # context: optional; widens the build context (e.g. .. for a monorepo)

The build runs in a container started from build.image (which supplies the
toolchain). It writes a runtime-neutral OCI image layout to $PULUMI_PACKAGE_LAYOUT
(the default command uses buildkit's OCI exporter, so the daemon needs a containerd
image store or a docker-container buildx builder). The engine then loads that layout
into the local container store under the convention ref, which is printed to stdout.`,
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
			// components, so a package builds identically however it is reached. The
			// destination host defaults to the private source (where a built package
			// is tagged and later pinned from, matching the language host's local
			// component build) and is overridable to publish to a different registry.
			host := oci.PrivateRegistry()
			if registry != "" {
				host = strings.TrimPrefix(registry, "oci://")
			}
			ref, err := oci.BuildPackage(cmd.Context(), dir, host, cmd.ErrOrStderr())
			if err != nil {
				return err
			}

			// The ref is the build's product — print it on stdout so a caller (or a
			// future `publish`) can consume it.
			fmt.Fprintln(cmd.OutOrStdout(), ref)
			return nil
		},
	}

	cmd.Flags().StringVar(
		&registry, "registry", "",
		"Tag the built image under this OCI source host (oci://<host>) instead of the "+
			"default private source. This is the host the package will be published to and "+
			"pulled from. This applies a location, not identity; the build leaves the image "+
			"local, publish moves it.")

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "dir", Usage: "[dir]"}},
	})
	return cmd
}
