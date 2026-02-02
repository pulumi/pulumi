// Copyright 2016-2025, Pulumi Corporation.
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

package insights

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type accountsCreateArgs struct {
	profile    string
	regions    []string
	allRegions bool
	roleName   string
	escEnv     string
	escProject string
	schedule   string
	yes        bool
	skipOIDC   bool
}

// newAccountsCreateCmd creates the `pulumi insights accounts create` command.
func newAccountsCreateCmd() *cobra.Command {
	args := &accountsCreateArgs{}

	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create an Insights discovery account",
		Long: "Create an Insights discovery account for AWS.\n" +
			"\n" +
			"This command sets up OIDC authentication, creates an ESC environment,\n" +
			"and registers a discovery account with Pulumi Insights.\n" +
			"\n" +
			"If no name is provided, defaults to aws-{accountID}.",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			displayOpts := getDisplayOptions()

			// Step 1: Get cloud backend and org
			cloudBackend, orgName, err := ensureCloudBackend(cmd, ws)
			if err != nil {
				return err
			}

			// Step 2: Select AWS profile
			profile := args.profile
			if profile == "" && cmdutil.Interactive() && !args.yes {
				profile, err = promptForAWSProfile(displayOpts)
				if err != nil {
					return err
				}
			}
			if profile == "" {
				profile = "default"
			}

			fmt.Printf("Detecting AWS credentials for profile %q...\n", profile)

			// Step 3: Detect AWS credentials
			awsCfg, err := detectAWSCredentials(ctx, profile)
			if err != nil {
				return err
			}
			fmt.Printf("Detected AWS account: %s (partition: %s)\n", awsCfg.AccountID, awsCfg.Partition)

			// Step 4: Determine account name
			accountName := ""
			if len(posArgs) > 0 {
				accountName = posArgs[0]
			}
			if accountName == "" {
				accountName = fmt.Sprintf("aws-%s", awsCfg.AccountID)
			}

			// Step 5: Determine regions
			regions := args.regions
			if args.allRegions {
				regions = allAWSRegions
			}
			if len(regions) == 0 {
				if cmdutil.Interactive() && !args.yes {
					regions = promptForRegions(displayOpts)
				}
				if len(regions) == 0 {
					regions = defaultRegions
				}
			}
			awsCfg.Regions = regions

			// Step 6: Determine role name
			roleName := args.roleName
			if roleName == "" {
				roleName = awsCfg.DefaultRoleName()
			}

			// Step 7: Setup OIDC (unless skipped)
			if !args.skipOIDC {
				// Show the user what will be created in their AWS account
				fmt.Printf("\nThe following resources will be created in AWS account %s:\n", awsCfg.AccountID)
				fmt.Printf("  OIDC Provider: %s\n", oidcIssuerURL)
				fmt.Printf("  IAM Role:      %s\n", roleName)
				fmt.Printf("  Policy:        ReadOnlyAccess (arn:aws:iam::aws:policy/ReadOnlyAccess)\n")
				fmt.Printf("  Trust:         %s (audience: aws:%s)\n", oidcIssuerURL, orgName)

				if cmdutil.Interactive() && !args.yes {
					option := ui.PromptUser(
						"Proceed with OIDC setup?",
						[]string{"yes", "no"},
						"yes",
						displayOpts.Color,
					)
					if option != "yes" {
						fmt.Println("Account creation cancelled.")
						return nil
					}
				}

				fmt.Println("\nSetting up OIDC authentication...")
				iamCli, iamErr := newIAMClient(ctx, profile)
				if iamErr != nil {
					return fmt.Errorf("creating IAM client: %w\n\n"+
						"Ensure your AWS credentials have IAM permissions:\n"+
						"  - iam:CreateOpenIDConnectProvider\n"+
						"  - iam:AddClientIDToOpenIDConnectProvider\n"+
						"  - iam:CreateRole\n"+
						"  - iam:AttachRolePolicy\n\n"+
						"Alternatively, set up OIDC manually and use --skip-oidc flag:\n"+
						"  https://www.pulumi.com/docs/insights/discovery/get-started/begin/", iamErr)
				}

				oidcResult, oidcErr := setupOIDC(ctx, iamCli, awsCfg, orgName, roleName)
				if oidcErr != nil {
					// Check if this is a role-exists error
					var roleExists *roleExistsError
					if errors.As(oidcErr, &roleExists) {
						// In interactive mode, give the user a choice
						if cmdutil.Interactive() && !args.yes {
							fmt.Printf("\n%s\n", oidcErr.Error())
							fmt.Printf("\nThe existing role's trust policy does not allow audience %q.\n", roleExists.audience)
							fmt.Printf("It may be configured for a different organization.\n")
							fmt.Printf("\nWhat would you like to do?\n")
							option := ui.PromptUser(
								"",
								[]string{"Continue with --skip-oidc (use existing role as-is)", "Cancel and fix manually"},
								"Cancel and fix manually",
								displayOpts.Color,
							)
							if option == "Continue with --skip-oidc (use existing role as-is)" {
								// Switch to skip-oidc mode
								args.skipOIDC = true
								awsCfg.RoleARN = fmt.Sprintf("arn:%s:iam::%s:role/%s",
									awsCfg.Partition, awsCfg.AccountID, roleName)
								fmt.Printf("Continuing with existing role: %s\n", awsCfg.RoleARN)
								goto skipOIDC // Skip to the ESC environment step
							} else {
								// User chose to cancel
								return fmt.Errorf("account creation cancelled\n\n"+
									"The existing role's trust policy does not allow audience %q.\n"+
									"To fix this, either:\n"+
									"  - Delete the existing role in the AWS console and re-run this command\n"+
									"  - Manually update the role's trust policy to allow audience %q\n"+
									"  - Re-run with --skip-oidc to use the role as-is",
									roleExists.audience, roleExists.audience)
							}
						} else {
							// Non-interactive mode, just return the error with details
							return fmt.Errorf("setting up OIDC: %w\n\n"+
								"The existing role's trust policy does not allow audience %q.\n"+
								"To fix this, either:\n"+
								"  - Delete the existing role in the AWS console and re-run this command\n"+
								"  - Manually update the role's trust policy to allow audience %q\n"+
								"  - Re-run with --skip-oidc to use the role as-is",
								oidcErr, roleExists.audience, roleExists.audience)
						}
					}
					return fmt.Errorf("setting up OIDC: %w\n\n"+
						"Ensure your AWS credentials have IAM permissions:\n"+
						"  - iam:CreateOpenIDConnectProvider\n"+
						"  - iam:AddClientIDToOpenIDConnectProvider\n"+
						"  - iam:CreateRole\n"+
						"  - iam:AttachRolePolicy\n\n"+
						"Alternatively, set up OIDC manually and use --skip-oidc flag:\n"+
						"  https://www.pulumi.com/docs/insights/discovery/get-started/begin/", oidcErr)
				}

				awsCfg.OIDCProviderARN = oidcResult.OIDCProviderARN
				awsCfg.RoleARN = oidcResult.RoleARN

				if oidcResult.OIDCProviderNew {
					fmt.Printf("Created OIDC provider: %s\n", oidcResult.OIDCProviderARN)
				} else {
					fmt.Printf("Using existing OIDC provider: %s\n", oidcResult.OIDCProviderARN)
				}
				if oidcResult.RoleNew {
					fmt.Printf("Created IAM role: %s (with ReadOnlyAccess)\n", roleName)
				} else {
					fmt.Printf("Using existing IAM role: %s\n", roleName)
				}
			} else {
				// If skipping OIDC, build the role ARN from the role name
				awsCfg.RoleARN = fmt.Sprintf("arn:%s:iam::%s:role/%s",
					awsCfg.Partition, awsCfg.AccountID, roleName)
				fmt.Printf("Skipping OIDC setup, using role: %s\n", awsCfg.RoleARN)
			}

		skipOIDC:
			// Step 8: Determine ESC environment
			escProject := args.escProject
			if escProject == "" {
				escProject = defaultESCProject
			}
			escEnvName := ""
			if args.escEnv != "" {
				// Parse the user-provided env reference
				_, escProject, escEnvName, err = parseESCEnvironmentRef(args.escEnv, orgName)
				if err != nil {
					return err
				}
			} else {
				escEnvName = defaultESCEnvName(awsCfg.AccountID)
			}

			// Step 9: Create ESC environment
			escRef := escEnvironmentRef(orgName, escProject, escEnvName)
			fmt.Printf("\nCreating ESC environment: %s\n", escRef)

			envBackend, ok := cloudBackend.(pkgBackend.EnvironmentsBackend)
			if !ok {
				return fmt.Errorf("current backend does not support environments")
			}

			err = createESCEnvironment(ctx, envBackend, orgName, escProject, escEnvName, awsCfg.RoleARN)
			if err != nil {
				// Check if this is a conflict error (environment already exists)
				if strings.Contains(err.Error(), "[409]") || strings.Contains(err.Error(), "already exists") {
					return fmt.Errorf("ESC environment %q already exists\n\n"+
						"The environment was likely created during a previous run.\n"+
						"To use the existing environment, re-run with --skip-oidc", escRef)
				}
				return err
			}
			fmt.Printf("Created ESC environment: %s\n", escRef)

			// Step 10: Determine scan schedule
			schedule := args.schedule
			if schedule == "" {
				if cmdutil.Interactive() && !args.yes {
					schedule = promptForSchedule(displayOpts)
				}
				if schedule == "" {
					schedule = "daily"
				}
			}

			// Step 11: Show confirmation
			if cmdutil.Interactive() && !args.yes {
				fmt.Printf("\nCreating discovery account:\n")
				fmt.Printf("  Name:            %s\n", accountName)
				fmt.Printf("  AWS Account:     %s\n", awsCfg.AccountID)
				fmt.Printf("  Regions:         %s\n", strings.Join(regions, ", "))
				fmt.Printf("  IAM Role:        %s\n", roleName)
				fmt.Printf("  ESC Environment: %s\n", escRef)
				fmt.Printf("  Schedule:        %s\n", schedule)
				fmt.Println()

				option := ui.PromptUser(
					"Proceed with account creation?",
					[]string{"yes", "no"},
					"yes",
					displayOpts.Color,
				)
				if option != "yes" {
					fmt.Println("Account creation cancelled.")
					return nil
				}
			}

			// Step 12: Create Insights account via API
			fmt.Printf("\nCreating discovery account %q...\n", accountName)

			providerConfig := map[string]interface{}{
				"regions": regions,
			}
			// Use the project/envName format for the environment reference
			envRef := fmt.Sprintf("%s/%s", escProject, escEnvName)

			req := apitype.CreateInsightsAccountRequest{
				Provider:       "aws",
				Environment:    envRef,
				ScanSchedule:   schedule,
				ProviderConfig: providerConfig,
			}

			err = cloudBackend.Client().CreateInsightsAccount(ctx, orgName, accountName, req)
			if err != nil {
				return fmt.Errorf("creating Insights account: %w", err)
			}

			// Step 13: Display success
			fmt.Printf("\nSuccessfully created account %q\n", accountName)
			fmt.Printf("\nNext steps:\n")
			fmt.Printf("  View account:      pulumi insights accounts show %s\n", accountName)
			fmt.Printf("  Trigger scan:      pulumi insights scans create %s\n", accountName)
			fmt.Printf("  View ESC env:      pulumi env open %s\n", escRef)

			return nil
		},
	}

	cmd.Flags().StringVar(&args.profile, "profile", "", "AWS profile name (prompts if not specified)")
	cmd.Flags().StringSliceVar(&args.regions, "regions", nil,
		"AWS regions to scan (comma-separated, default: us-east-1,us-west-2,eu-west-1)")
	cmd.Flags().BoolVar(&args.allRegions, "all-regions", false, "Scan all AWS regions")
	cmd.Flags().StringVar(&args.roleName, "role-name", "",
		"IAM role name (default: pulumi-insights-{accountID})")
	cmd.Flags().StringVar(&args.escEnv, "esc-env", "",
		"ESC environment reference (default: {org}/insights/aws-{accountID})")
	cmd.Flags().StringVar(&args.escProject, "esc-project", "",
		"ESC project name (default: insights)")
	cmd.Flags().StringVar(&args.schedule, "schedule", "",
		"Scan schedule: daily or none (default: daily)")
	cmd.Flags().BoolVarP(&args.yes, "yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().BoolVar(&args.skipOIDC, "skip-oidc", false,
		"Skip OIDC setup (assumes role already exists)")

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "name", Usage: "[optional account name]"},
		},
		Required: 0,
	})

	return cmd
}

// promptForAWSProfile prompts the user to select an AWS profile.
func promptForAWSProfile(displayOpts display.Options) (string, error) {
	profiles, err := listAWSProfiles()
	if err != nil || len(profiles) == 0 {
		// If we can't list profiles, just use default
		return "default", nil
	}

	// Ensure "default" is first if it exists
	hasDefault := false
	for _, p := range profiles {
		if p == "default" {
			hasDefault = true
			break
		}
	}
	if !hasDefault {
		profiles = append([]string{"default"}, profiles...)
	}

	if len(profiles) == 1 {
		fmt.Printf("Using AWS profile: %s\n", profiles[0])
		return profiles[0], nil
	}

	selected := ui.PromptUser(
		"Select AWS profile:",
		profiles,
		"default",
		displayOpts.Color,
	)
	if selected == "" {
		return "default", nil
	}
	return selected, nil
}

// promptForRegions prompts the user to select AWS regions.
func promptForRegions(displayOpts display.Options) []string {
	selected := ui.PromptUserMulti(
		"Select AWS regions to scan:",
		allAWSRegions,
		defaultRegions,
		displayOpts.Color,
	)
	if len(selected) == 0 {
		return defaultRegions
	}
	return selected
}

// promptForSchedule prompts the user to select a scan schedule.
func promptForSchedule(displayOpts display.Options) string {
	selected := ui.PromptUser(
		"Select scan schedule:",
		[]string{"daily", "none"},
		"daily",
		displayOpts.Color,
	)
	if selected == "" {
		return "daily"
	}
	return selected
}
