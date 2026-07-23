// Copyright 2026, Pulumi Corporation.
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
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func newEnvProviderAzureLoginCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "azure-login",
		Short: "Add an Azure login provider to an environment",
		Long: "[EXPERIMENTAL] Add an Azure login provider to an environment\n" +
			"\n" +
			"Subcommands select the authentication mode: `static` for static credentials,\n" +
			"`oidc` for federated identity via OpenID Connect.\n" +
			"\n" +
			"See https://www.pulumi.com/docs/esc/integrations/dynamic-login-credentials/azure-login/\n" +
			"for the full provider reference.\n",
		Args: cobra.NoArgs,
	}

	cmd.AddCommand(newEnvProviderAzureLoginStaticCmd(env))
	cmd.AddCommand(newEnvProviderAzureLoginOIDCCmd(env))

	return cmd
}

func newEnvProviderAzureLoginStaticCmd(env *envCommand) *cobra.Command {
	var pathStr string
	var draft string
	var create bool
	var exportEnvVars bool

	cmd := &cobra.Command{
		Use:   "static [<org>/][<project>/]<environment-name> <tenant-id> <subscription-id> <client-id> <client-secret>",
		Args:  cobra.RangeArgs(4, 5),
		Short: "Add an Azure static-credentials login provider to an environment",
		Long: "[EXPERIMENTAL] Add an Azure static-credentials login provider to an environment\n" +
			"\n" +
			"Writes an `fn::open::azure-login` block at the configured path under `values`.\n" +
			"The client secret is wrapped in `fn::secret`. If a block already exists at the\n" +
			"path it is replaced.\n" +
			"\n" +
			"See https://www.pulumi.com/docs/esc/integrations/dynamic-login-credentials/azure-login/\n" +
			"for the full provider reference.\n",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return errors.New("the provider command does not accept versions")
			}
			if len(args) != 4 {
				return errors.New("expected <tenant-id> <subscription-id> <client-id> <client-secret>")
			}
			tenantID, subscriptionID, clientID, clientSecret := args[0], args[1], args[2], args[3]

			path, err := resource.ParsePropertyPath(pathStr)
			if err != nil {
				return fmt.Errorf("invalid --path: %w", err)
			}

			node := buildAzureLoginStaticNode(clientID, tenantID, subscriptionID, clientSecret)

			var envVars []envVar
			if exportEnvVars {
				envVars = azureLoginStaticEnvVars(propertyPathRef(path))
			}

			if err := ensureProviderEnv(ctx, env, ref, create); err != nil {
				return err
			}
			return applyProviderUpdate(ctx, env, ref, draft, path, node, envVars)
		},
	}

	cmd.Flags().StringVar(&pathStr, "path", "azure.login", "property path under `values` where the provider block is written") //nolint:lll
	cmd.Flags().BoolVar(&exportEnvVars, "export-env-vars", false,
		"also set the Azure environment variables (ARM_CLIENT_ID, etc.) referencing the login outputs")
	cmd.Flags().BoolVar(&create, "create", false,
		"create the environment if it does not already exist")
	cmd.Flags().StringVar(&draft, "draft", "",
		"set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request.") //nolint:lll
	cmd.Flag("draft").NoOptDefVal = "new"

	return cmd
}

// azureLoginOIDCEnvVars returns the ARM_* environment variables that reference the azure-login
// provider's OIDC outputs at pathRef (e.g. "azure.login"), matching the environments the Pulumi
// Cloud console writes. ARM_SUBSCRIPTION_ID is included only when the login block pins a
// subscription; the tenant-scoped environment pins none, so its consumers select their own.
func azureLoginOIDCEnvVars(pathRef string, hasSubscription bool) []envVar {
	vars := []envVar{
		{"ARM_USE_OIDC", "true"},
		{"ARM_CLIENT_ID", "${" + pathRef + ".clientId}"},
		{"ARM_TENANT_ID", "${" + pathRef + ".tenantId}"},
		{"ARM_OIDC_TOKEN", "${" + pathRef + ".oidc.token}"},
	}
	if hasSubscription {
		vars = append(vars, envVar{"ARM_SUBSCRIPTION_ID", "${" + pathRef + ".subscriptionId}"})
	}
	return vars
}

// azureLoginStaticEnvVars returns the ARM_* environment variables that reference the azure-login
// provider's client-secret outputs at pathRef, matching the environments the Pulumi Cloud console
// writes.
func azureLoginStaticEnvVars(pathRef string) []envVar {
	return []envVar{
		{"ARM_CLIENT_ID", "${" + pathRef + ".clientId}"},
		{"ARM_TENANT_ID", "${" + pathRef + ".tenantId}"},
		{"ARM_CLIENT_SECRET", "${" + pathRef + ".clientSecret}"},
		{"ARM_SUBSCRIPTION_ID", "${" + pathRef + ".subscriptionId}"},
	}
}

// buildAzureLoginStaticNode returns a yaml.Node representing
// `fn::open::azure-login: { ... }`. clientSecret is required and wrapped in
// `fn::secret`.
//
// Provider reference:
// https://www.pulumi.com/docs/esc/integrations/dynamic-login-credentials/azure-login/
func buildAzureLoginStaticNode(clientID, tenantID, subscriptionID, clientSecret string) *yaml.Node {
	loginContent := []*yaml.Node{
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "clientId"},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: clientID},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "tenantId"},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: tenantID},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "subscriptionId"},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: subscriptionID},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "clientSecret"},
		secretNode(clientSecret),
	}

	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "fn::open::azure-login"},
			{Kind: yaml.MappingNode, Tag: "!!map", Content: loginContent},
		},
	}
}

func newEnvProviderAzureLoginOIDCCmd(env *envCommand) *cobra.Command {
	var subjectAttributes []string
	var pathStr string
	var draft string
	var create bool
	var exportEnvVars bool

	cmd := &cobra.Command{
		Use:   "oidc [<org>/][<project>/]<environment-name> <tenant-id> <subscription-id> <client-id>",
		Args:  cobra.RangeArgs(3, 4),
		Short: "Add an Azure OIDC login provider to an environment",
		Long: "[EXPERIMENTAL] Add an Azure OIDC login provider to an environment\n" +
			"\n" +
			"Writes an `fn::open::azure-login` block with `oidc: true` at the configured\n" +
			"path under `values`. The Azure federated credential must be provisioned\n" +
			"separately (e.g. with Pulumi). If a block already exists at the path it is\n" +
			"replaced.\n" +
			"\n" +
			"See https://www.pulumi.com/docs/esc/integrations/dynamic-login-credentials/azure-login/\n" +
			"for the full provider reference.\n",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return errors.New("the provider command does not accept versions")
			}
			if len(args) != 3 {
				return errors.New("expected <tenant-id> <subscription-id> <client-id>")
			}
			tenantID, subscriptionID, clientID := args[0], args[1], args[2]

			path, err := resource.ParsePropertyPath(pathStr)
			if err != nil {
				return fmt.Errorf("invalid --path: %w", err)
			}

			node := buildAzureLoginOIDCNode(clientID, tenantID, subscriptionID, subjectAttributes)

			var envVars []envVar
			if exportEnvVars {
				envVars = azureLoginOIDCEnvVars(propertyPathRef(path), subscriptionID != "")
			}

			if err := ensureProviderEnv(ctx, env, ref, create); err != nil {
				return err
			}
			return applyProviderUpdate(ctx, env, ref, draft, path, node, envVars)
		},
	}

	cmd.Flags().StringArrayVar(&subjectAttributes, "subject-attribute", nil,
		"OIDC subject attribute to include in the federated token (repeatable)")
	cmd.Flags().StringVar(&pathStr, "path", "azure.login", "property path under `values` where the provider block is written") //nolint:lll
	cmd.Flags().BoolVar(&exportEnvVars, "export-env-vars", false,
		"also set the Azure environment variables (ARM_CLIENT_ID, etc.) referencing the login outputs")
	cmd.Flags().BoolVar(&create, "create", false,
		"create the environment if it does not already exist")
	cmd.Flags().StringVar(&draft, "draft", "",
		"set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request.") //nolint:lll
	cmd.Flag("draft").NoOptDefVal = "new"

	return cmd
}

// buildAzureLoginOIDCNode returns a yaml.Node representing
// `fn::open::azure-login: { ..., oidc: true }`. subjectAttributes is omitted
// when empty. clientSecret is not supported in OIDC mode.
//
// Provider reference:
// https://www.pulumi.com/docs/esc/integrations/dynamic-login-credentials/azure-login/
func buildAzureLoginOIDCNode(clientID, tenantID, subscriptionID string, subjectAttributes []string) *yaml.Node {
	loginContent := []*yaml.Node{
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "clientId"},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: clientID},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "tenantId"},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: tenantID},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "subscriptionId"},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: subscriptionID},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "oidc"},
		{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"},
	}
	if len(subjectAttributes) > 0 {
		loginContent = append(loginContent,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "subjectAttributes"},
			stringSequenceNode(subjectAttributes),
		)
	}

	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "fn::open::azure-login"},
			{Kind: yaml.MappingNode, Tag: "!!map", Content: loginContent},
		},
	}
}
