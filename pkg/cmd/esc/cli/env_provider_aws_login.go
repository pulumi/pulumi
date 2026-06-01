// Copyright 2026, Pulumi Corporation.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func newEnvProviderAWSLoginCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aws-login",
		Short: "Add an AWS login provider to an environment",
		Long: "[EXPERIMENTAL] Add an AWS login provider to an environment\n" +
			"\n" +
			"Subcommands select the authentication mode: `static` for static credentials,\n" +
			"`oidc` for federated identity via OpenID Connect.\n" +
			"\n" +
			"See https://www.pulumi.com/docs/esc/integrations/dynamic-login-credentials/aws-login/\n" +
			"for the full provider reference.\n",
		Args: cobra.NoArgs,
	}

	cmd.AddCommand(newEnvProviderAWSLoginStaticCmd(env))
	cmd.AddCommand(newEnvProviderAWSLoginOIDCCmd(env))

	return cmd
}

func newEnvProviderAWSLoginStaticCmd(env *envCommand) *cobra.Command {
	var sessionToken string
	var pathStr string
	var draft string
	var create bool

	cmd := &cobra.Command{
		Use:   "static [<org>/][<project>/]<environment-name> <access-key-id> <secret-access-key>",
		Args:  cobra.RangeArgs(2, 3),
		Short: "Add an AWS static-credentials login provider to an environment",
		Long: "[EXPERIMENTAL] Add an AWS static-credentials login provider to an environment\n" +
			"\n" +
			"Writes an `fn::open::aws-login` block with static credentials at the configured\n" +
			"path under `values`. The secret access key and session token, if any, are\n" +
			"wrapped in `fn::secret`. If a block already exists at the path it is replaced.\n" +
			"\n" +
			"See https://www.pulumi.com/docs/esc/integrations/dynamic-login-credentials/aws-login/\n" +
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
				return fmt.Errorf("the provider command does not accept versions")
			}
			if len(args) != 2 {
				return fmt.Errorf("expected <access-key-id> and <secret-access-key>")
			}
			accessKeyID, secretAccessKey := args[0], args[1]

			path, err := resource.ParsePropertyPath(pathStr)
			if err != nil {
				return fmt.Errorf("invalid --path: %w", err)
			}

			node := buildAWSLoginStaticNode(accessKeyID, secretAccessKey, sessionToken)

			if err := ensureProviderEnv(ctx, env, ref, create); err != nil {
				return err
			}
			return applyProviderUpdate(ctx, env, ref, draft, path, node)
		},
	}

	cmd.Flags().StringVar(&sessionToken, "session-token", "", "optional AWS session token")
	cmd.Flags().StringVar(&pathStr, "path", "aws.login", "property path under `values` where the provider block is written")
	cmd.Flags().BoolVar(&create, "create", false,
		"create the environment if it does not already exist")
	cmd.Flags().StringVar(&draft, "draft", "",
		"set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request.")
	cmd.Flag("draft").NoOptDefVal = "new"

	return cmd
}

// buildAWSLoginStaticNode returns a yaml.Node representing
// `fn::open::aws-login: { static: {...} }`. secretAccessKey and sessionToken
// are wrapped in `fn::secret`. sessionToken is omitted when empty.
//
// Provider reference:
// https://www.pulumi.com/docs/esc/integrations/dynamic-login-credentials/aws-login/
func buildAWSLoginStaticNode(accessKeyID, secretAccessKey, sessionToken string) *yaml.Node {
	staticContent := []*yaml.Node{
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "accessKeyId"},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: accessKeyID},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "secretAccessKey"},
		secretNode(secretAccessKey),
	}
	if sessionToken != "" {
		staticContent = append(staticContent,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "sessionToken"},
			secretNode(sessionToken),
		)
	}

	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "fn::open::aws-login"},
			{
				Kind: yaml.MappingNode,
				Tag:  "!!map",
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "static"},
					{Kind: yaml.MappingNode, Tag: "!!map", Content: staticContent},
				},
			},
		},
	}
}

func newEnvProviderAWSLoginOIDCCmd(env *envCommand) *cobra.Command {
	var duration string
	var policyArns []string
	var subjectAttributes []string
	var pathStr string
	var draft string
	var create bool

	cmd := &cobra.Command{
		Use:   "oidc [<org>/][<project>/]<environment-name> <role-arn> <session-name>",
		Args:  cobra.RangeArgs(2, 3),
		Short: "Add an AWS OIDC login provider to an environment",
		Long: "[EXPERIMENTAL] Add an AWS OIDC login provider to an environment\n" +
			"\n" +
			"Writes an `fn::open::aws-login` block with an `oidc` federation block at the\n" +
			"configured path under `values`. The OIDC IAM role and trust policy must be\n" +
			"provisioned separately (e.g. with Pulumi). If a block already exists at the\n" +
			"path it is replaced.\n" +
			"\n" +
			"See https://www.pulumi.com/docs/esc/integrations/dynamic-login-credentials/aws-login/\n" +
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
				return fmt.Errorf("the provider command does not accept versions")
			}
			if len(args) != 2 {
				return fmt.Errorf("expected <role-arn> and <session-name>")
			}
			roleArn, sessionName := args[0], args[1]

			path, err := resource.ParsePropertyPath(pathStr)
			if err != nil {
				return fmt.Errorf("invalid --path: %w", err)
			}

			node := buildAWSLoginOIDCNode(roleArn, sessionName, duration, policyArns, subjectAttributes)

			if err := ensureProviderEnv(ctx, env, ref, create); err != nil {
				return err
			}
			return applyProviderUpdate(ctx, env, ref, draft, path, node)
		},
	}

	cmd.Flags().StringVar(&duration, "duration", "", "optional session duration, e.g. 1h")
	cmd.Flags().StringArrayVar(&policyArns, "policy-arn", nil,
		"AWS managed-policy ARN to attach to the role session (repeatable)")
	cmd.Flags().StringArrayVar(&subjectAttributes, "subject-attribute", nil,
		"OIDC subject attribute to include in the session token (repeatable)")
	cmd.Flags().StringVar(&pathStr, "path", "aws.login", "property path under `values` where the provider block is written")
	cmd.Flags().BoolVar(&create, "create", false,
		"create the environment if it does not already exist")
	cmd.Flags().StringVar(&draft, "draft", "",
		"set flag without a value (--draft) to create a draft rather than saving changes directly. --draft=<change-request-id> to update an existing change request.")
	cmd.Flag("draft").NoOptDefVal = "new"

	return cmd
}

// buildAWSLoginOIDCNode returns a yaml.Node representing
// `fn::open::aws-login: { oidc: {...} }`. duration, policyArns, and
// subjectAttributes are omitted when empty.
//
// Provider reference:
// https://www.pulumi.com/docs/esc/integrations/dynamic-login-credentials/aws-login/
func buildAWSLoginOIDCNode(roleArn, sessionName, duration string, policyArns, subjectAttributes []string) *yaml.Node {
	oidcContent := []*yaml.Node{
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "roleArn"},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: roleArn},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "sessionName"},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: sessionName},
	}
	if duration != "" {
		oidcContent = append(oidcContent,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "duration"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: duration},
		)
	}
	if len(policyArns) > 0 {
		oidcContent = append(oidcContent,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "policyArns"},
			stringSequenceNode(policyArns),
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
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "fn::open::aws-login"},
			{
				Kind: yaml.MappingNode,
				Tag:  "!!map",
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "oidc"},
					{Kind: yaml.MappingNode, Tag: "!!map", Content: oidcContent},
				},
			},
		},
	}
}
