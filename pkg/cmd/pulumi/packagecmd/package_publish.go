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

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
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
}

func newPackagePublishCmd() *cobra.Command {
	args := publishPackageArgs{}
	var pkgPublishCmd packagePublishCmd

	cmd := &cobra.Command{
		Use:   "publish <provider|schema> --readme <path> [provider-parameter...]",
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
	contract.AssertNoErrorf(cmd.MarkFlagRequired("readme"), "failed to mark 'readme' as a required flag")
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
	if args.readmePath == "" {
		return errors.New("no readme specified, please provide the path to the readme file")
	}

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

	var installDocsBytes *bytes.Buffer
	if args.installDocsPath != "" {
		installDocs, err := os.Open(args.installDocsPath)
		if err != nil {
			return fmt.Errorf("failed to open install docs file: %w", err)
		}
		defer contract.IgnoreClose(installDocs)
		installDocsBytes = bytes.NewBuffer(nil)
		if _, err := io.Copy(installDocsBytes, installDocs); err != nil {
			return fmt.Errorf("failed to read install docs file: %w", err)
		}
	}

	err = registry.Publish(ctx, backend.PackagePublishOp{
		Source:      args.source,
		Publisher:   publisher,
		Name:        name,
		Version:     version,
		Schema:      bytes.NewReader(json),
		Readme:      readmeBytes,
		InstallDocs: installDocsBytes,
	})
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
