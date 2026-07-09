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
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/pgavlin/fx/v2"

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

	appRegistrationDisplayName = "pulumi-esc-oidc-app"
)

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

// LocalClient is a Client that can also resolve the tenant of a set of subscriptions. It is
// returned by NewClientFromCredential; kept separate from Client so the Client interface the
// Pulumi service consumes stays unchanged.
type LocalClient interface {
	Client
	// Tenant returns the Azure tenant the given subscriptions belong to, erroring if they
	// span more than one: OIDC setup targets a single tenant (one app registration).
	Tenant(ctx context.Context, subscriptionIDs []string) (string, error)
}

type client struct {
	oidcIssuer string
	// appDisplayName is the display name used to find-or-create the app registration. A
	// distinct name yields a distinct app registration, each with its own 20-credential budget.
	appDisplayName string

	graphClient            GraphClient
	subscriptionsClient    SubscriptionsClient
	roleAssignmentsClients map[string]RoleAssignmentsClient

	maxRetryAttempts int
}

// NewClient builds a Client from pre-fetched ARM and Microsoft Graph access tokens, as the
// Pulumi service does after collecting them through its browser OAuth flow.
func NewClient(armAccessToken string, graphAccessToken string, oidcIssuer string, subscriptionIDs []string) Client {
	return newClient(
		&StaticTokenCredential{token: armAccessToken},
		&StaticTokenCredential{token: graphAccessToken},
		oidcIssuer, "", subscriptionIDs)
}

// NewClientFromCredential builds a Client from a single azcore.TokenCredential that serves both
// the ARM and Microsoft Graph scopes, for callers authenticating locally (e.g. an azidentity
// credential backed by `az login` or a browser sign-in) rather than with pre-fetched tokens.
//
// A real credential honors the scope each SDK client requests, so one credential covers both
// ARM and Graph; the service needs two tokens only because its static credential cannot.
//
// appDisplayName is the app registration to find-or-create; empty uses the default name.
func NewClientFromCredential(
	cred azcore.TokenCredential, oidcIssuer, appDisplayName string, subscriptionIDs []string,
) LocalClient {
	return newClient(cred, cred, oidcIssuer, appDisplayName, subscriptionIDs)
}

func newClient(
	armCred, graphCred azcore.TokenCredential, oidcIssuer, appDisplayName string, subscriptionIDs []string,
) *client {
	if appDisplayName == "" {
		appDisplayName = appRegistrationDisplayName
	}
	client := &client{
		oidcIssuer:             oidcIssuer,
		appDisplayName:         appDisplayName,
		roleAssignmentsClients: make(map[string]RoleAssignmentsClient),
		maxRetryAttempts:       6,
	}

	graphClient, err := msgraphsdk.NewGraphServiceClientWithCredentials(graphCred, []string{})
	if err == nil {
		client.graphClient = NewGraphClientWrapper(graphClient)
	}

	subClient, err := armsubscriptions.NewClient(armCred, nil)
	if err == nil {
		client.subscriptionsClient = subClient
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

	// Create or find existing federated identity credentials
	// We need a federated identity credential for each environment name (which is part of the subject claim),
	// as well as <yaml> which is used by IAC. Environments shared across envInfos (the org-scoped
	// setup shape: one environment, many subscriptions) need only one credential.
	envIdentifiers := []string{"<yaml>"}
	seenEnvIdentifiers := fx.NewSet(envIdentifiers...)
	for _, envInfo := range envInfos {
		envIdentifier := fmt.Sprintf("%s/%s", envInfo.ProjectName, envInfo.EnvironmentName)
		if seenEnvIdentifiers.Has(envIdentifier) {
			continue
		}
		seenEnvIdentifiers.Add(envIdentifier)
		envIdentifiers = append(envIdentifiers, envIdentifier)
	}

	for _, envIdentifier := range envIdentifiers {
		fedCredResource, err := c.findOrCreateFederatedIdentityCredential(
			ctx, appObjectID, c.oidcIssuer, orgName, envIdentifier)
		if err != nil {
			return cloudsetup.WrapSetupError(result, ResourceTypeAzureFederatedCredential, err)
		}
		result.Resources = append(result.Resources, fedCredResource)
	}

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

	if err == nil && found {
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

func (c *client) findOrCreateFederatedIdentityCredential(
	ctx context.Context, appObjectID string, oidcIssuer string, orgName string, envIdentifier string,
) (cloudsetup.CloudSetupResource, error) {
	subject := fmt.Sprintf("pulumi:environments:org:%s:env:%s", orgName, envIdentifier)
	audience := "azure:" + orgName

	// Check if a federated identity credential already exists with the same issuer, subject, and audience
	found, err := c.graphClient.FindFederatedCredential(ctx, appObjectID, oidcIssuer, subject, audience)

	if err == nil && found {
		name := "pulumi-esc-oidc-credential-" + uuid.NewString()
		return cloudsetup.CloudSetupResource{
			Type:   ResourceTypeAzureFederatedCredential,
			ID:     fmt.Sprintf("%s/%s", appObjectID, name),
			Name:   name,
			Status: cloudsetup.ResourceStatusExisting,
		}, nil
	}

	// Create new federated identity credential
	name := "pulumi-esc-oidc-credential-" + uuid.NewString()
	description := "Pulumi ESC federated credential"
	credentialID, err := c.graphClient.CreateFederatedCredential(
		ctx, appObjectID, name, oidcIssuer, subject, audience, description)
	if err != nil {
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

	if err == nil && found {
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
			ID:     roleDefID,
			Name:   roleAssignmentName,
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

func (c *client) Tenant(ctx context.Context, subscriptionIDs []string) (string, error) {
	if c.subscriptionsClient == nil {
		return "", errors.New("failed to create subscriptions client")
	}

	// Map every visible subscription to its tenant, then resolve the requested ones. The
	// subscription object carries its tenant directly, so no separate lookup is needed.
	tenantBySub := map[string]string{}
	pager := c.subscriptionsClient.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return "", err
		}
		for _, sub := range page.Value {
			if sub.SubscriptionID != nil && sub.TenantID != nil {
				tenantBySub[*sub.SubscriptionID] = *sub.TenantID
			}
		}
	}

	tenant := ""
	for _, id := range subscriptionIDs {
		t, ok := tenantBySub[id]
		if !ok {
			return "", fmt.Errorf("subscription %s is not accessible", id)
		}
		if tenant == "" {
			tenant = t
		} else if tenant != t {
			return "", errors.New("selected subscriptions span more than one tenant; set --tenant to choose one")
		}
	}
	if tenant == "" {
		return "", errors.New("could not determine the tenant for the selected subscriptions")
	}
	return tenant, nil
}
