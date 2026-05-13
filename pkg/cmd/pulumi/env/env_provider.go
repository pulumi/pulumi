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

package env

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// TODO[https://github.com/pulumi/pulumi/issues/23043]: Not yet implemented.
func newEnvProviderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "provider",
		Short:  "Manage environment OIDC providers",
		Long:   "[EXPERIMENTAL] Manage environment OIDC providers.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newEnvProviderNewCmd())
	return cmd
}

func newEnvProviderNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "new",
		Short:  "Register a new OIDC issuer for an organization",
		Long:   "[EXPERIMENTAL] Register a new OIDC issuer for an organization.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newEnvProviderNewAWSCmd())
	cmd.AddCommand(newEnvProviderNewAWSSSOCmd())
	cmd.AddCommand(newEnvProviderNewAzureCmd())
	cmd.AddCommand(newEnvProviderNewGCPCmd())
	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23040]: Not yet implemented.
func newEnvProviderNewAWSCmd() *cobra.Command {
	var (
		org    string
		issuer string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "aws",
		Short:  "Register an AWS OIDC issuer",
		Long:   "[EXPERIMENTAL] Register an AWS OIDC issuer.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "name"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization to register the issuer in")
	cmd.Flags().StringVar(&issuer, "issuer", "", "The OIDC issuer URL")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23039]: Not yet implemented.
func newEnvProviderNewAWSSSOCmd() *cobra.Command {
	var (
		org    string
		issuer string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "aws-sso",
		Short:  "Register an AWS SSO OIDC issuer",
		Long:   "[EXPERIMENTAL] Register an AWS SSO OIDC issuer.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "name"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization to register the issuer in")
	cmd.Flags().StringVar(&issuer, "issuer", "", "The OIDC issuer URL")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23038]: Not yet implemented.
func newEnvProviderNewAzureCmd() *cobra.Command {
	var (
		org    string
		issuer string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "azure",
		Short:  "Register an Azure OIDC issuer",
		Long:   "[EXPERIMENTAL] Register an Azure OIDC issuer.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "name"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization to register the issuer in")
	cmd.Flags().StringVar(&issuer, "issuer", "", "The OIDC issuer URL")

	return cmd
}

// TODO[https://github.com/pulumi/pulumi/issues/23037]: Not yet implemented.
func newEnvProviderNewGCPCmd() *cobra.Command {
	var (
		org    string
		issuer string
	)

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "gcp",
		Short:  "Register a Google Cloud OIDC issuer",
		Long:   "[EXPERIMENTAL] Register a Google Cloud OIDC issuer.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "name"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&org, "org", "", "The organization to register the issuer in")
	cmd.Flags().StringVar(&issuer, "issuer", "", "The OIDC issuer URL")

	return cmd
}
