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
	"slices"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cloudsetup/azuresetup"
	cloudsetup "github.com/pulumi/pulumi/pkg/v3/cloudsetup/common"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// azureLoginPath is the property path under `values` where the login block is written,
// matching the default of `env provider azure-login`.
const azureLoginPath = "azure.login"

// The --policy presets, mapping each official role name to the built-in role definition assigned
// per subscription. Any other --policy value is used as a role definition ID as-is, which is how a
// custom role is selected; Azure assigns roles by definition ID, not by name.
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
const armScope = "https://management.azure.com/.default"

// newAzureDeviceCodeCredential returns a credential that signs the user in through the browser
// using the device authorization flow, printing the code and opening the verification URL.
//
// The default public client ID already has the delegated ARM and Microsoft Graph permissions
// needed, so unlike the Pulumi service there is no app registration to pre-provision.
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
	cred, err := azidentity.NewDefaultAzureCredential(&azidentity.DefaultAzureCredentialOptions{TenantID: tenantID})
	if err != nil {
		return nil, err
	}
	// DefaultAzureCredential is lazy; a token request is what proves it actually resolves.
	if _, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{armScope}}); err != nil {
		return nil, err
	}
	return cred, nil
}

// resolveAzureCredential decides how to authenticate, offering a choice between existing
// credentials and a browser sign-in. Both span the whole tenant, so unlike AWS the options do
// not differ in how many accounts they can configure.
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

// selectAzureSubscriptions resolves which subscriptions to configure. Unlike AWS accounts, a
// subscription carries no role to choose: the role is the same --policy for all of them.
func selectAzureSubscriptions(
	esc *escCommand, subscriptions []cloudsetup.CloudAccount, subscriptionIDs []string, yes bool,
) ([]cloudsetup.CloudAccount, error) {
	if len(subscriptions) == 0 {
		return nil, errors.New("no Azure subscriptions are accessible with these credentials")
	}

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

	if len(subscriptions) == 1 {
		return subscriptions, nil
	}
	if yes {
		return nil, errors.New("multiple subscriptions are accessible; pass --subscription to choose without prompting")
	}

	labels := make([]string, len(subscriptions))
	for i, s := range subscriptions {
		labels[i] = fmt.Sprintf("%s (%s)", s.Name, s.ID)
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

// azureAppClientID returns the client ID of the app registration created during setup. Its
// resource ID is the client ID (see azuresetup's appRegistrationResource).
func azureAppClientID(result *cloudsetup.CloudSetupResult) (string, bool) {
	for _, res := range result.Resources {
		if res.Type == azuresetup.ResourceTypeAzureApplication && res.ID != "" {
			return res.ID, true
		}
	}
	return "", false
}

// azureEnvTarget is an environment to write, pinning the subscription it was created for.
type azureEnvTarget struct {
	ref            environmentRef
	subscriptionID string
}

// createAzureEnvironments writes an `fn::open::azure-login` OIDC block per target, reusing the
// same helpers as `env provider azure-login oidc`.
//
// Each environment's path is embedded in the subject of the federated credential created for it
// during setup, so every ref here must be the same one threaded into SetupOIDCInfrastructure.
func createAzureEnvironments(
	ctx context.Context, setup *setupCommand, result *cloudsetup.CloudSetupResult,
	tenantID string, targets []azureEnvTarget,
) error {
	clientID, ok := azureAppClientID(result)
	if !ok {
		return errors.New("app registration client ID missing from the setup result")
	}

	path, err := resource.ParsePropertyPath(azureLoginPath)
	if err != nil {
		return fmt.Errorf("invalid provider path %q: %w", azureLoginPath, err)
	}

	var failed int
	for _, t := range targets {
		fmt.Fprintf(setup.esc().stdout, "\nConfiguring environment %s for subscription %s (tenant %s):\n",
			t.ref.String(), t.subscriptionID, tenantID)

		node := buildAzureLoginOIDCNode(clientID, tenantID, t.subscriptionID, nil)
		envVars := azureLoginOIDCEnvVars(propertyPathRef(path), t.subscriptionID != "")
		if err := ensureProviderEnv(ctx, setup.env, t.ref, true); err != nil {
			fmt.Fprintf(setup.esc().stderr, "  %v\n", err)
			failed++
			continue
		}
		if err := applyProviderUpdate(ctx, setup.env, t.ref, "", path, node, envVars); err != nil {
			fmt.Fprintf(setup.esc().stderr, "  %v\n", err)
			failed++
			continue
		}
	}

	if failed == len(targets) {
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
		appName         string

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

			roleID, err := setup.resolvePolicy(policy, azurePolicyChoices, yes)
			if err != nil {
				return err
			}

			cred, err := resolveAzureCredential(ctx, esc, browserAuth, tenantID, yes)
			if err != nil {
				return err
			}

			// Enumerate subscriptions with a client that needs no subscription IDs, then build
			// the setup client for the chosen ones (its role clients are keyed by subscription).
			subscriptions, err := azuresetup.NewClientFromCredential(cred, oidcIssuer, "", nil).ListAccounts(ctx)
			if err != nil {
				return fmt.Errorf("listing Azure subscriptions: %w", err)
			}
			selected, err := selectAzureSubscriptions(esc, subscriptions, subscriptionIDs, yes)
			if err != nil {
				return err
			}

			selectedIDs := make([]string, len(selected))
			for i, sub := range selected {
				selectedIDs[i] = sub.ID
			}

			// The tenant is needed before setup: the app registration is tenant-scoped, and each
			// federated credential's subject embeds its environment's path.
			tenant := tenantID
			if tenant == "" {
				tenant, err = azuresetup.NewClientFromCredential(cred, oidcIssuer, "", nil).Tenant(ctx, selectedIDs)
				if err != nil {
					return err
				}
			}

			// One environment per subscription. Each environment's path is baked into its
			// federated credential's subject, so the environments are decided up front and
			// threaded into SetupOIDCInfrastructure rather than chosen after setup.
			envRefFor := func(sub cloudsetup.CloudAccount) environmentRef {
				return setup.env.parseRef(
					fmt.Sprintf("%s/%s/%s", org, projectName, sanitizeEnvName(sub.Name, sub.ID)))
			}

			envInfos := make([]cloudsetup.AzureEnvironmentInfo, len(selected))
			targets := make([]azureEnvTarget, len(selected))
			for i, sub := range selected {
				ref := envRefFor(sub)
				envInfos[i] = cloudsetup.AzureEnvironmentInfo{
					SubscriptionID:  sub.ID,
					RoleID:          roleID,
					ProjectName:     ref.projectName,
					EnvironmentName: ref.envName,
				}
				targets[i] = azureEnvTarget{ref: ref, subscriptionID: sub.ID}
			}

			fmt.Fprintf(esc.stdout, "\nAbout to configure OIDC for organization %s (tenant %s):\n", org, tenant)
			for _, sub := range selected {
				ref := envRefFor(sub)
				fmt.Fprintf(esc.stdout, "  subscription %s (%s): assign %s, environment %s\n",
					sub.Name, sub.ID, policy, ref.String())
			}
			if !yes {
				if ui.PromptUser("Proceed?", []string{"yes", "no"}, "no", esc.colors) != "yes" {
					return errors.New("cancelled")
				}
			}

			setup.printHeading("Setting up Infrastructure")
			client := azuresetup.NewClientFromCredential(cred, oidcIssuer, appName, selectedIDs)
			// Empty existing IDs: a single CLI invocation does the whole setup in one call, so
			// there is no prior app registration to thread through.
			result, err := client.SetupOIDCInfrastructure(ctx, org, envInfos, "", "")
			if result != nil {
				renderSetupResults(esc.stdout, []accountSetupResult{{
					account: cloudsetup.CloudAccount{Name: "Azure tenant"},
					result:  result,
					err:     err,
				}}, azureResourceNames)
			}
			if err != nil {
				return err
			}
			if !result.Success {
				return errors.New("failed to configure Azure OIDC")
			}

			setup.printHeading("Setting up Environment(s)")
			return createAzureEnvironments(ctx, setup, result, tenant, targets)
		},
	}

	cmd.Flags().StringVar(&policy, "policy", "",
		"the role assigned per subscription: Contributor (required for Deployments), Reader "+
			"(required for Insights), or any other role definition ID; prompted for when omitted")
	cmd.Flags().StringArrayVar(&subscriptionIDs, "subscription", nil,
		"an Azure subscription to set up (repeatable; prompted for when omitted)")
	cmd.Flags().StringVar(&tenantID, "tenant", "", "the Azure tenant to authenticate in (defaults to your home tenant)")
	cmd.Flags().StringVar(&appName, "app-name", "",
		"the app registration display name to create or reuse; a new name gets its own "+
			"20 federated-credential budget (defaults to pulumi-esc-oidc-app)")
	cmd.Flags().BoolVar(&browserAuth, "browser", false, "force browser sign-in instead of using existing credentials")
	cmd.Flags().StringVar(&orgName, "org", "", "the Pulumi organization to configure OIDC for")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip all confirmation prompts")

	cmd.Flags().StringVar(&projectName, "project", "azure-login",
		"the ESC project that per-subscription environments are created in")

	return cmd
}
