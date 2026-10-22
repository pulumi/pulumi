// Copyright 2016, Pulumi Corporation.
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

package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	resourceanalyzer "github.com/pulumi/pulumi/pkg/v3/resource/analyzer"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
)

const latestKeyword = "latest"

type policyEnableArgs struct {
	policyGroup string
	config      string
}

type policyPackCmdDeps func(
	ctx context.Context, policyPackRef string,
) (backend.PolicyPack, policyGroupGetClient, error)

func defaultPolicyPackCmdDeps(
	ctx context.Context, policyPackRef string,
) (backend.PolicyPack, policyGroupGetClient, error) {
	policyPack, err := requirePolicyPack(ctx, policyPackRef, cmdBackend.DefaultLoginManager)
	if err != nil {
		return nil, nil, err
	}
	cloudBackend, ok := policyPack.Backend().(httpstate.Backend)
	if !ok {
		return nil, nil, errors.New("this command requires the Pulumi Cloud backend; run `pulumi login`")
	}
	return policyPack, cloudBackend.Client(), nil
}

// readBackPolicyGroup fetches the group a pack was just enabled in or disabled from, since the update endpoint
// returns nothing.
func readBackPolicyGroup(
	ctx context.Context, client policyGroupGetClient, policyPack backend.PolicyPack, group string,
) (cloudapitype.PolicyGroup, error) {
	if group == "" {
		group = apitype.DefaultPolicyGroup
	}
	resp, err := client.GetPolicyGroup(ctx, policyPack.Ref().OrgName(), group)
	if err != nil {
		return cloudapitype.PolicyGroup{}, fmt.Errorf("reading policy group %q after update: %w", group, err)
	}
	return resp, nil
}

// policyEnableClient is the subset of cloud-API operations `policy enable` needs.
type policyEnableClient interface {
	policyGroupGetClient
	packSettingsClient
}

type policyEnableDeps func(
	ctx context.Context, policyPackRef string,
) (backend.PolicyPack, policyEnableClient, error)

func defaultPolicyEnableDeps(
	ctx context.Context, policyPackRef string,
) (backend.PolicyPack, policyEnableClient, error) {
	policyPack, err := requirePolicyPack(ctx, policyPackRef, cmdBackend.DefaultLoginManager)
	if err != nil {
		return nil, nil, err
	}
	cloudBackend, ok := policyPack.Backend().(httpstate.Backend)
	if !ok {
		return nil, nil, errors.New("this command requires the Pulumi Cloud backend; run `pulumi login`")
	}
	return policyPack, cloudBackend.Client(), nil
}

func newPolicyEnableCmd() *cobra.Command {
	return newPolicyEnableCmdWith(defaultPolicyEnableDeps)
}

func newPolicyEnableCmdWith(deps policyEnableDeps) *cobra.Command {
	contract.Assertf(deps != nil, "policyEnableDeps must not be nil")
	args := policyEnableArgs{}
	var settings packSettingsFlags
	outputFormat := defaultPolicyGroupGetOutputFormat()

	cmd := &cobra.Command{
		Use:   "enable",
		Short: "Enable a Policy Pack for a Pulumi organization",
		Long: "Enable a Policy Pack for a Pulumi organization, or change the settings of a Policy Pack that is\n" +
			"already enabled in a Policy Group.\n" +
			"\n" +
			"The version is 'latest' or a specific version. It may be omitted when the Policy Pack is already\n" +
			"enabled in the Policy Group, in which case the enabled version is kept. When the version changes,\n" +
			"configuration for policies that do not exist in the new version is removed and reported.\n" +
			"\n" +
			"The Policy Pack's existing configuration and ESC environments in the Policy Group are kept, and only\n" +
			"the changes requested with flags are applied. --config replaces the whole configuration instead.\n" +
			"\n" +
			"Enforcement levels are one of: " + enforcementLevelValues + ".\n" +
			"--enforcement-level sets the level of every policy and clears per-policy enforcement levels;\n" +
			"--policy-enforcement then sets the level of individual policies. The remediate level only takes\n" +
			"effect for organizations entitled to it.",
		Example: "  # Enable the latest version in the default Policy Group\n" +
			"  pulumi policy enable acme/aws-guardrails latest\n" +
			"\n" +
			"  # Promote every policy to mandatory, keeping one advisory\n" +
			"  pulumi policy enable acme/aws-guardrails --policy-group prod \\\n" +
			"    --enforcement-level mandatory --policy-enforcement s3-no-public-read=advisory\n" +
			"\n" +
			"  # Upgrade to version 3 and attach an ESC environment\n" +
			"  pulumi policy enable acme/aws-guardrails 3 --policy-group prod \\\n" +
			"    --environment policies/aws-credentials --output json",
		RunE: func(cmd *cobra.Command, cliArgs []string) error {
			ctx := cmd.Context()
			changes, err := settings.parse()
			if err != nil {
				return err
			}
			if args.config != "" && changes.changesConfig() {
				return cmdCmd.ConfigurationError{Message: "--config cannot be combined with " +
					"--enforcement-level or --policy-enforcement"}
			}

			// Load the configuration from the user-specified JSON file into config object.
			var config map[string]*json.RawMessage
			if args.config != "" {
				config, err = loadPolicyConfigFromFile(args.config)
				if err != nil {
					return err
				}
			}

			// Obtain current PolicyPack, tied to the Pulumi Cloud backend.
			policyPack, client, err := deps(ctx, cliArgs[0])
			if err != nil {
				return err
			}

			org := policyPack.Ref().OrgName()
			name := string(policyPack.Ref().Name())
			group := args.policyGroup
			if group == "" {
				group = apitype.DefaultPolicyGroup
			}

			before, err := client.GetPolicyGroup(ctx, org, group)
			if err != nil {
				return fmt.Errorf("reading policy group %q: %w", group, err)
			}
			current := findAppliedPack(before, name)

			var version *string
			var targetVersionTag string
			switch {
			case len(cliArgs) < 2:
				if current == nil {
					return cmdCmd.ConfigurationError{Message: fmt.Sprintf(
						"policy pack %q is not enabled in policy group %q; specify a version or %q",
						name, group, latestKeyword)}
				}
				targetVersionTag = packVersionTag(*current)
				version = &targetVersionTag
			case cliArgs[1] != latestKeyword:
				targetVersionTag = cliArgs[1]
				version = &targetVersionTag
			}

			merged, err := mergePackSettings(ctx, client, org, name, packMergeInput{
				current:          current,
				targetVersionTag: targetVersionTag,
				replaceConfig:    config,
				changes:          changes,
			})
			if err != nil {
				return err
			}
			warnPackSettings(cmd.ErrOrStderr(), name, changes, merged)

			// Attempt to enable the Policy Pack.
			err = policyPack.Enable(ctx, args.policyGroup,
				backend.PolicyPackOperation{
					VersionTag:   version,
					Scopes:       backend.CancellationScopes,
					Config:       merged.config,
					Environments: merged.environments,
				})
			if err != nil {
				return err
			}

			resp, err := readBackPolicyGroup(ctx, client, policyPack, args.policyGroup)
			if err != nil {
				return err
			}
			return outputFormat.Get()(cmd.OutOrStdout(), resp)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "policy-pack", Usage: "<org-name>/<policy-pack-name>"},
			{Name: "version", Usage: "[latest|version]"},
		},
		Required: 1,
	})

	cmd.PersistentFlags().StringVar(
		&args.policyGroup, "policy-group", "",
		"The Policy Group for which the Policy Pack will be enabled; if not specified, the default Policy Group is used")

	cmd.PersistentFlags().StringVar(
		&args.config, "config", "",
		"The file path for the Policy Pack configuration file; replaces the Policy Pack's whole configuration")

	settings.register(cmd.Flags())
	outputflag.VarP(cmd.Flags(), &outputFormat)
	return cmd
}

func loadPolicyConfigFromFile(file string) (map[string]*json.RawMessage, error) {
	analyzerPolicyConfigMap, err := resourceanalyzer.LoadPolicyPackConfigFromFile(file)
	if err != nil {
		return nil, err
	}

	// Convert type map[string]plugin.AnalyzerPolicyConfig to map[string]*json.RawMessage.
	config := make(map[string]*json.RawMessage)
	for k, v := range analyzerPolicyConfigMap {
		raw, err := marshalAnalyzerPolicyConfig(v)
		if err != nil {
			return nil, err
		}
		config[k] = raw
	}
	return config, nil
}

// marshalAnalyzerPolicyConfig converts the type plugin.AnalyzerPolicyConfig to structure the data
// in a format the way the API service is expecting.
func marshalAnalyzerPolicyConfig(c plugin.AnalyzerPolicyConfig) (*json.RawMessage, error) {
	m := make(map[string]any)
	maps.Copy(m, c.Properties)
	if c.EnforcementLevel != "" {
		m["enforcementLevel"] = c.EnforcementLevel
	}
	bytes, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	raw := json.RawMessage(bytes)
	return &raw, nil
}
