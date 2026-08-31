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

// Package azuresetup provides Azure-specific cloud setup functionality
package azuresetup

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"

	"github.com/google/uuid"
	cloudsetup "github.com/pulumi/pulumi/pkg/v3/cloudsetup/common"
)

const (
	ResourceTypeAzureApplication         = "azure:application"
	ResourceTypeAzureFederatedCredential = "azure:federated-credential"
	ResourceTypeAzureServicePrincipal    = "azure:service-principal"
	ResourceTypeAzureRoleAssignment      = "azure:role-assignment"

	// ResourcePropertyObjectID is the result-resource property carrying the Graph object ID of
	// the app registration (its resource ID is the client ID). Batched setup callers read it to
	// thread ExistingAppIdentity into follow-up calls.
	ResourcePropertyObjectID = "objectId"

	// federatedCredentialNamePrefix names every federated identity credential this setup flow
	// creates on the shared app registration; the FIC-limit guidance below tells users which
	// entries are safe to prune, so keep the two in sync.
	//nolint:gosec // G101: constant name contains "Credential", not a hardcoded credential value
	federatedCredentialNamePrefix = "pulumi-esc-oidc-credential"

	// federatedCredentialLimitMessage is the distinctive fragment of the Graph API error returned
	// when an app registration already holds Azure's maximum of 20 federated identity credentials.
	// Graph reports it under a generic bad-request code, so the message text is the only signal.
	//nolint:gosec // G101: constant name contains "Credential", not a hardcoded credential value
	federatedCredentialLimitMessage = "size of the object has exceeded its limit"
)

func isFederatedCredentialLimitError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), federatedCredentialLimitMessage)
}

type RoleAssignmentsClient interface {
	Create(
		ctx context.Context,
		scope string,
		roleAssignmentName string,
		parameters armauthorization.RoleAssignmentCreateParameters,
		options *armauthorization.RoleAssignmentsClientCreateOptions,
	) (armauthorization.RoleAssignmentsClientCreateResponse, error)
}

type SubscriptionsClient interface {
	NewListPager(options *armsubscriptions.ClientListOptions) *runtime.Pager[armsubscriptions.ClientListResponse]
}

type TenantsClient interface {
	NewListPager(
		options *armsubscriptions.TenantsClientListOptions,
	) *runtime.Pager[armsubscriptions.TenantsClientListResponse]
}

// StaticTokenCredential implements azcore.TokenCredential for using access tokens
type StaticTokenCredential struct {
	token string
}

func (c *StaticTokenCredential) GetToken(
	ctx context.Context, opts policy.TokenRequestOptions,
) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: c.token}, nil
}

type Client interface {
	// SetupOIDCInfrastructure provisions the app registration, federated credentials, service
	// principal, and per-subscription role assignments for the given environments.
	//
	// existingAppObjectID and existingServicePrincipalID carry the Graph object IDs of the app
	// registration and service principal created by a prior call. Batched setup requests pass
	// these so follow-up calls resolve the app by ID (a strongly consistent Graph read) instead
	// of searching by display name — Graph $filter queries are eventually consistent and can
	// miss a just-created app, which would split one org's setup across two app registrations.
	// Empty strings mean find-or-create.
	SetupOIDCInfrastructure(
		ctx context.Context, orgName string, envInfos []cloudsetup.AzureEnvironmentInfo,
		existingAppObjectID, existingServicePrincipalID string,
	) (*cloudsetup.CloudSetupResult, error)
	ListAccounts(ctx context.Context) ([]cloudsetup.CloudAccount, error)
}

// LocalClient is a Client that can also enumerate the tenants its credential can reach.
type LocalClient interface {
	Client
	// ListTenants returns the Azure tenants the signed-in user can reach.
	ListTenants(ctx context.Context) ([]cloudsetup.CloudAccount, error)
}

type client struct {
	oidcIssuer string
	// appDisplayName is the display name used to find-or-create the app registration.
	appDisplayName string

	graphClient            GraphClient
	subscriptionsClient    SubscriptionsClient
	tenantsClient          TenantsClient
	roleAssignmentsClients map[string]RoleAssignmentsClient

	maxRetryAttempts int
}

// NewClient builds a Client from pre-fetched ARM and Microsoft Graph access tokens.
// appDisplayName is the app registration to find-or-create.
func NewClient(
	armAccessToken, graphAccessToken, oidcIssuer, appDisplayName string, subscriptionIDs []string,
) Client {
	return newClient(
		&StaticTokenCredential{token: armAccessToken},
		&StaticTokenCredential{token: graphAccessToken},
		oidcIssuer, appDisplayName, subscriptionIDs)
}

// NewClientFromCredential builds a Client from a local azcore.TokenCredential (e.g. `az login`
// or a browser sign-in) that serves both the ARM and Microsoft Graph scopes. appDisplayName is
// the app registration to find-or-create.
func NewClientFromCredential(
	cred azcore.TokenCredential, oidcIssuer, appDisplayName string, subscriptionIDs []string,
) LocalClient {
	return newClient(cred, cred, oidcIssuer, appDisplayName, subscriptionIDs)
}

func newClient(
	armCred, graphCred azcore.TokenCredential, oidcIssuer, appDisplayName string, subscriptionIDs []string,
) *client {
	client := &client{
		oidcIssuer:             oidcIssuer,
		appDisplayName:         appDisplayName,
		roleAssignmentsClients: make(map[string]RoleAssignmentsClient),
		maxRetryAttempts:       6,
	}

	client.graphClient = NewGraphClient(graphCred)

	subClient, err := armsubscriptions.NewClient(armCred, nil)
	if err == nil {
		client.subscriptionsClient = subClient
	}

	tenantsClient, err := armsubscriptions.NewTenantsClient(armCred, nil)
	if err == nil {
		client.tenantsClient = tenantsClient
	}

	for _, id := range subscriptionIDs {
		roleAssignmentsClient, err := armauthorization.NewRoleAssignmentsClient(id, armCred, nil)
		if err == nil {
			client.roleAssignmentsClients[id] = roleAssignmentsClient
		}
	}

	return client
}

func (c *client) SetupOIDCInfrastructure(
	ctx context.Context, orgName string, envInfos []cloudsetup.AzureEnvironmentInfo,
	existingAppObjectID, existingServicePrincipalID string,
) (*cloudsetup.CloudSetupResult, error) {
	if c.graphClient == nil {
		return nil, cloudsetup.NewSetupError(
			cloudsetup.ErrorCodeInvalidCredentials, "failed to create graph client", errors.New("unknown error"))
	}

	result := &cloudsetup.CloudSetupResult{
		Success:   false,
		Resources: []cloudsetup.CloudSetupResource{},
	}

	// Resolve the app registration: by object ID when a prior call already created it,
	// otherwise find by display name or create.
	var appResource cloudsetup.CloudSetupResource
	var appObjectID, appClientID string
	var err error
	if existingAppObjectID != "" {
		appResource, appObjectID, appClientID, err = c.getAppRegistration(ctx, existingAppObjectID)
	} else {
		appResource, appObjectID, appClientID, err = c.findOrCreateAppRegistration(ctx)
	}
	if err != nil {
		return cloudsetup.WrapSetupError(result, ResourceTypeAzureApplication, err)
	}
	result.Resources = append(result.Resources, appResource)

	fedCredResources, err := c.findOrCreateFederatedIdentityCredentials(ctx, appObjectID, orgName, envInfos)
	if err != nil {
		return cloudsetup.WrapSetupError(result, ResourceTypeAzureFederatedCredential, err)
	}
	result.Resources = append(result.Resources, fedCredResources...)

	// Resolve the service principal: a provided ID is verified to belong to the resolved app
	// registration before any role is assigned to it, otherwise find or create.
	var servicePrincipalResource cloudsetup.CloudSetupResource
	var principalID string
	if existingServicePrincipalID != "" {
		principalAppID, err := c.graphClient.GetServicePrincipalByID(ctx, existingServicePrincipalID)
		if err != nil {
			return cloudsetup.WrapSetupError(result, ResourceTypeAzureServicePrincipal, err)
		}
		if principalAppID != appClientID {
			return cloudsetup.WrapSetupError(result, ResourceTypeAzureServicePrincipal, fmt.Errorf(
				"service principal %s belongs to application %s, not %s",
				existingServicePrincipalID, principalAppID, appClientID,
			))
		}
		principalID = existingServicePrincipalID
		servicePrincipalResource = cloudsetup.CloudSetupResource{
			Type:   ResourceTypeAzureServicePrincipal,
			ID:     principalID,
			Name:   c.appDisplayName,
			Status: cloudsetup.ResourceStatusExisting,
		}
	} else {
		servicePrincipalResource, principalID, err = c.findOrCreateServicePrincipal(ctx, appClientID)
		if err != nil {
			return cloudsetup.WrapSetupError(result, ResourceTypeAzureServicePrincipal, err)
		}
	}
	result.Resources = append(result.Resources, servicePrincipalResource)

	// Assign roles to service principal
	maxAttempts := c.maxRetryAttempts
	var roleErr error
	for _, envInfo := range envInfos {
		subscriptionID := envInfo.SubscriptionID
		resource, err := c.assignPrincipalRole(ctx, subscriptionID, principalID, envInfo.RoleID, maxAttempts)
		if err != nil {
			result.Resources = append(result.Resources, cloudsetup.CloudSetupResource{
				Type:   ResourceTypeAzureRoleAssignment,
				Status: cloudsetup.ResourceStatusFailed,
				Error:  err.Error(),
				Properties: map[string]string{
					"subscriptionID": subscriptionID,
				},
			})
			roleErr = &cloudsetup.SetupError{
				Code:    cloudsetup.ErrorCodeSetupFailed,
				Message: "failed to create: " + ResourceTypeAzureRoleAssignment,
				Cause:   err,
			}
		} else {
			result.Resources = append(result.Resources, resource)
		}

		// After the first iteration, we shouldn't be running into issues with propagation delays
		maxAttempts = 1
	}

	if roleErr == nil {
		result.Success = true
	}

	return result, roleErr
}

func appRegistrationResource(appObjectID, appClientID, displayName, status string) cloudsetup.CloudSetupResource {
	return cloudsetup.CloudSetupResource{
		Type:   ResourceTypeAzureApplication,
		ID:     appClientID,
		Name:   displayName,
		Status: status,
		Properties: map[string]string{
			ResourcePropertyObjectID: appObjectID,
		},
	}
}

// getAppRegistration resolves an app registration by its Graph object ID — a direct,
// strongly consistent read, unlike the display-name search in findOrCreateAppRegistration.
func (c *client) getAppRegistration(
	ctx context.Context, appObjectID string,
) (cloudsetup.CloudSetupResource, string, string, error) {
	appClientID, displayName, err := c.graphClient.GetAppRegistrationByObjectID(ctx, appObjectID)
	if err != nil {
		return cloudsetup.CloudSetupResource{}, "", "", err
	}
	if displayName != c.appDisplayName {
		return cloudsetup.CloudSetupResource{}, "", "", fmt.Errorf(
			"application %s has display name %q, expected the %q app registration created by onboarding",
			appObjectID, displayName, c.appDisplayName,
		)
	}
	return appRegistrationResource(appObjectID, appClientID, displayName, cloudsetup.ResourceStatusExisting),
		appObjectID, appClientID, nil
}

func (c *client) findOrCreateAppRegistration(
	ctx context.Context,
) (cloudsetup.CloudSetupResource, string, string, error) {
	displayName := c.appDisplayName
	// https://learn.microsoft.com/en-us/entra/identity-platform/supported-accounts-validation
	signInAudience := "AzureADMyOrg"

	// Check if app registration with same name and signin audience already exists
	appObjectID, appClientID, found, err := c.graphClient.FindAppRegistrationByName(ctx, displayName, signInAudience)
	if err != nil {
		return cloudsetup.CloudSetupResource{}, "", "", err
	}
	if found {
		return appRegistrationResource(appObjectID, appClientID, displayName, cloudsetup.ResourceStatusExisting),
			appObjectID, appClientID, nil
	}

	// Create new app registration
	appObjectID, appClientID, err = c.graphClient.CreateAppRegistration(ctx, displayName, signInAudience)
	if err != nil {
		return cloudsetup.CloudSetupResource{}, "", "", err
	}

	return appRegistrationResource(appObjectID, appClientID, displayName, cloudsetup.ResourceStatusCreated),
		appObjectID, appClientID, nil
}

// findOrCreateFederatedIdentityCredentials creates one credential per environment, matching the
// subject the generated `subjectAttributes: ["currentEnvironment.name"]` block presents. A subject
// naming the environment rather than the organization keeps an environment that copies the
// generated azure-login block from opening the credential.
func (c *client) findOrCreateFederatedIdentityCredentials(
	ctx context.Context, appObjectID string, orgName string, envInfos []cloudsetup.AzureEnvironmentInfo,
) ([]cloudsetup.CloudSetupResource, error) {
	type credential struct{ subject, audience string }
	credentials := make([]credential, 0, len(envInfos))
	for _, envInfo := range envInfos {
		escEnvName := fmt.Sprintf("%s/%s", envInfo.ProjectName, envInfo.EnvironmentName)
		audience, subjectEnvName := cloudsetup.OIDCClaims("azure", orgName, escEnvName)
		cred := credential{
			subject: fmt.Sprintf(
				"pulumi:environments:pulumi.organization.login:%s:currentEnvironment.name:%s",
				orgName, subjectEnvName,
			),
			audience: audience,
		}
		if !slices.Contains(credentials, cred) {
			credentials = append(credentials, cred)
		}
	}

	resources := make([]cloudsetup.CloudSetupResource, 0, len(credentials))
	for _, cred := range credentials {
		resource, err := c.findOrCreateFederatedIdentityCredential(ctx, appObjectID, cred.subject, cred.audience)
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func (c *client) findOrCreateFederatedIdentityCredential(
	ctx context.Context, appObjectID string, subject string, audience string,
) (cloudsetup.CloudSetupResource, error) {
	oidcIssuer := c.oidcIssuer

	// Check if a federated identity credential already exists with the same issuer, subject, and audience
	existingID, existingName, found, err := c.graphClient.FindFederatedCredential(
		ctx, appObjectID, oidcIssuer, subject, audience)
	if err != nil {
		return cloudsetup.CloudSetupResource{}, err
	}
	if found {
		return cloudsetup.CloudSetupResource{
			Type:   ResourceTypeAzureFederatedCredential,
			ID:     existingID,
			Name:   existingName,
			Status: cloudsetup.ResourceStatusExisting,
		}, nil
	}

	// Create new federated identity credential
	name := federatedCredentialNamePrefix + "-" + uuid.NewString()
	description := "Pulumi ESC federated credential"
	credentialID, err := c.graphClient.CreateFederatedCredential(
		ctx, appObjectID, name, oidcIssuer, subject, audience, description)
	if err != nil {
		if isFederatedCredentialLimitError(err) {
			return cloudsetup.CloudSetupResource{}, fmt.Errorf(
				"the %q app registration in your Azure tenant has reached Azure's limit of 20 federated "+
					"identity credentials, "+
					"usually from credentials for Pulumi organizations or ESC environments that no longer exist. "+
					"In the Azure portal (Microsoft Entra ID → App registrations → %s → "+
					"Certificates & secrets → Federated credentials), "+
					"remove %q entries whose Subject references a Pulumi organization or environment you no longer use — "+
					"entries with an in-use Subject still provide that environment's Azure access — then retry. "+
					"Azure error: %w",
				c.appDisplayName, c.appDisplayName, federatedCredentialNamePrefix+"-*", err)
		}
		return cloudsetup.CloudSetupResource{}, err
	}

	return cloudsetup.CloudSetupResource{
		Type:   ResourceTypeAzureFederatedCredential,
		ID:     credentialID,
		Name:   name,
		Status: cloudsetup.ResourceStatusCreated,
	}, nil
}

func (c *client) findOrCreateServicePrincipal(
	ctx context.Context, appClientID string,
) (cloudsetup.CloudSetupResource, string, error) {
	// Check if a service principal already exists for this app ID
	principalID, found, err := c.graphClient.FindServicePrincipalByAppID(ctx, appClientID)
	if err != nil {
		return cloudsetup.CloudSetupResource{}, "", err
	}
	if found {
		return cloudsetup.CloudSetupResource{
			Type:   ResourceTypeAzureServicePrincipal,
			ID:     principalID,
			Name:   c.appDisplayName,
			Status: cloudsetup.ResourceStatusExisting,
		}, principalID, nil
	}

	// Create new service principal
	principalID, servicePrincipalName, err := c.graphClient.CreateServicePrincipal(ctx, appClientID)
	if err != nil {
		return cloudsetup.CloudSetupResource{}, "", err
	}

	return cloudsetup.CloudSetupResource{
		Type:   ResourceTypeAzureServicePrincipal,
		ID:     principalID,
		Name:   servicePrincipalName,
		Status: cloudsetup.ResourceStatusCreated,
	}, principalID, nil
}

func (c *client) assignPrincipalRole(
	ctx context.Context, subscriptionID string, principalID string, roleDefinitionID string, maxAttempts int,
) (cloudsetup.CloudSetupResource, error) {
	armClient, ok := c.roleAssignmentsClients[subscriptionID]
	if !ok {
		return cloudsetup.CloudSetupResource{}, errors.New("failed to create role assignments client")
	}

	// https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles/privileged#contributor
	roleDefID := fmt.Sprintf(
		"/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", subscriptionID, roleDefinitionID)

	// Create role assignment
	roleAssignmentName := uuid.NewString()
	roleAssignment := armauthorization.RoleAssignmentCreateParameters{
		Properties: &armauthorization.RoleAssignmentProperties{
			RoleDefinitionID: &roleDefID,
			PrincipalID:      &principalID,
		},
	}

	var resp armauthorization.RoleAssignmentsClientCreateResponse
	isExisting := false
	createRoleAssignment := func() error {
		var createErr error
		resp, createErr = armClient.Create(ctx, "/subscriptions/"+subscriptionID, roleAssignmentName, roleAssignment, nil)

		// Treat RoleAssignmentExists error as success
		if createErr != nil && strings.Contains(createErr.Error(), "RoleAssignmentExists") {
			isExisting = true
			return nil
		}

		return createErr
	}

	err := cloudsetup.RunWithRetries(ctx, maxAttempts, 5*time.Second, createRoleAssignment)
	if err != nil {
		return cloudsetup.CloudSetupResource{}, fmt.Errorf("failed to assign role after retries: %w", err)
	}

	if isExisting {
		// Role assignment already exists, return as existing resource
		return cloudsetup.CloudSetupResource{
			Type:   ResourceTypeAzureRoleAssignment,
			Status: cloudsetup.ResourceStatusExisting,
			Properties: map[string]string{
				"subscriptionID": subscriptionID,
			},
		}, nil
	}

	return cloudsetup.CloudSetupResource{
		Type:   ResourceTypeAzureRoleAssignment,
		ID:     *resp.ID,
		Name:   *resp.Name,
		Status: cloudsetup.ResourceStatusCreated,
		Properties: map[string]string{
			"subscriptionID": subscriptionID,
		},
	}, nil
}

func (c *client) ListAccounts(ctx context.Context) ([]cloudsetup.CloudAccount, error) {
	if c.subscriptionsClient == nil {
		return nil, errors.New("failed to create subscriptions client")
	}

	pager := c.subscriptionsClient.NewListPager(nil)
	subscriptions := []cloudsetup.CloudAccount{}
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, sub := range page.Value {
			subscriptions = append(subscriptions, cloudsetup.CloudAccount{ID: *sub.SubscriptionID, Name: *sub.DisplayName})
		}
	}

	return subscriptions, nil
}

func (c *client) ListTenants(ctx context.Context) ([]cloudsetup.CloudAccount, error) {
	if c.tenantsClient == nil {
		return nil, errors.New("failed to create tenants client")
	}

	// ARM's tenants endpoint reports every tenant the signed-in user can reach, not just the one
	// the token was issued for, so a home-tenant credential is enough to enumerate the rest.
	tenants := []cloudsetup.CloudAccount{}
	pager := c.tenantsClient.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, tenant := range page.Value {
			if tenant.TenantID == nil {
				continue
			}
			name := ""
			if tenant.DisplayName != nil {
				name = *tenant.DisplayName
			}
			tenants = append(tenants, cloudsetup.CloudAccount{ID: *tenant.TenantID, Name: name})
		}
	}

	return tenants, nil
}
