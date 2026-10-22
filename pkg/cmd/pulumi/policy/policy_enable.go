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
	"io"
	"maps"
	"slices"
	"strconv"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
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

type policyEnableResult struct {
	Organization string                             `json:"organization"`
	PolicyGroup  string                             `json:"policyGroup"`
	PolicyPack   cloudapitype.AppPolicyPackMetadata `json:"policyPack"`
}

type policyEnableRenderFunc func(w io.Writer, result policyEnableResult) error

func renderPolicyEnableText(w io.Writer, result policyEnableResult) error {
	version := result.PolicyPack.VersionTag
	if version == "" {
		version = strconv.Itoa(result.PolicyPack.Version)
	}
	fmt.Fprintf(w, "Enabled policy pack %s/%s version %s in policy group %s.\n",
		result.Organization, result.PolicyPack.Name, version, result.PolicyGroup)
	return nil
}

func renderPolicyEnableJSON(w io.Writer, result policyEnableResult) error {
	return ui.FprintJSON(w, result)
}

func newPolicyEnableCmd() *cobra.Command {
	return newPolicyEnableCmdWith(defaultPolicyPackCmdDeps)
}

func newPolicyEnableCmdWith(deps policyPackCmdDeps) *cobra.Command {
	contract.Assertf(deps != nil, "policyPackCmdDeps must not be nil")
	args := policyEnableArgs{}
	outputFormat := outputflag.OutputFlag[policyEnableRenderFunc]{
		RenderForTerminal: renderPolicyEnableText,
		RenderJSON:        renderPolicyEnableJSON,
	}

	cmd := &cobra.Command{
		Use:   "enable",
		Short: "Enable a Policy Pack for a Pulumi organization",
		Long: "Enable a Policy Pack for a Pulumi organization. " +
			"Can specify latest to enable the latest version of the Policy Pack or a specific version number.\n" +
			"\n" +
			"Examples:\n" +
			"  pulumi policy enable acme/aws-guardrails latest\n" +
			"  pulumi policy enable acme/aws-guardrails 3 --policy-group prod --output json",
		RunE: func(cmd *cobra.Command, cliArgs []string) error {
			ctx := cmd.Context()
			// Obtain current PolicyPack, tied to the Pulumi Cloud backend.
			policyPack, client, err := deps(ctx, cliArgs[0])
			if err != nil {
				return err
			}

			// Parse version if it's specified.
			var version *string
			if cliArgs[1] != latestKeyword {
				version = &cliArgs[1]
			}

			// Load the configuration from the user-specified JSON file into config object.
			var config map[string]*json.RawMessage
			if args.config != "" {
				config, err = loadPolicyConfigFromFile(args.config)
				if err != nil {
					return err
				}
			}

			// Attempt to enable the Policy Pack.
			err = policyPack.Enable(ctx, args.policyGroup,
				backend.PolicyPackOperation{
					VersionTag: version,
					Scopes:     backend.CancellationScopes,
					Config:     config,
				})
			if err != nil {
				return err
			}

			// The enable endpoint returns nothing, so read the group back to report the version that was resolved.
			org := policyPack.Ref().OrgName()
			name := string(policyPack.Ref().Name())
			group := args.policyGroup
			if group == "" {
				group = apitype.DefaultPolicyGroup
			}
			resp, err := client.GetPolicyGroup(ctx, org, group)
			if err != nil {
				return fmt.Errorf("policy pack %q was enabled, but reading back policy group %q failed: %w",
					name, group, err)
			}
			idx := slices.IndexFunc(resp.AppliedPolicyPacks, func(p cloudapitype.AppPolicyPackMetadata) bool {
				return p.Name == name
			})
			if idx < 0 {
				return fmt.Errorf("policy pack %q not found in policy group %q after enabling", name, group)
			}
			return outputFormat.Get()(cmd.OutOrStdout(), policyEnableResult{
				Organization: org,
				PolicyGroup:  group,
				PolicyPack:   resp.AppliedPolicyPacks[idx],
			})
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
		"The file path for the Policy Pack configuration file")

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
