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
	"net/http"
	"net/url"
	"slices"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
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

// graphBaseURL is the Microsoft Graph v1.0 endpoint.
const graphBaseURL = "https://graph.microsoft.com/v1.0"

// graphScope is the OAuth scope requested for Microsoft Graph calls; ".default" grants whatever
// Graph permissions the caller's credential already holds.
var graphScope = []string{"https://graph.microsoft.com/.default"}

// graphClient talks to Microsoft Graph directly over REST. It replaces the msgraph-sdk-go client,
// whose generated bindings for the entire Graph API added roughly 150MB to the CLI binary; only a
// handful of endpoints are needed here.
type graphClient struct {
	pipeline runtime.Pipeline
}

// NewGraphClient builds a GraphClient backed by direct Microsoft Graph REST calls, authenticated
// with the given credential (which must be able to issue tokens for the Graph scope).
func NewGraphClient(cred azcore.TokenCredential) GraphClient {
	authPolicy := runtime.NewBearerTokenPolicy(cred, graphScope, nil)
	pl := runtime.NewPipeline("pulumi-cloudsetup-graph", "v1.0",
		runtime.PipelineOptions{PerRetry: []policy.Policy{authPolicy}}, nil)
	return &graphClient{pipeline: pl}
}

// Request/response bodies for the Graph endpoints used below. Only the fields we read or write are
// modeled; omitempty keeps PATCH/POST bodies to just the properties being set.
type (
	graphApp struct {
		ID             string    `json:"id,omitempty"`
		AppID          string    `json:"appId,omitempty"`
		DisplayName    string    `json:"displayName,omitempty"`
		SignInAudience string    `json:"signInAudience,omitempty"`
		Notes          *string   `json:"notes,omitempty"`
		Description    *string   `json:"description,omitempty"`
		Web            *graphWeb `json:"web,omitempty"`
	}
	graphWeb struct {
		HomePageURL *string `json:"homePageUrl,omitempty"`
	}
	graphFedCred struct {
		ID          string   `json:"id,omitempty"`
		Name        string   `json:"name,omitempty"`
		Issuer      string   `json:"issuer,omitempty"`
		Subject     string   `json:"subject,omitempty"`
		Description string   `json:"description,omitempty"`
		Audiences   []string `json:"audiences,omitempty"`
	}
	graphServicePrincipal struct {
		ID          string `json:"id,omitempty"`
		AppID       string `json:"appId,omitempty"`
		DisplayName string `json:"displayName,omitempty"`
	}
	// graphList is the OData collection envelope Graph wraps list results in.
	graphList[T any] struct {
		Value []T `json:"value"`
	}
)

// do issues a Graph request, marshaling body when non-nil and verifying the response status. When
// okCodes is empty, 200 OK is required. The response is returned for the caller to unmarshal.
func (c *graphClient) do(
	ctx context.Context, method, path string, query url.Values, body any, okCodes ...int,
) (*http.Response, error) {
	req, err := runtime.NewRequest(ctx, method, graphBaseURL+path)
	if err != nil {
		return nil, err
	}
	if len(query) > 0 {
		req.Raw().URL.RawQuery = query.Encode()
	}
	if body != nil {
		if err := runtime.MarshalAsJSON(req, body); err != nil {
			return nil, err
		}
	}
	resp, err := c.pipeline.Do(req)
	if err != nil {
		return nil, err
	}
	if len(okCodes) == 0 {
		okCodes = []int{http.StatusOK}
	}
	if !runtime.HasStatusCode(resp, okCodes...) {
		return nil, runtime.NewResponseError(resp)
	}
	return resp, nil
}

func (c *graphClient) FindAppRegistrationByName(
	ctx context.Context, displayName, signInAudience string,
) (appObjectID, appClientID string, found bool, err error) {
	query := url.Values{"$filter": {fmt.Sprintf(
		"displayName eq '%s' and signInAudience eq '%s'", displayName, signInAudience)}}
	resp, err := c.do(ctx, http.MethodGet, "/applications", query, nil)
	if err != nil {
		return "", "", false, err
	}
	var list graphList[graphApp]
	if err := runtime.UnmarshalAsJSON(resp, &list); err != nil {
		return "", "", false, err
	}
	if len(list.Value) > 0 {
		app := list.Value[len(list.Value)-1]
		return app.ID, app.AppID, true, nil
	}
	return "", "", false, nil
}

func (c *graphClient) GetAppRegistrationByObjectID(
	ctx context.Context, appObjectID string,
) (appClientID, displayName string, err error) {
	resp, err := c.do(ctx, http.MethodGet, "/applications/"+url.PathEscape(appObjectID), nil, nil)
	if err != nil {
		return "", "", err
	}
	var app graphApp
	if err := runtime.UnmarshalAsJSON(resp, &app); err != nil {
		return "", "", err
	}
	if app.AppID == "" {
		return "", "", fmt.Errorf("application %s is missing an app ID", appObjectID)
	}
	if app.DisplayName == "" {
		return "", "", fmt.Errorf("application %s is missing a display name", appObjectID)
	}
	return app.AppID, app.DisplayName, nil
}

func (c *graphClient) CreateAppRegistration(
	ctx context.Context, displayName, signInAudience string,
) (appObjectID, appClientID string, err error) {
	body := graphApp{DisplayName: displayName, SignInAudience: signInAudience}
	resp, err := c.do(ctx, http.MethodPost, "/applications", nil, body, http.StatusCreated)
	if err != nil {
		return "", "", err
	}
	var app graphApp
	if err := runtime.UnmarshalAsJSON(resp, &app); err != nil {
		return "", "", err
	}
	return app.ID, app.AppID, nil
}

func (c *graphClient) UpdateAppRegistration(
	ctx context.Context, appObjectID string, notes, description, homePageURL *string,
) error {
	body := graphApp{Notes: notes, Description: description}
	if homePageURL != nil {
		body.Web = &graphWeb{HomePageURL: homePageURL}
	}
	_, err := c.do(ctx, http.MethodPatch,
		"/applications/"+url.PathEscape(appObjectID), nil, body, http.StatusNoContent)
	return err
}

func (c *graphClient) FindFederatedCredential(
	ctx context.Context, appObjectID, issuer, subject, audience string,
) (found bool, err error) {
	resp, err := c.do(ctx, http.MethodGet,
		"/applications/"+url.PathEscape(appObjectID)+"/federatedIdentityCredentials", nil, nil)
	if err != nil {
		return false, err
	}
	var list graphList[graphFedCred]
	if err := runtime.UnmarshalAsJSON(resp, &list); err != nil {
		return false, err
	}
	for _, fc := range list.Value {
		if fc.Issuer == issuer && fc.Subject == subject && slices.Contains(fc.Audiences, audience) {
			return true, nil
		}
	}
	return false, nil
}

func (c *graphClient) CreateFederatedCredential(
	ctx context.Context, appObjectID, name, issuer, subject, audience, description string,
) (credentialID string, err error) {
	body := graphFedCred{
		Name:        name,
		Issuer:      issuer,
		Subject:     subject,
		Description: description,
		Audiences:   []string{audience},
	}
	resp, err := c.do(ctx, http.MethodPost,
		"/applications/"+url.PathEscape(appObjectID)+"/federatedIdentityCredentials",
		nil, body, http.StatusCreated)
	if err != nil {
		return "", err
	}
	var fc graphFedCred
	if err := runtime.UnmarshalAsJSON(resp, &fc); err != nil {
		return "", err
	}
	return fc.ID, nil
}

func (c *graphClient) FindServicePrincipalByAppID(
	ctx context.Context, appClientID string,
) (principalID string, found bool, err error) {
	query := url.Values{"$filter": {fmt.Sprintf("appId eq '%s'", appClientID)}}
	resp, err := c.do(ctx, http.MethodGet, "/servicePrincipals", query, nil)
	if err != nil {
		return "", false, err
	}
	var list graphList[graphServicePrincipal]
	if err := runtime.UnmarshalAsJSON(resp, &list); err != nil {
		return "", false, err
	}
	if len(list.Value) > 0 {
		return list.Value[0].ID, true, nil
	}
	return "", false, nil
}

func (c *graphClient) GetServicePrincipalByID(
	ctx context.Context, principalID string,
) (appClientID string, err error) {
	resp, err := c.do(ctx, http.MethodGet, "/servicePrincipals/"+url.PathEscape(principalID), nil, nil)
	if err != nil {
		return "", err
	}
	var sp graphServicePrincipal
	if err := runtime.UnmarshalAsJSON(resp, &sp); err != nil {
		return "", err
	}
	if sp.AppID == "" {
		return "", fmt.Errorf("service principal %s is missing an app ID", principalID)
	}
	return sp.AppID, nil
}

func (c *graphClient) CreateServicePrincipal(
	ctx context.Context, appClientID string,
) (principalID, displayName string, err error) {
	body := graphServicePrincipal{AppID: appClientID}
	resp, err := c.do(ctx, http.MethodPost, "/servicePrincipals", nil, body, http.StatusCreated)
	if err != nil {
		return "", "", err
	}
	var sp graphServicePrincipal
	if err := runtime.UnmarshalAsJSON(resp, &sp); err != nil {
		return "", "", err
	}
	return sp.ID, sp.DisplayName, nil
}

func (c *graphClient) DeleteAppRegistration(ctx context.Context, appObjectID string) error {
	_, err := c.do(ctx, http.MethodDelete,
		"/applications/"+url.PathEscape(appObjectID), nil, nil, http.StatusNoContent)
	return err
}
