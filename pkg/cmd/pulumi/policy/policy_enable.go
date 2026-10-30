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
	"fmt"
	"maps"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	resourceanalyzer "github.com/pulumi/pulumi/pkg/v3/resource/analyzer"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/spf13/cobra"
)

const latestKeyword = "latest"

type policyEnableArgs struct {
	policyGroup string
	config      string
}

// requirePolicyPackAndClient logs into the cloud backend and retrieves the requested policy pack along with
// a client for reading back the result of an update.
func requirePolicyPackAndClient(
	ctx context.Context, ws pkgWorkspace.Context, lm cmdBackend.LoginManager, policyPackRef string,
) (backend.PolicyPack, policyGroupGetClient, error) {
	b, err := loginToCloudBackend(ctx, ws, lm)
	if err != nil {
		return nil, nil, err
	}
	client, err := cloudClient(b)
	if err != nil {
		return nil, nil, err
	}
	policyPack, err := requirePolicyPackForBackend(ctx, policyPackRef, b)
	if err != nil {
		return nil, nil, err
	}
	return policyPack, client, nil
}

// readBackPolicyGroup fetches the group a pack was just enabled in or disabled from, since the update endpoint
// returns nothing.
func readBackPolicyGroup(
	ctx context.Context, client policyGroupGetClient, policyPack backend.PolicyPack, group string,
) (apitype.GetPolicyGroupResponse, error) {
	if group == "" {
		group = apitype.DefaultPolicyGroup
	}
	resp, err := client.GetPolicyGroup(ctx, policyPack.Ref().OrgName(), group)
	if err != nil {
		return apitype.GetPolicyGroupResponse{}, fmt.Errorf("reading policy group %q after update: %w", group, err)
	}
	return resp, nil
}

func newPolicyEnableCmd(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
	args := policyEnableArgs{}
	outputFormat := defaultPolicyGroupGetOutputFormat()

	cmd := &cobra.Command{
		Use:   "enable",
		Short: "Enable a Policy Pack for a Pulumi organization",
		Long: "Enable a Policy Pack for a Pulumi organization. " +
			"Can specify latest to enable the latest version of the Policy Pack or a specific version number.\n" +
			"\n" +
			"Enabling replaces the Policy Pack's configuration and ESC environments in the Policy Group with those " +
			"in the --config file. Without --config, both are cleared.\n" +
			"\n" +
			"Examples:\n" +
			"  pulumi policy enable acme/aws-guardrails latest\n" +
			"  pulumi policy enable acme/aws-guardrails 3 --policy-group prod --output json",
		RunE: func(cmd *cobra.Command, cliArgs []string) error {
			ctx := cmd.Context()
			// Obtain current PolicyPack, tied to the Pulumi Cloud backend.
			policyPack, client, err := requirePolicyPackAndClient(ctx, ws, lm, cliArgs[0])
			if err != nil {
				return err
			}

			// Parse version if it's specified.
			var version *string
			if cliArgs[1] != latestKeyword {
				version = &cliArgs[1]
			}

			// Load the configuration from the user-specified JSON file into config object.
			var file policyConfigFile
			if args.config != "" {
				file, err = loadPolicyConfigFromFile(args.config)
				if err != nil {
					return err
				}
				if file.exceptions.Len() > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"warning: ignoring the exceptions in %s; pulumi policy enable does not apply them\n", args.config)
				}
			}

			// Attempt to enable the Policy Pack.
			err = policyPack.Enable(ctx, args.policyGroup,
				backend.PolicyPackOperation{
					VersionTag:   version,
					Scopes:       backend.CancellationScopes,
					Config:       file.config,
					Environments: file.environments,
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
			{Name: "version", Usage: "<latest|version>"},
		},
		Required: 2,
	})

	cmd.PersistentFlags().StringVar(
		&args.policyGroup, "policy-group", "",
		"The Policy Group for which the Policy Pack will be enabled; if not specified, the default Policy Group is used")

	cmd.PersistentFlags().StringVar(
		&args.config, "config", "",
		"The file path for the Policy Pack configuration file; ESC environments listed in its \"environments\" key "+
			"are attached to the Policy Pack")

	outputflag.VarP(cmd.Flags(), &outputFormat)
	return cmd
}

// policyConfigFile is a policy pack config file, with its policy config in the form the service expects.
type policyConfigFile struct {
	config       map[string]*json.RawMessage
	environments []string
	exceptions   resourceanalyzer.PolicyExceptions
}

func loadPolicyConfigFromFile(path string) (policyConfigFile, error) {
	file, err := resourceanalyzer.LoadPolicyPackConfigFile(path)
	if err != nil {
		return policyConfigFile{}, err
	}

	// Convert type map[string]plugin.AnalyzerPolicyConfig to map[string]*json.RawMessage.
	config := make(map[string]*json.RawMessage)
	for k, v := range file.Config {
		raw, err := marshalAnalyzerPolicyConfig(v)
		if err != nil {
			return policyConfigFile{}, err
		}
		config[k] = raw
	}
	return policyConfigFile{config: config, environments: file.Environments, exceptions: file.Exceptions}, nil
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
