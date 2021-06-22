// Copyright 2016-2020, Pulumi Corporation.
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
	"encoding/json"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	resourceanalyzer "github.com/pulumi/pulumi/pkg/v3/resource/analyzer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

const latestKeyword = "latest"

type policyEnableArgs struct {
	policyGroup string
	config      string
}

func newPolicyEnableCmd() *cobra.Command {
	args := policyEnableArgs{}

	var cmd = &cobra.Command{
		Use:   "enable <org-name>/<policy-pack-name> <latest|version>",
		Args:  cmdutil.ExactArgs(2),
		Short: "Enable a Policy Pack for a Pulumi organization",
		Long: "Enable a Policy Pack for a Pulumi organization. " +
			"Can specify latest to enable the latest version of the Policy Pack or a specific version number.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, cliArgs []string) error {
			// Obtain current PolicyPack, tied to the Pulumi service backend.
			policyPack, err := requirePolicyPack(cliArgs[0])
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
			return policyPack.Enable(commandContext(), args.policyGroup,
				backend.PolicyPackOperation{
					VersionTag: version,
					Scopes:     cancellationScopes,
					Config:     config,
				})
		}),
	}

	cmd.PersistentFlags().StringVar(
		&args.policyGroup, "policy-group", "",
		"The Policy Group for which the Policy Pack will be enabled; if not specified, the default Policy Group is used")

	cmd.PersistentFlags().StringVar(
		&args.config, "config", "",
		"The file path for the Policy Pack configuration file")

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
	m := make(map[string]interface{})
	for k, v := range c.Properties {
		m[k] = v
	}
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
