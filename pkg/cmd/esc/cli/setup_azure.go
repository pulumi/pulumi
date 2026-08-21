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
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cloudsetup/azuresetup"
	cloudsetup "github.com/pulumi/pulumi/pkg/v3/cloudsetup/common"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// azureLoginPath is the property path under `values` where the login block is written,
// matching the default of `env provider azure-login`.
const azureLoginPath = "azure.login"

var azurePolicyChoices = []policyChoice{
	{
		// Contributor - https://learn.microsoft.com/azure/role-based-access-control/built-in-roles/privileged#contributor
		name:  "Contributor",
		id:    "b24988ac-6180-42a0-ab88-20f7382dd24c",
		alias: policyAliasAdmin,
		desc:  policyAdminAccess,
	},
	{
		// Reader - https://learn.microsoft.com/azure/role-based-access-control/built-in-roles/general#reader
		name:  "Reader",
		id:    "acdd72a7-3385-48ef-bd42-f606fba81ae7",
		alias: policyAliasReadonly,
		desc:  policyReadonlyAccess,
	},
}

var azureResourceNames = map[string]string{
	azuresetup.ResourceTypeAzureApplication:         "App Registration",
	azuresetup.ResourceTypeAzureFederatedCredential: "Federated Credential",
	azuresetup.ResourceTypeAzureServicePrincipal:    "Service Principal",
	azuresetup.ResourceTypeAzureRoleAssignment:      "Role Assignment",
}

// armScope is the ARM resource scope used to probe whether existing credentials work.
func azureOIDCAppDisplayName(orgID, escEnvironmentName string) string {
	return fmt.Sprintf("pulumi-esc-oidc-app-%s-%s", orgIDPrefix(orgID), escEnvironmentName)
}

const armScope = "https://management.azure.com/.default"

// newAzureDeviceCodeCredential returns a credential that signs the user in through the browser
// using the device authorization flow, printing the code and opening the verification URL.
func newAzureDeviceCodeCredential(esc *escCommand, tenantID string) (azcore.TokenCredential, error) {
	return azidentity.NewDeviceCodeCredential(&azidentity.DeviceCodeCredentialOptions{
		TenantID: tenantID,
		UserPrompt: func(_ context.Context, m azidentity.DeviceCodeMessage) error {
			fmt.Fprintf(esc.stdout, "\nConfirm the code %s to authorize this device:\n  %s\n\n",
				m.UserCode, m.VerificationURL)
			if err := browser.OpenURL(m.VerificationURL); err != nil {
				fmt.Fprintf(esc.stderr, "Could not open a browser automatically; visit the URL above.\n")
			}
			fmt.Fprintf(esc.stdout, "Waiting for authorization...\n")
			return nil
		},
	})
}

// tryExistingAzureCredential returns a working ambient credential, or an error if none is
// configured. Success means az login / env vars / managed identity produced a usable ARM token.
func tryExistingAzureCredential(ctx context.Context, tenantID string) (azcore.TokenCredential, error) {
	cred, err := azidentity.NewDefaultAzureCredential(&azidentity.DefaultAzureCredentialOptions{
		TenantID: tenantID,
		// The chosen tenant need not be the one the local session signed into, and azidentity
		// refuses to issue tokens for any other unless they are allowed here.
		AdditionallyAllowedTenants: []string{"*"},
	})
	if err != nil {
		return nil, err
	}
	// Perform a token request to confirm the credentials are valid
	if _, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{armScope}}); err != nil {
		return nil, err
	}
	return cred, nil
}

var azureTenantIDRE = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

const azureTenantManualOption = "[Enter manually]"

// resolveAzureTenant returns the tenant to configure, prompting when --tenant was omitted.
func resolveAzureTenant(ctx context.Context, esc *escCommand, tenantID string, yes bool) (string, error) {
	if tenantID != "" {
		if !azureTenantIDRE.MatchString(tenantID) {
			return "", fmt.Errorf("--tenant %q is not an Azure tenant ID", tenantID)
		}
		return tenantID, nil
	}
	if yes {
		return "", errors.New("--tenant must be set when using --yes")
	}

	tenants := azureTenantChoices(ctx, esc)
	if len(tenants) == 0 {
		return promptAzureTenantID(esc)
	}

	labels := append(accountLabels(tenants), azureTenantManualOption)
	choice := ui.PromptUser("Which Azure tenant should be configured?", labels, labels[0], esc.colors)
	if choice == "" || choice == azureTenantManualOption {
		return promptAzureTenantID(esc)
	}
	return tenants[slices.Index(labels, choice)].ID, nil
}

// azureTenantChoices suggests the tenants any existing credentials can reach.
func azureTenantChoices(ctx context.Context, esc *escCommand) []cloudsetup.CloudAccount {
	cred, err := tryExistingAzureCredential(ctx, "")
	if err != nil {
		return nil
	}
	tenants, err := azuresetup.NewClientFromCredential(cred, "", "", nil).ListTenants(ctx)
	if err != nil {
		fmt.Fprintf(esc.stderr, "Could not list your Azure tenants: %v\n", err)
		return nil
	}
	return tenants
}

func promptAzureTenantID(esc *escCommand) (string, error) {
	return ui.PromptForValue(false, "Azure tenant ID", "", false,
		func(value string) error {
			if !azureTenantIDRE.MatchString(value) {
				return errors.New("not an Azure tenant ID; find it with `az account show --query tenantId`")
			}
			return nil
		},
		display.Options{Stdout: esc.stdout, Stdin: esc.stdin, Color: esc.colors})
}

// resolveAzureCredential decides how to authenticate, either using existing credentials
// or a browser sign-in.
func resolveAzureCredential(
	ctx context.Context, esc *escCommand, forceBrowser bool, tenantID string, yes bool,
) (azcore.TokenCredential, error) {
	if forceBrowser {
		return newAzureDeviceCodeCredential(esc, tenantID)
	}

	existing, existingErr := tryExistingAzureCredential(ctx, tenantID)
	if existingErr != nil {
		fmt.Fprintf(esc.stdout, "No existing Azure credentials found; signing in with your browser.\n")
		return newAzureDeviceCodeCredential(esc, tenantID)
	}

	if yes {
		return existing, nil
	}

	const existingLabel = "Use existing Azure credentials (az login / environment variables)"
	const browserLabel = "Sign in with your browser"
	switch ui.PromptUser("How would you like to authenticate to Azure?",
		[]string{existingLabel, browserLabel}, existingLabel, esc.colors) {
	case existingLabel:
		return existing, nil
	case browserLabel:
		return newAzureDeviceCodeCredential(esc, tenantID)
	default:
		return nil, errors.New("cancelled")
	}
}

// selectAzureSubscriptions resolves which subscriptions to configure.
func selectAzureSubscriptions(
	esc *escCommand, subscriptions []cloudsetup.CloudAccount, subscriptionIDs []string, yes bool,
) ([]cloudsetup.CloudAccount, error) {
	if len(subscriptions) == 0 {
		return nil, errors.New("no Azure subscriptions are accessible with these credentials")
	}

	labels := accountLabels(subscriptions)

	if len(subscriptionIDs) > 0 {
		var chosen []cloudsetup.CloudAccount
		for _, id := range subscriptionIDs {
			i := slices.IndexFunc(subscriptions, func(s cloudsetup.CloudAccount) bool { return s.ID == id })
			if i < 0 {
				return nil, fmt.Errorf("subscription %s is not accessible with these credentials", id)
			}
			chosen = append(chosen, subscriptions[i])
		}
		return chosen, nil
	}

	if yes {
		if len(subscriptions) > 1 {
			return nil, errors.New(
				"multiple subscriptions are accessible; pass --subscription to choose without prompting")
		}
		return subscriptions, nil
	}

	picked := ui.PromptUserMulti("Which subscriptions should be set up?", labels, nil, esc.colors)
	if len(picked) == 0 {
		return nil, errors.New("no subscriptions selected")
	}
	var chosen []cloudsetup.CloudAccount
	for _, label := range picked {
		chosen = append(chosen, subscriptions[slices.Index(labels, label)])
	}
	return chosen, nil
}

// azureAppClientID returns the client ID of the app registration created during setup.
func azureAppClientID(result *cloudsetup.CloudSetupResult) (string, bool) {
	for _, res := range result.Resources {
		if res.Type == azuresetup.ResourceTypeAzureApplication && res.ID != "" {
			return res.ID, true
		}
	}
	return "", false
}

// createAzureEnvironments adds the azure-login provider into each environment. Each subscription
// has its own app registration, so the client ID comes from that subscription's setup result.
func createAzureEnvironments(
	ctx context.Context, setup *setupCommand, org, projectName, tenantID string, results []accountSetupResult,
) error {
	path, err := resource.ParsePropertyPath(azureLoginPath)
	if err != nil {
		return fmt.Errorf("invalid provider path %q: %w", azureLoginPath, err)
	}

	var attempted, failed int
	for _, r := range results {
		if !r.succeeded() {
			continue
		}
		attempted++

		clientID, ok := azureAppClientID(r.result)
		if !ok {
			fmt.Fprintf(setup.esc().stderr, "%s: app registration client ID missing from the setup result\n", r.label())
			failed++
			continue
		}

		ref := setup.env.parseRef(org + "/" + escEnvName(projectName, r.account))
		fmt.Fprintf(setup.esc().stdout, "\nConfiguring environment %s for subscription %s (tenant %s):\n",
			ref.String(), r.account.ID, tenantID)

		node := buildAzureLoginOIDCNode(clientID, tenantID, r.account.ID, oidcSubjectAttributes)
		envVars := azureLoginOIDCEnvVars(propertyPathRef(path), r.account.ID != "")
		if err := ensureProviderEnv(ctx, setup.env, ref, true); err != nil {
			fmt.Fprintf(setup.esc().stderr, "  %v\n", err)
			failed++
			continue
		}
		if err := applyProviderUpdate(ctx, setup.env, ref, "", path, node, envVars); err != nil {
			fmt.Fprintf(setup.esc().stderr, "  %v\n", err)
			failed++
			continue
		}
	}

	if attempted > 0 && failed == attempted {
		return errors.New("failed to create any environment")
	}
	return nil
}

func newSetupAzureCmd(setup *setupCommand) *cobra.Command {
	var (
		subscriptionIDs []string
		policy          string
		orgName         string
		tenantID        string
		browserAuth     bool
		yes             bool

		projectName string
	)

	cmd := &cobra.Command{
		Use:   "azure",
		Short: "Set up Azure OIDC integration for Pulumi ESC",
		Long: "[EXPERIMENTAL] Set up Azure OIDC integration for Pulumi ESC\n" +
			"\n" +
			"Creates, in your Azure tenant:\n" +
			"  - an app registration trusting Pulumi Cloud as an OIDC identity provider\n" +
			"  - a federated identity credential and a service principal\n" +
			"  - a role assignment on each selected subscription\n" +
			"\n" +
			"You are asked how to authenticate: with the Azure credentials you already have (from\n" +
			"`az login` or environment variables), or by signing in through your browser. Both span\n" +
			"the whole tenant.\n" +
			"\n" +
			"Each selected subscription gets its own environment, pinning that subscription.\n" +
			"\n" +
			"Examples:\n" +
			"  pulumi env setup azure --policy Contributor\n" +
			"  pulumi env setup azure --policy Reader --subscription <sub-id> --yes\n",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			esc := setup.esc()

			if err := esc.getCachedClient(ctx); err != nil {
				return err
			}

			oidcIssuer, err := setup.oidcIssuer()
			if err != nil {
				return err
			}
			org, err := setup.org(orgName)
			if err != nil {
				return err
			}

			tenant, err := resolveAzureTenant(ctx, esc, tenantID, yes)
			if err != nil {
				return err
			}

			roleID, err := setup.resolvePolicy(policy, azurePolicyChoices, yes)
			if err != nil {
				return err
			}
			// Azure role IDs are GUIDs, so name the preset when the ID is one of ours.
			roleName := roleID
			if i := slices.IndexFunc(azurePolicyChoices, func(c policyChoice) bool { return c.id == roleID }); i >= 0 {
				roleName = azurePolicyChoices[i].name
			}

			cred, err := resolveAzureCredential(ctx, esc, browserAuth, tenant, yes)
			if err != nil {
				return err
			}

			// Enumerate subscriptions with a client that needs neither subscription IDs nor an
			// app registration, then build a setup client per chosen subscription below.
			subscriptions, err := azuresetup.NewClientFromCredential(cred, oidcIssuer, "", nil).ListAccounts(ctx)
			if err != nil {
				return fmt.Errorf("listing Azure subscriptions: %w", err)
			}
			selected, err := selectAzureSubscriptions(esc, subscriptions, subscriptionIDs, yes)
			if err != nil {
				return err
			}

			orgID, err := setup.orgID(ctx, org)
			if err != nil {
				return err
			}

			fmt.Fprintf(esc.stdout, "\nAbout to configure OIDC for organization %s (tenant %s):\n", org, tenant)
			for _, sub := range selected {
				envName := escEnvName(projectName, sub)
				printSetupTarget(esc, fmt.Sprintf("subscription %s (%s):", sub.Name, sub.ID))
				fmt.Fprintf(esc.stdout, "    assign %s\n", roleName)
				fmt.Fprintf(esc.stdout, "    create app %s\n", azureOIDCAppDisplayName(orgID, envName))
				fmt.Fprintf(esc.stdout, "    create ESC environment %s/%s\n", org, envName)
			}
			fmt.Fprintln(esc.stdout)

			if !yes {
				if ui.PromptUser("Proceed?", []string{"yes", "no"}, "no", esc.colors) != "yes" {
					return errors.New("cancelled")
				}
			}

			setup.printHeading("Setting up Infrastructure")
			results := make([]accountSetupResult, 0, len(selected))
			for _, sub := range selected {
				fmt.Fprintf(esc.stdout, "\nSetting up subscription %s...\n", sub.ID)

				envName := escEnvName(projectName, sub)
				ref := setup.env.parseRef(org + "/" + envName)
				envInfos := []cloudsetup.AzureEnvironmentInfo{{
					SubscriptionID:  sub.ID,
					RoleID:          roleID,
					ProjectName:     ref.projectName,
					EnvironmentName: ref.envName,
				}}

				appName := azureOIDCAppDisplayName(orgID, envName)
				client := azuresetup.NewClientFromCredential(cred, oidcIssuer, appName, []string{sub.ID})
				result, err := client.SetupOIDCInfrastructure(ctx, org, envInfos, "", "")
				results = append(results, accountSetupResult{account: sub, result: result, err: err})
			}
			renderSetupResults(esc.stdout, results, azureResourceNames)

			if !slices.ContainsFunc(results, accountSetupResult.succeeded) {
				return errors.New("failed to configure Azure OIDC")
			}

			setup.printHeading("Setting up Environment(s)")
			return createAzureEnvironments(ctx, setup, org, projectName, tenant, results)
		},
	}

	cmd.Flags().StringVar(&policy, "policy", "",
		"the role assigned per subscription: Contributor (required for Deployments), Reader "+
			"(required for Insights), or any other role definition ID; prompted for when omitted")
	cmd.Flags().StringArrayVar(&subscriptionIDs, "subscription", nil,
		"an Azure subscription to set up (repeatable; prompted for when omitted)")
	cmd.Flags().StringVar(&tenantID, "tenant", "",
		"the Azure tenant to configure, which only its own subscriptions are visible through "+
			"(prompted for when omitted)")
	cmd.Flags().BoolVar(&browserAuth, "browser", false, "force browser sign-in instead of using existing credentials")
	cmd.Flags().StringVar(&orgName, "org", "", "the Pulumi organization to configure OIDC for")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip all confirmation prompts")

	cmd.Flags().StringVar(&projectName, "project", "azure-login",
		"the ESC project that per-subscription environments are created in")

	return cmd
}
