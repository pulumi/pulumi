// Copyright 2025, Pulumi Corporation.

package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// This command should maintain interface compatibility as a subset of env get.
// See env_settings_contract_test.go for basic validation.
func newEnvSettingsGetCmd(env *envCommand, registry *EnvSettingsRegistry) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [<org-name>/][<project-name>/]<environment-name> [<setting-name>]",
		Args:  cobra.RangeArgs(1, 2),
		Short: "Get environment settings",
		Long: "Get environment settings\n" +
			"\n" +
			"This command gets environment settings. If no setting name is provided,\n" +
			"all settings are returned. Otherwise, only the specified setting value is returned.\n" +
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

			settings, err := env.esc.client.GetEnvironmentSettings(ctx, ref.orgName, ref.projectName, ref.envName)
			if err != nil {
				return err
			}

			if len(args) == 0 {
				for _, setting := range registry.Settings {
					fmt.Fprintf(env.esc.stdout, "%s %v\n", setting.KebabName(), setting.GetValue(settings))
				}
				return nil
			}

			rawSettingName := args[0]
			setting, ok := registry.GetSetting(rawSettingName)
			if !ok {
				return fmt.Errorf("unknown setting name: %s", rawSettingName)
			}

			value := setting.GetValue(settings)
			fmt.Fprintln(env.esc.stdout, value)

			return nil
		},
	}

	return cmd
}
