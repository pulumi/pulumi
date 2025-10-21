// Copyright 2025, Pulumi Corporation.

package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/spf13/cobra"
)

// This command should maintain interface compatibility as a subset of env set.
// See env_settings_contract_test.go for basic validation.
func newEnvSettingsSetCmd(env *envCommand, registry *EnvSettingsRegistry) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set [<org-name>/][<project-name>/]<environment-name> <setting-name> <setting-value>",
		Args:  cobra.RangeArgs(2, 3),
		Short: "Set an environment setting",
		Long: "Set an environment setting\n" +
			"\n" +
			"This command sets the value of a single environment setting.\n" +
			"\n" +
			"Available settings:\n" +
			registry.GetSettingsHelpText(),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return errors.New("the settings command does not accept versions")
			}

			if len(args) != 2 {
				return fmt.Errorf("expected a setting name and value")
			}

			rawSettingName := args[0]
			rawValue := args[1]

			setting, ok := registry.GetSetting(rawSettingName)
			if !ok {
				return fmt.Errorf("unknown setting name: %s", rawSettingName)
			}
			value, err := setting.ValidateValue(rawValue)
			if err != nil {
				return err
			}

			req := client.PatchEnvironmentSettingsRequest{}
			setting.SetValue(&req, value)

			err = env.esc.client.PatchEnvironmentSettings(ctx, ref.orgName, ref.projectName, ref.envName, req)
			if err != nil {
				var errResp *apitype.ErrorResponse
				if errors.As(err, &errResp) {
					if errResp.Code == http.StatusForbidden {
						return fmt.Errorf("permission denied: insufficient permissions to update settings")
					}
				}
				return err
			}

			settings, err := env.esc.client.GetEnvironmentSettings(ctx, ref.orgName, ref.projectName, ref.envName)
			if err != nil {
				return err
			}

			outputValue := setting.GetValue(settings)
			fmt.Fprintln(env.esc.stdout, outputValue)

			return nil
		},
	}

	return cmd
}
