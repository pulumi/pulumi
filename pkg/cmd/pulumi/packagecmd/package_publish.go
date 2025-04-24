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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

const (
	// The default package source is "pulumi" for packages published to the Pulumi Registry.
	// This is the source that will be used if none is specified on the command line.
	// Examples of other sources include "opentofu" for packages published to the OpenTofu Registry.
	defaultPackageSource = "pulumi"
)

type publishPackageArgs struct {
	source          string
	publisher       string
	readmePath      string
	installDocsPath string
}

type packagePublishCmd struct {
	defaultOrg    func(*workspace.Project) (string, error)
	extractSchema func(pctx *plugin.Context, packageSource string, args []string) (*schema.Package, error)
	pluginDir     string
}

func newPackagePublishCmd() *cobra.Command {
	args := publishPackageArgs{}
	var pkgPublishCmd packagePublishCmd

	cmd := &cobra.Command{
		Use:   "publish <provider|schema> --readme <path> [--] [provider-parameter...]",
		Args:  cmdutil.MinimumNArgs(1),
		Short: "Publish a package to the Pulumi Registry",
		Long: "Publish a package to the Pulumi Registry.\n\n" +
			"This command publishes a package to the Pulumi Registry. The package can be a provider " +
			"or a schema.\n\n" +
			"When <provider> is specified as a PLUGIN[@VERSION] reference, Pulumi attempts to " +
			"resolve a resource plugin first, installing it on-demand, similarly to:\n\n" +
			"  pulumi plugin install resource PLUGIN [VERSION]\n\n" +
			"When <provider> is specified as a local path, Pulumi executes the provider " +
			"binary to extract its package schema.\n\n" +
			"For parameterized providers, parameters may be specified as additional " +
			"arguments. The exact format of parameters is provider-specific; consult the " +
			"provider's documentation for more information. If the parameters include flags " +
			"that begin with dashes, you may need to use '--' to separate the provider name " +
			"from the parameters, as in:\n\n" +
			"  pulumi package publish <provider> --readme ./README.md -- --provider-parameter-flag value\n\n" +
			"When <schema> is a path to a local file with a '.json', '.yml' or '.yaml' " +
			"extension, Pulumi package schema is read from it directly:\n\n" +
			"  pulumi package publish ./my/schema.json --readme ./README.md",
		Hidden: !env.Experimental.Value(),
		RunE: func(cmd *cobra.Command, cliArgs []string) error {
			ctx := cmd.Context()
			pkgPublishCmd.defaultOrg = pkgWorkspace.GetBackendConfigDefaultOrg
			pkgPublishCmd.extractSchema = SchemaFromSchemaSource
			return pkgPublishCmd.Run(ctx, args, cliArgs[0], cliArgs[1:])
		},
	}

	cmd.Flags().StringVar(
		&args.source, "source", defaultPackageSource,
		"The origin of the package (e.g., 'pulumi', 'opentofu'). Defaults to the current registry.")

	cmd.Flags().StringVar(
		&args.publisher, "publisher", "",
		"The publisher of the package (e.g., 'pulumi'). Defaults to the publisher set in the package "+
			"schema or the default organization in your pulumi config.")

	cmd.Flags().StringVar(
		&args.readmePath, "readme", "",
		"Path to the package readme/index markdown file")
	cmd.Flags().StringVar(
		&args.installDocsPath, "installation-configuration", "",
		"Path to the installation configuration markdown file")

	return cmd
}

func (cmd *packagePublishCmd) Run(
	ctx context.Context,
	args publishPackageArgs,
	packageSrc string,
	packageParams []string,
) error {
	project, _, err := pkgWorkspace.Instance.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return fmt.Errorf("failed to determine current project: %w", err)
	}

	b, err := login(ctx, project)
	if err != nil {
		return err
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	sink := cmdutil.Diag()
	pctx, err := plugin.NewContext(sink, sink, nil, nil, wd, nil, false, nil)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(pctx)

	pkg, err := cmd.extractSchema(pctx, packageSrc, packageParams)
	if err != nil {
		return fmt.Errorf("failed to get schema: %w", err)
	}

	// If no readme path is provided, check if there's a readme in the package source or plugin directory we can slurp up.
	if args.readmePath == "" {
		readmePath, err := cmd.findReadme(packageSrc)
		if err != nil {
			return fmt.Errorf("failed to find readme: %w", err)
		}
		args.readmePath = readmePath
	}
	if args.readmePath == "" {
		return errors.New("no README found. Please add one named README.md to the package, " +
			"or use --readme to specify the path")
	}

	var publisher string
	// If the publisher is set on the command line, use it.
	if args.publisher != "" {
		publisher = args.publisher
	} else if pkg.Publisher != "" { // Otherwise, fall back to the publisher set in the package schema.
		publisher = pkg.Publisher
	} else { // As a last resort, try to determine the publisher from the default organization or fail if none is found.
		publisher, err = cmd.defaultOrg(project)
		if err != nil {
			return fmt.Errorf("failed to determine default organization: %w", err)
		}
		if publisher == "" {
			return errors.New("no publisher specified and no default organization found, please set a publisher in " +
				"the package schema or set a default organization in your pulumi config")
		}
	}

	name := pkg.Name
	if name == "" {
		return errors.New("no package name specified, please set one in the package schema")
	}
	var version semver.Version
	if pkg.Version != nil {
		version = *pkg.Version
	} else {
		return errors.New("no version specified, please set a version in the package schema")
	}

	json, err := pkg.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	registry, err := b.GetPackageRegistry()
	if err != nil {
		return fmt.Errorf("failed to get package registry: %w", err)
	}

	// We need to set the content-size header for S3 puts. For byte buffers (or deterministic readers)
	// the stdlib http client does that automatically. For os.File it cannot do that because the size
	// returned by Stat() is not reliable:
	// - Another process might modify the file by appending to it or truncating it
	// - If the file is a special file like a device file, pipe, or socket, its size may change dynamically
	// - On network filesystems, there could be caching or synchronization issues
	//
	// By reading it into a buffer we circumvent these problems. This should not have a meaningful impact
	// on performance, as these files are typically small (likely less than one MB).

	// In the future we could slurp up the readme from the package source and add the necessary markdown headers.
	readme, err := os.Open(args.readmePath)
	if err != nil {
		return fmt.Errorf("failed to open readme file: %w", err)
	}
	defer contract.IgnoreClose(readme)
	readmeBytes := bytes.NewBuffer(nil)
	if _, err := io.Copy(readmeBytes, readme); err != nil {
		return fmt.Errorf("failed to read readme file: %w", err)
	}

	publishInput := apitype.PackagePublishOp{
		Source:    args.source,
		Publisher: publisher,
		Name:      name,
		Version:   version,
		Schema:    bytes.NewReader(json),
		Readme:    readmeBytes,
	}

	if args.installDocsPath != "" {
		installDocs, err := os.Open(args.installDocsPath)
		if err != nil {
			return fmt.Errorf("failed to open install docs file: %w", err)
		}
		defer contract.IgnoreClose(installDocs)
		installDocsBytes := bytes.NewBuffer(nil)
		if _, err := io.Copy(installDocsBytes, installDocs); err != nil {
			return fmt.Errorf("failed to read install docs file: %w", err)
		}
		publishInput.InstallDocs = installDocsBytes
	}

	err = registry.Publish(ctx, publishInput)
	if err != nil {
		return fmt.Errorf("failed to publish package: %w", err)
	}

	fmt.Printf("Successfully published package %s/%s@%s\n", publisher, name, version)

	return nil
}

func login(ctx context.Context, project *workspace.Project) (backend.Backend, error) {
	b, err := cmdBackend.CurrentBackend(
		ctx, pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, project,
		display.Options{Color: cmdutil.GetGlobalColorization()})
	if err != nil {
		return nil, err
	}

	return b, nil
}

// findReadme attempts to find a file named README.md (case insensitive) in the given package source.
// It tries to find a readme in the following order and returns the path to the first one it finds:
// 1. The package source if it is a directory
// 2. The installed plugin directory
// If no readme is found, an empty string is returned.
func (cmd *packagePublishCmd) findReadme(packageSrc string) (string, error) {
	findReadmeInDir := func(dir string) string {
		info, err := os.Stat(dir)
		if err != nil && errors.Is(err, os.ErrNotExist) {
			return ""
		} else if err != nil {
			return ""
		}
		if !info.IsDir() {
			return ""
		}
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					name := strings.ToLower(entry.Name())
					if name == "readme.md" {
						return filepath.Join(dir, entry.Name())
					}
				}
			}
		}
		return ""
	}

	if ext := filepath.Ext(packageSrc); ext == ".json" || ext == ".yaml" || ext == ".yml" {
		// If the package source is a schema file, there's no README.md to be found.
		return "", nil
	}
	// If the source is a directory, check if it contains a readme.
	if readmeFromPackage := findReadmeInDir(packageSrc); readmeFromPackage != "" {
		return readmeFromPackage, nil
	}

	// Otherwise, try to retrieve the readme from the installed plugin.
	pluginSpec, err := workspace.NewPluginSpec(packageSrc, apitype.ResourcePlugin, nil, "", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create plugin spec: %w", err)
	}
	pluginSpec.PluginDir = cmd.pluginDir

	pluginDir, err := pluginSpec.DirPath()
	if err != nil {
		return "", fmt.Errorf("failed to get plugin directory: %w", err)
	}
	_, path := pluginSpec.LocalName()
	dir := filepath.Join(pluginDir, path)

	if readmeFromPlugin := findReadmeInDir(dir); readmeFromPlugin != "" {
		return readmeFromPlugin, nil
	}

	return "", nil
}
