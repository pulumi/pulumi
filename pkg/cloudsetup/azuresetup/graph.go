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

package azuresetup

import (
	"context"
	"fmt"
	"slices"

	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/applications"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/serviceprincipals"
)

// GraphClient is a wrapper around the Azure Graph API
type GraphClient interface {
	FindAppRegistrationByName(
		ctx context.Context, displayName, signInAudience string,
	) (appObjectID, appClientID string, found bool, err error)
	GetAppRegistrationByObjectID(
		ctx context.Context, appObjectID string,
	) (appClientID, displayName string, err error)
	CreateAppRegistration(
		ctx context.Context, displayName, signInAudience string,
	) (appObjectID, appClientID string, err error)
	UpdateAppRegistration(ctx context.Context, appObjectID string, notes, description, homePageURL *string) error
	FindFederatedCredential(
		ctx context.Context, appObjectID, issuer, subject, audience string,
	) (found bool, err error)
	CreateFederatedCredential(
		ctx context.Context, appObjectID, name, issuer, subject, audience, description string,
	) (credentialID string, err error)
	FindServicePrincipalByAppID(ctx context.Context, appClientID string) (principalID string, found bool, err error)
	GetServicePrincipalByID(ctx context.Context, principalID string) (appClientID string, err error)
	CreateServicePrincipal(ctx context.Context, appClientID string) (principalID, displayName string, err error)
	DeleteAppRegistration(ctx context.Context, appObjectID string) error
}

// graphClientWrapper wraps the real Microsoft Graph SDK to implement our GraphClient interface
type graphClientWrapper struct {
	client *msgraphsdk.GraphServiceClient
}

// NewGraphClientWrapper creates a GraphClient from a Microsoft Graph SDK client.
// This is exported to allow other packages to create GraphClient instances.
func NewGraphClientWrapper(client *msgraphsdk.GraphServiceClient) GraphClient {
	return &graphClientWrapper{client: client}
}

func (g *graphClientWrapper) FindAppRegistrationByName(
	ctx context.Context, displayName, signInAudience string,
) (appObjectID, appClientID string, found bool, err error) {
	filter := fmt.Sprintf("displayName eq '%s' and signInAudience eq '%s'", displayName, signInAudience)
	requestConfig := &applications.ApplicationsRequestBuilderGetRequestConfiguration{
		QueryParameters: &applications.ApplicationsRequestBuilderGetQueryParameters{
			Filter: &filter,
		},
	}
	existingApps, err := g.client.Applications().Get(ctx, requestConfig)
	if err != nil {
		return "", "", false, err
	}

	if len(existingApps.GetValue()) > 0 {
		existingAppsList := existingApps.GetValue()
		existingApp := existingAppsList[len(existingAppsList)-1]
		return *existingApp.GetId(), *existingApp.GetAppId(), true, nil
	}

	return "", "", false, nil
}

func (g *graphClientWrapper) GetAppRegistrationByObjectID(
	ctx context.Context, appObjectID string,
) (appClientID, displayName string, err error) {
	app, err := g.client.Applications().ByApplicationId(appObjectID).Get(ctx, nil)
	if err != nil {
		return "", "", err
	}
	appID := app.GetAppId()
	if appID == nil {
		return "", "", fmt.Errorf("application %s is missing an app ID", appObjectID)
	}
	name := app.GetDisplayName()
	if name == nil {
		return "", "", fmt.Errorf("application %s is missing a display name", appObjectID)
	}
	return *appID, *name, nil
}

func (g *graphClientWrapper) CreateAppRegistration(
	ctx context.Context, displayName, signInAudience string,
) (appObjectID, appClientID string, err error) {
	app := models.NewApplication()
	app.SetDisplayName(&displayName)
	app.SetSignInAudience(&signInAudience)

	createdApp, err := g.client.Applications().Post(ctx, app, nil)
	if err != nil {
		return "", "", err
	}

	return *createdApp.GetId(), *createdApp.GetAppId(), nil
}

func (g *graphClientWrapper) UpdateAppRegistration(
	ctx context.Context, appObjectID string, notes, description, homePageURL *string,
) error {
	app := models.NewApplication()

	if notes != nil {
		app.SetNotes(notes)
	}
	if description != nil {
		app.SetDescription(description)
	}
	if homePageURL != nil {
		web := models.NewWebApplication()
		web.SetHomePageUrl(homePageURL)
		app.SetWeb(web)
	}

	_, err := g.client.Applications().ByApplicationId(appObjectID).Patch(ctx, app, nil)
	return err
}

func (g *graphClientWrapper) FindFederatedCredential(
	ctx context.Context, appObjectID, issuer, subject, audience string,
) (found bool, err error) {
	existingFedCreds, err := g.client.Applications().
		ByApplicationId(appObjectID).FederatedIdentityCredentials().Get(ctx, nil)
	if err != nil {
		return false, err
	}

	if existingFedCreds != nil && existingFedCreds.GetValue() != nil {
		for _, existing := range existingFedCreds.GetValue() {
			if existing.GetIssuer() != nil && *existing.GetIssuer() == issuer &&
				existing.GetSubject() != nil && *existing.GetSubject() == subject &&
				slices.Contains(existing.GetAudiences(), audience) {
				return true, nil
			}
		}
	}

	return false, nil
}

func (g *graphClientWrapper) CreateFederatedCredential(
	ctx context.Context, appObjectID, name, issuer, subject, audience, description string,
) (credentialID string, err error) {
	fedCred := models.NewFederatedIdentityCredential()
	fedCred.SetName(&name)
	fedCred.SetIssuer(&issuer)
	fedCred.SetSubject(&subject)
	fedCred.SetDescription(&description)
	fedCred.SetAudiences([]string{audience})

	createdFedCred, err := g.client.Applications().
		ByApplicationId(appObjectID).FederatedIdentityCredentials().Post(ctx, fedCred, nil)
	if err != nil {
		return "", err
	}

	return *createdFedCred.GetId(), nil
}

func (g *graphClientWrapper) FindServicePrincipalByAppID(
	ctx context.Context, appClientID string,
) (principalID string, found bool, err error) {
	filter := fmt.Sprintf("appId eq '%s'", appClientID)
	requestConfig := &serviceprincipals.ServicePrincipalsRequestBuilderGetRequestConfiguration{
		QueryParameters: &serviceprincipals.ServicePrincipalsRequestBuilderGetQueryParameters{
			Filter: &filter,
		},
	}

	existingPrincipals, err := g.client.ServicePrincipals().Get(ctx, requestConfig)
	if err != nil {
		return "", false, err
	}

	if existingPrincipals != nil && len(existingPrincipals.GetValue()) > 0 {
		existing := existingPrincipals.GetValue()[0]
		return *existing.GetId(), true, nil
	}

	return "", false, nil
}

func (g *graphClientWrapper) GetServicePrincipalByID(
	ctx context.Context, principalID string,
) (appClientID string, err error) {
	servicePrincipal, err := g.client.ServicePrincipals().ByServicePrincipalId(principalID).Get(ctx, nil)
	if err != nil {
		return "", err
	}
	appID := servicePrincipal.GetAppId()
	if appID == nil {
		return "", fmt.Errorf("service principal %s is missing an app ID", principalID)
	}
	return *appID, nil
}

func (g *graphClientWrapper) CreateServicePrincipal(
	ctx context.Context, appClientID string,
) (principalID, displayName string, err error) {
	servicePrincipal := models.NewServicePrincipal()
	servicePrincipal.SetAppId(&appClientID)

	createdServicePrincipal, err := g.client.ServicePrincipals().Post(ctx, servicePrincipal, nil)
	if err != nil {
		return "", "", err
	}

	return *createdServicePrincipal.GetId(), *createdServicePrincipal.GetDisplayName(), nil
}

func (g *graphClientWrapper) DeleteAppRegistration(ctx context.Context, appObjectID string) error {
	return g.client.Applications().ByApplicationId(appObjectID).Delete(ctx, nil)
}
