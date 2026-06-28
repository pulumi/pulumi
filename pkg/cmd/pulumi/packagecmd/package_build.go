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
	"fmt"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/oci"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
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
				return fmt.Errorf("pulumi package build must run inside the engine container (pod mode)")
			}

			proj, err := workspace.LoadPluginProject(filepath.Join(dir, "PulumiPlugin.yaml"))
			if err != nil {
				return fmt.Errorf("loading PulumiPlugin.yaml in %s: %w", dir, err)
			}
			if proj.Runtime.Name() != "oci" {
				return fmt.Errorf("package build supports runtime: oci, got %q", proj.Runtime.Name())
			}
			opts := proj.Runtime.Options()

			name, _ := opts["name"].(string)
			version, _ := opts["version"].(string)
			if name == "" || version == "" {
				return fmt.Errorf("the package must declare runtime.options.name and runtime.options.version")
			}
			build, _ := opts["build"].(map[string]any)
			buildImage, _ := build["image"].(string)
			if buildImage == "" {
				return fmt.Errorf("the package must declare runtime.options.build.image (the build environment)")
			}

			// One source of truth for the convention, shared with the container host
			// (resolve) and the language host (local-component tagging), qualified with
			// the plugin registry when one is configured — so the built ref is named
			// exactly where it resolves at Construct and where publish will push it.
			registry := os.Getenv("PULUMI_POD_PLUGIN_REGISTRY")
			ref := oci.ProviderImageRef(registry, name, version)

			command, _ := build["command"].(string)
			if command == "" {
				// Default: build the Dockerfile in the package dir and tag it as the
				// convention ref (delivered via env so the command stays ref-agnostic).
				command = `docker build -q -t "$PULUMI_PACKAGE_IMAGE" .`
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Building %s (v%s) in %s -> %s\n", name, version, buildImage, ref)
			if _, err := oci.BuildInContainer(
				cmd.Context(), buildImage, command, dir, optStringSlice(build["caches"]),
				map[string]string{"PULUMI_PACKAGE_IMAGE": ref}, cmd.ErrOrStderr(),
			); err != nil {
				return fmt.Errorf("building package %s: %w", name, err)
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

// optStringSlice reads a YAML list-of-strings option (parsed as []any) into []string,
// skipping non-string/empty entries. nil-safe.
func optStringSlice(v any) []string {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		if s, ok := e.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}
