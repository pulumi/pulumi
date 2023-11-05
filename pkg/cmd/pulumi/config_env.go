// Copyright 2016-2023, Pulumi Corporation.
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
	"errors"
	"fmt"

	"github.com/erikgeiser/promptkit/confirmation"
	"github.com/pulumi/esc"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func newConfigEnvCmd(stack *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage ESC environments for a stack",
		Long: "Manages the ESC environment associated with a specific stack. To create a new environment\n" +
			"from a stack's configuration, use `pulumi config env init`.",
		Args: cmdutil.NoArgs,
	}

	cmd.AddCommand(newConfigEnvInitCmd(stack))
	cmd.AddCommand(newConfigEnvAddCmd(stack))
	cmd.AddCommand(newConfigEnvRmCmd(stack))

	return cmd
}

func editStackEnvironment(
	stackRef string,
	showSecrets bool,
	yes bool,
	edit func(stack *workspace.ProjectStack) error,
) error {
	ctx := commandContext()

	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	project, _, err := readProject()
	if err != nil {
		return err
	}

	stack, err := requireStack(ctx, stackRef, stackOfferNew|stackSetCurrent, opts)
	if err != nil {
		return err
	}

	_, ok := stack.Backend().(backend.EnvironmentsBackend)
	if !ok {
		return fmt.Errorf("backend %v does not support environments", stack.Backend().Name())
	}

	projectStack, err := loadProjectStack(project, stack)
	if err != nil {
		return err
	}

	if err := edit(projectStack); err != nil {
		return err
	}

	if err := listConfig(ctx, project, stack, projectStack, showSecrets, false); err != nil {
		return err
	}

	if !yes {
		fmt.Println()

		save, err := confirmation.New("Save?", confirmation.Yes).RunPrompt()
		if err != nil {
			return err
		}
		if !save {
			return errors.New("canceled")
		}
	}

	if err = saveProjectStack(stack, projectStack); err != nil {
		return fmt.Errorf("saving stack config: %w", err)
	}
	return nil
}

func warnOnNoEnvironmentEffects(env *esc.Environment) {
	hasEnvVars := len(env.GetEnvironmentVariables()) != 0
	hasFiles := len(env.GetTemporaryFiles()) != 0
	_, hasPulumiConfig := env.Properties["pulumiConfig"].Value.(map[string]esc.Value)

	//nolint:lll
	if !hasEnvVars && !hasFiles && !hasPulumiConfig {
		color := cmdutil.GetGlobalColorization()
		fmt.Println(color.Colorize(colors.SpecWarning + "The stack's environment does not define the `environmentVariables`, `files`, or `pulumiConfig` properties."))
		fmt.Println(color.Colorize(colors.SpecWarning + "Without at least one of these properties, the environment will not affect the stack's behavior." + colors.Reset))
		fmt.Println()
	}
}
