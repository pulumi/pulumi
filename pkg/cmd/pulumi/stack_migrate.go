// Copyright 2016-2022, Pulumi Corporation.
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

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func migrateStack(
	ctx context.Context, opts display.Options,
	getBackend func(context.Context, display.Options, string) (backend.Backend, error),
	sourceStack string, version string,
	targetURL string, secretsProvider string) error {

	// Fetch the current stack and export its deployment
	s, err := requireStack(ctx, sourceStack, false, opts, false /*setCurrent*/)
	if err != nil {
		return err
	}
	currentProjectStack, err := loadProjectStack(s)
	if err != nil {
		return err
	}

	var deployment *apitype.UntypedDeployment
	// Export the latest version of the checkpoint by default. Otherwise, we require that
	// the backend/stack implements the ability the export previous checkpoints.
	if version == "" {
		deployment, err = s.ExportDeployment(ctx)
		if err != nil {
			return err
		}
	} else {
		// Check that the stack and its backend supports the ability to do this.
		be := s.Backend()
		specificExpBE, ok := be.(backend.SpecificDeploymentExporter)
		if !ok {
			return fmt.Errorf("the current backend (%s) does not provide the ability to export previous deployments",
				be.Name())
		}

		deployment, err = specificExpBE.ExportDeploymentForVersion(ctx, s, version)
		if err != nil {
			return err
		}
	}

	// Build decrypter based on the existing secrets provider
	var decrypter config.Decrypter
	currentConfig := currentProjectStack.Config

	if currentConfig.HasSecureValue() {
		dec, decerr := getStackDecrypter(s)
		if decerr != nil {
			return decerr
		}
		decrypter = dec
	} else {
		decrypter = config.NewPanicCrypter()
	}

	// Get the new backend
	targetBackend, err := getBackend(ctx, opts, targetURL)
	if err != nil {
		return err
	}

	// Create the target stack, this just uses the name of the current stack but picks up any default values
	// for project and organization (if the backend uses them).
	targetStack := s.Ref().Name().String()
	targetStackRef, err := targetBackend.ParseStackReference(targetStack)
	if err != nil {
		return err
	}
	t, err := targetBackend.CreateStack(ctx, targetStackRef, nil)
	if err != nil {
		return err
	}

	err = t.ImportDeployment(ctx, deployment)
	if err != nil {
		return err
	}

	// Create the new secrets provider and set to the currentStack
	if err := createSecretsManager(ctx, t, secretsProvider, false); err != nil {
		return err
	}
	return migrateOldConfigAndCheckpointToNewSecretsProvider(ctx, t, currentConfig, decrypter)
}

func newStackMigrateCmd() *cobra.Command {
	var stackName string
	var version string

	cmd := &cobra.Command{
		Use:   "migrate <target backend url> [target secret provider]",
		Args:  cmdutil.RangeArgs(1, 2),
		Short: "Migrate a stack to a different backend",
		Long: "Migrate a stack to a different backend.\n" +
			"\n" +
			"This will export the current (or selected stack) from the backend currently logged into, and then " +
			"import that stack into the new target backend. This will correctly re-encrypt secrets for the new stack, " +
			"and also handle any secrets in stack config.\n" +
			"\n" +
			"examples:\n" +
			"\n" +
			" - Moving the current stack into the pulumi service:\n" +
			"   `pulumi migrate https://app.pulumi.com`\n" +
			"",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := commandContext()
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			secretProvider := ""
			if len(args) > 1 {
				secretProvider = args[1]
			}

			return migrateStack(
				ctx, opts,
				getBackend,
				stackName, version,
				args[0], secretProvider)
		}),
	}
	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "", "The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().StringVarP(
		&version, "version", "", "", "Previous stack version to migrate. (If unset, will export the latest.)")
	return cmd
}
