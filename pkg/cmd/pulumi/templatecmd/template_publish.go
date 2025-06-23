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

package templatecmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

type publishTemplateArgs struct {
	publisher string
	version   string
	name      string
}

type templatePublishCmd struct {
	defaultOrg func(context.Context, backend.Backend, *workspace.Project) (string, error)
}

func newTemplatePublishCmd() *cobra.Command {
	var args publishTemplateArgs

	cmd := &cobra.Command{
		Use:   "publish <directory>",
		Args:  cmdutil.ExactArgs(1),
		Short: "Publish a template to the Pulumi Registry",
		Long: "Publish a template to the Pulumi Registry.\n\n" +
			"This command publishes a template directory to the Pulumi Registry.",
		RunE: func(cmd *cobra.Command, cliArgs []string) error {
			ctx := cmd.Context()
			tplPublishCmd := templatePublishCmd{defaultOrg: backend.GetDefaultOrg}
			return tplPublishCmd.Run(ctx, cmd, args, cliArgs[0])
		},
	}

	cmd.Flags().StringVar(
		&args.version, "version", "",
		"The version of the template (required, semver format)")
	cmd.Flags().StringVar(
		&args.name, "name", "",
		"The name of the template (required)")
	cmd.Flags().StringVar(
		&args.publisher, "publisher", "",
		"The publisher of the template (e.g., 'pulumi'). Defaults to the default organization in your pulumi config.")
	contract.AssertNoErrorf(cmd.MarkFlagRequired("version"), "Could not mark \"version\" as required")
	contract.AssertNoErrorf(cmd.MarkFlagRequired("name"), "Could not mark \"name\" as required")

	return cmd
}

func (tplCmd *templatePublishCmd) Run(
	ctx context.Context,
	cmd *cobra.Command,
	args publishTemplateArgs,
	templateDir string,
) error {
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		return fmt.Errorf("template directory does not exist: %s", templateDir)
	}

	version, err := semver.ParseTolerant(args.version)
	if err != nil {
		return fmt.Errorf("invalid version format: %w", err)
	}

	project, _, err := pkgWorkspace.Instance.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return fmt.Errorf("failed to determine current project: %w", err)
	}

	b, err := cmdBackend.CurrentBackend(
		ctx, pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, project,
		display.Options{Color: cmdutil.GetGlobalColorization()})
	if err != nil {
		return err
	}

	_, err = b.GetCloudRegistry()
	if err != nil {
		return fmt.Errorf("backend does not support registry operations: %w", err)
	}

	var publisher string
	if args.publisher != "" {
		publisher = args.publisher
	} else {
		publisher, err = tplCmd.defaultOrg(ctx, b, project)
		if err != nil {
			return fmt.Errorf("failed to determine default organization: %w", err)
		}
		if publisher == "" {
			return errors.New(
				"no publisher specified and no default organization found, " +
					"please set a publisher or set a default organization in your pulumi config",
			)
		}
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Creating archive from directory: %s\n", templateDir)
	archiveBytes, err := archive.TGZ(templateDir, "", true /*useDefaultExcludes*/)
	if err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}
	archiveData := bytes.NewBuffer(archiveBytes)

	if err := tokens.ValidateProjectName(args.name); err != nil {
		return fmt.Errorf("invalid template name: %w", err)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Publishing template %s/%s@%s...\n", publisher, args.name, version.String())
	err = tplCmd.publishTemplate(ctx, b, publisher, args.name, version, archiveData)
	if err != nil {
		return fmt.Errorf("failed to publish template: %w", err)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Successfully published template %s/%s@%s\n", publisher, args.name, version.String())
	return nil
}

func (cmd *templatePublishCmd) publishTemplate(
	ctx context.Context,
	b backend.Backend,
	publisher, name string,
	version semver.Version,
	archiveData *bytes.Buffer,
) error {
	registry, err := b.GetCloudRegistry()
	if err != nil {
		return fmt.Errorf("failed to get cloud registry: %w", err)
	}

	publishInput := apitype.TemplatePublishOp{
		Source:    "private",
		Publisher: publisher,
		Name:      name,
		Version:   version,
		Archive:   archiveData,
	}

	err = registry.PublishTemplate(ctx, publishInput)
	if err != nil {
		return fmt.Errorf("failed to publish template: %w", err)
	}

	return nil
}
