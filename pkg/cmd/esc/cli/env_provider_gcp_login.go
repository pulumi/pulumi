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
	"strconv"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func newEnvProviderGCPLoginCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gcp-login",
		Short: "Add a GCP login provider to an environment",
		Long: "[EXPERIMENTAL] Add a GCP login provider to an environment\n" +
			"\n" +
			"Subcommands select the authentication mode: `static` for static credentials,\n" +
			"`oidc` for federated identity via OpenID Connect.\n" +
			"\n" +
			"See https://www.pulumi.com/docs/esc/integrations/dynamic-login-credentials/gcp-login/\n" +
			"for the full provider reference.\n",
		Args: cobra.NoArgs,
	}

	cmd.AddCommand(newEnvProviderGCPLoginStaticCmd(env))
	cmd.AddCommand(newEnvProviderGCPLoginOIDCCmd(env))

	return cmd
}

func newEnvProviderGCPLoginStaticCmd(env *envCommand) *cobra.Command {
	var serviceAccount string
	var tokenLifetime string
	var pathStr string
	var draft string
	var create bool

	cmd := &cobra.Command{
		Use:   "static [<org>/][<project>/]<environment-name> <project-number> <access-token>",
		Args:  cobra.RangeArgs(2, 3),
		Short: "Add a GCP static-credentials login provider to an environment",
		Long: "[EXPERIMENTAL] Add a GCP static-credentials login provider to an environment\n" +
			"\n" +
			"Writes an `fn::open::gcp-login` block at the configured path under `values`. The\n" +
			"access token is wrapped in `fn::secret`. <project-number> must be the numerical\n" +
			"GCP project ID. If a block already exists at the path it is replaced.\n" +
			"\n" +
			"See https://www.pulumi.com/docs/esc/integrations/dynamic-login-credentials/gcp-login/\n" +
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
			if len(args) != 2 {
				return errors.New("expected <project-number> and <access-token>")
			}
			project, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid project number %q: must be a positive integer", args[0])
			}
			if project <= 0 {
				return fmt.Errorf("invalid project number %q: must be a positive integer", args[0])
			}
			accessToken := args[1]

			path, err := resource.ParsePropertyPath(pathStr)
			if err != nil {
				return fmt.Errorf("invalid --path: %w", err)
			}

			node := buildGCPLoginStaticNode(project, accessToken, serviceAccount, tokenLifetime)

			if err := ensureProviderEnv(ctx, env, ref, create); err != nil {
				return err
			}
			return applyProviderUpdate(ctx, env, ref, draft, path, node)
		},
	}

	cmd.Flags().StringVar(&serviceAccount, "service-account", "", "optional GCP service account to impersonate")
	cmd.Flags().
		StringVar(&tokenLifetime, "token-lifetime", "", "optional lifetime for impersonated credentials, e.g. 1h30m")
	cmd.Flags().
		StringVar(&pathStr, "path", "gcp.login", "property path under `values` where the provider block is written")
	cmd.Flags().BoolVar(&create, "create", false,
		"create the environment if it does not already exist")
	cmd.Flags().StringVar(&draft, "draft", "",
		"set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request.") //nolint:lll
	cmd.Flag("draft").NoOptDefVal = "new"

	return cmd
}

// buildGCPLoginStaticNode returns a yaml.Node representing
// `fn::open::gcp-login: { project, accessToken: { accessToken: {fn::secret}, ... } }`.
// serviceAccount and tokenLifetime are omitted when empty.
//
// Provider reference:
// https://www.pulumi.com/docs/esc/integrations/dynamic-login-credentials/gcp-login/
func buildGCPLoginStaticNode(project int64, accessToken, serviceAccount, tokenLifetime string) *yaml.Node {
	accessTokenContent := []*yaml.Node{
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "accessToken"},
		secretNode(accessToken),
	}
	if serviceAccount != "" {
		accessTokenContent = append(accessTokenContent,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "serviceAccount"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: serviceAccount},
		)
	}
	if tokenLifetime != "" {
		accessTokenContent = append(accessTokenContent,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "tokenLifetime"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: tokenLifetime},
		)
	}

	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "fn::open::gcp-login"},
			{
				Kind: yaml.MappingNode,
				Tag:  "!!map",
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "project"},
					{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.FormatInt(project, 10)},
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "accessToken"},
					{Kind: yaml.MappingNode, Tag: "!!map", Content: accessTokenContent},
				},
			},
		},
	}
}

func newEnvProviderGCPLoginOIDCCmd(env *envCommand) *cobra.Command {
	var workloadPoolID string
	var providerID string
	var serviceAccount string
	var region string
	var tokenLifetime string
	var subjectAttributes []string
	var pathStr string
	var draft string
	var create bool

	cmd := &cobra.Command{
		Use:   "oidc [<org>/][<project>/]<environment-name> <project-number>",
		Args:  cobra.RangeArgs(1, 2),
		Short: "Add a GCP OIDC login provider to an environment",
		Long: "[EXPERIMENTAL] Add a GCP OIDC login provider to an environment\n" +
			"\n" +
			"Writes an `fn::open::gcp-login` block with an `oidc` workload-identity\n" +
			"federation block at the configured path under `values`. <project-number> must\n" +
			"be the numerical GCP project ID. The workload-identity pool, provider, and\n" +
			"service account must be provisioned separately (e.g. with Pulumi). If a block\n" +
			"already exists at the path it is replaced.\n" +
			"\n" +
			"See https://www.pulumi.com/docs/esc/integrations/dynamic-login-credentials/gcp-login/\n" +
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
			if len(args) != 1 {
				return errors.New("expected <project-number>")
			}
			project, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid project number %q: must be a positive integer", args[0])
			}
			if project <= 0 {
				return fmt.Errorf("invalid project number %q: must be a positive integer", args[0])
			}

			path, err := resource.ParsePropertyPath(pathStr)
			if err != nil {
				return fmt.Errorf("invalid --path: %w", err)
			}

			node := buildGCPLoginOIDCNode(
				project,
				workloadPoolID,
				providerID,
				serviceAccount,
				region,
				tokenLifetime,
				subjectAttributes,
			)

			if err := ensureProviderEnv(ctx, env, ref, create); err != nil {
				return err
			}
			return applyProviderUpdate(ctx, env, ref, draft, path, node)
		},
	}

	cmd.Flags().StringVar(&workloadPoolID, "workload-pool-id", "", "GCP workload identity pool ID (required)")
	cmd.Flags().StringVar(&providerID, "provider-id", "", "GCP workload identity pool provider ID (required)")
	cmd.Flags().StringVar(&serviceAccount, "service-account", "", "GCP service account to impersonate (required)")
	cmd.Flags().StringVar(&region, "region", "", "optional GCP region for the workload identity pool")
	cmd.Flags().
		StringVar(&tokenLifetime, "token-lifetime", "", "optional lifetime for impersonated credentials, e.g. 1h30m")
	cmd.Flags().StringArrayVar(&subjectAttributes, "subject-attribute", nil,
		"OIDC subject attribute to include in the federated token (repeatable)")
	cmd.Flags().
		StringVar(&pathStr, "path", "gcp.login", "property path under `values` where the provider block is written")
	cmd.Flags().BoolVar(&create, "create", false,
		"create the environment if it does not already exist")
	cmd.Flags().StringVar(&draft, "draft", "",
		"set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request.") //nolint:lll
	cmd.Flag("draft").NoOptDefVal = "new"

	_ = cmd.MarkFlagRequired("workload-pool-id")
	_ = cmd.MarkFlagRequired("provider-id")
	_ = cmd.MarkFlagRequired("service-account")

	return cmd
}

// buildGCPLoginOIDCNode returns a yaml.Node representing
// `fn::open::gcp-login: { project, oidc: {...} }`. region, tokenLifetime, and
// subjectAttributes are omitted when empty.
//
// Provider reference:
// https://www.pulumi.com/docs/esc/integrations/dynamic-login-credentials/gcp-login/
func buildGCPLoginOIDCNode(
	project int64,
	workloadPoolID, providerID, serviceAccount, region, tokenLifetime string,
	subjectAttributes []string,
) *yaml.Node {
	oidcContent := []*yaml.Node{
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "workloadPoolId"},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: workloadPoolID},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "providerId"},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: providerID},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "serviceAccount"},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: serviceAccount},
	}
	if region != "" {
		oidcContent = append(oidcContent,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "region"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: region},
		)
	}
	if tokenLifetime != "" {
		oidcContent = append(oidcContent,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "tokenLifetime"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: tokenLifetime},
		)
	}
	if len(subjectAttributes) > 0 {
		oidcContent = append(oidcContent,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "subjectAttributes"},
			stringSequenceNode(subjectAttributes),
		)
	}

	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "fn::open::gcp-login"},
			{
				Kind: yaml.MappingNode,
				Tag:  "!!map",
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "project"},
					{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.FormatInt(project, 10)},
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "oidc"},
					{Kind: yaml.MappingNode, Tag: "!!map", Content: oidcContent},
				},
			},
		},
	}
}
