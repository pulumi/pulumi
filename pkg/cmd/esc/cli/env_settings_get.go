// Copyright 2024, Pulumi Corporation.
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

package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// This command should maintain interface compatibility as a subset of env get.
// See env_settings_contract_test.go for basic validation.
func newEnvSettingsGetCmd(env *envCommand, registry *EnvSettingsRegistry) *cobra.Command {
	var output string

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
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			format, err := parseOutputFormat(output)
			if err != nil {
				return err
			}

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
				if format == outputJSON {
					all := map[string]any{}
					for _, setting := range registry.Settings {
						all[setting.KebabName()] = setting.GetValue(settings)
					}
					return writeJSON(env.esc.stdout, all)
				}
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
			if format == outputJSON {
				return writeJSON(env.esc.stdout, map[string]any{setting.KebabName(): value})
			}
			fmt.Fprintln(env.esc.stdout, value)

			return nil
		},
	}

	addOutputFlag(cmd, &output)

	return cmd
}
