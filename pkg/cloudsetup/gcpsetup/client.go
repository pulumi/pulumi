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

// Package gcpsetup provides GCP-specific cloud setup functionality
package gcpsetup

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	fxs "github.com/pgavlin/fx/v2/slices"
	"golang.org/x/oauth2"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
	"google.golang.org/api/serviceusage/v1"

	cloudsetup "github.com/pulumi/pulumi/pkg/v3/cloudsetup/common"
)

//nolint:gosec // (G101)
const (
	ResourceTypeGCPWorkloadIdentityPool     = "gcp:iam:workload-identity-pool"
	ResourceTypeGCPWorkloadIdentityProvider = "gcp:iam:workload-identity-provider"
	ResourceTypeGCPServiceAccount           = "gcp:iam:service-account"
	ResourceTypeGCPIAMBinding               = "gcp:iam:binding"
)

type Config struct {
	AccessToken string
}

type Client interface {
	SetupOIDCInfrastructure(
		ctx context.Context, orgName string, projectID string, oidcServiceAccountName string, role string,
	) (*cloudsetup.CloudSetupResult, error)
	ListAccounts(ctx context.Context) ([]cloudsetup.CloudAccount, error)
}

type client struct {
	crmClient          crmClient
	iamClient          iamClient
	serviceUsageClient serviceUsageClient
	oidcIssuer         string

	maxRetryAttempts int
}

// NewClient builds a Client from a pre-fetched OAuth access token, as the Pulumi service does
// after collecting it through its browser OAuth flow.
func NewClient(ctx context.Context, cfg Config, oidcIssuer string) (Client, error) {
	if cfg.AccessToken == "" {
		return nil, cloudsetup.NewSetupError(cloudsetup.ErrorCodeInvalidCredentials, "access token is required", nil)
	}

	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: cfg.AccessToken,
	})
	return newClient(ctx, oidcIssuer, option.WithTokenSource(tokenSource))
}

// NewClientFromADC builds a Client from Google Application Default Credentials, for callers
// authenticating locally (`gcloud auth application-default login`, a service-account key, or the
// metadata server) rather than with a pre-fetched token.
func NewClientFromADC(ctx context.Context, oidcIssuer string) (Client, error) {
	return newClient(ctx, oidcIssuer)
}

func newClient(ctx context.Context, oidcIssuer string, authOpts ...option.ClientOption) (Client, error) {
	// Each Google service takes the auth option plus its own scope; build a fresh slice per
	// call so append never aliases authOpts.
	withScope := func(scope string) []option.ClientOption {
		opts := append([]option.ClientOption{}, authOpts...)
		return append(opts, option.WithScopes(scope), option.WithRequestReason("pulumi-oidc-setup"))
	}

	// look up the project number
	crmService, err := cloudresourcemanager.NewService(ctx,
		withScope("https://www.googleapis.com/auth/cloudplatformprojects.readonly")...)
	if err != nil {
		return nil, cloudsetup.NewSetupError(cloudsetup.ErrorCodeInvalidCredentials, "failed to create CRM service", err)
	}

	iamService, err := iam.NewService(ctx, withScope("https://www.googleapis.com/auth/iam")...)
	if err != nil {
		return nil, cloudsetup.NewSetupError(cloudsetup.ErrorCodeInvalidCredentials, "failed to create IAM service", err)
	}

	serviceUsageService, err := serviceusage.NewService(ctx,
		withScope("https://www.googleapis.com/auth/service.management")...)
	if err != nil {
		return nil, cloudsetup.NewSetupError(
			cloudsetup.ErrorCodeInvalidCredentials, "failed to create Service Usage service", err)
	}

	return &client{
		crmClient:          &realCRMClient{crmService},
		iamClient:          &realIAMClient{iamService},
		serviceUsageClient: &realServiceUsageClient{serviceUsageService},
		oidcIssuer:         oidcIssuer,
		maxRetryAttempts:   6,
	}, nil
}

func (c *client) SetupOIDCInfrastructure(
	ctx context.Context, orgName string, projectID string, oidcServiceAccountName string, role string,
) (*cloudsetup.CloudSetupResult, error) {
	result := &cloudsetup.CloudSetupResult{
		Success:   false,
		Resources: []cloudsetup.CloudSetupResource{},
	}

	// Lookup project number
	project, err := c.crmClient.GetProject(ctx, projectID)
	if err != nil {
		return nil, cloudsetup.NewSetupError(cloudsetup.ErrorCodeInvalidCredentials, "failed to lookup project", err)
	}

	// Enable IAM Service Account Credentials API
	const iamCredentialsAPI = "iamcredentials.googleapis.com"
	err = c.serviceUsageClient.EnableService(ctx, projectID, iamCredentialsAPI)
	if err != nil {
		return nil, cloudsetup.NewSetupError(
			cloudsetup.ErrorCodeSetupFailed, "failed to enable IAM Service Account Credentials API", err)
	}

	// Create Workload Identity Pool for Pulumi
	const poolID = "pulumi-cloud"

	pool := &iam.WorkloadIdentityPool{
		DisplayName: "Pulumi OIDC Pool",
		Description: "OIDC setup for Pulumi ESC",
	}

	err = c.iamClient.CreateWorkloadIdentityPool(ctx, project.ProjectId, poolID, pool)
	if err != nil && !isAlreadyExistsError(err) {
		return cloudsetup.WrapSetupError(result, ResourceTypeGCPWorkloadIdentityPool, err)
	}

	result.Resources = append(result.Resources, cloudsetup.CloudSetupResource{
		Type:   ResourceTypeGCPWorkloadIdentityPool,
		ID:     poolID,
		Name:   "PulumiWorkloadIdentityPool",
		Status: status(isAlreadyExistsError(err)),
	})

	// Create org specific Workload Identity Provider
	providerID := safeProviderID(orgName)
	audience := "gcp:" + orgName

	provider := &iam.WorkloadIdentityPoolProvider{
		DisplayName: providerID,
		Description: "Allows Pulumi ESC to assume roles via OIDC",
		Oidc: &iam.Oidc{
			IssuerUri:        c.oidcIssuer,
			AllowedAudiences: []string{audience},
		},
		AttributeMapping: map[string]string{
			"google.subject":     "assertion.sub",
			"attribute.oidc_aud": "assertion.aud",
		},
	}

	err = c.iamClient.CreateWorkloadIdentityProvider(ctx, project.ProjectId, poolID, providerID, provider)
	if err != nil && !isAlreadyExistsError(err) {
		return cloudsetup.WrapSetupError(result, ResourceTypeGCPWorkloadIdentityProvider, err)
	}

	result.Resources = append(result.Resources, cloudsetup.CloudSetupResource{
		Type:   ResourceTypeGCPWorkloadIdentityProvider,
		ID:     providerID,
		Name:   "PulumiOIDCProvider",
		Status: status(isAlreadyExistsError(err)),
	})

	// Create Service Account
	saID := oidcServiceAccountName
	saEmail := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", saID, project.ProjectId)
	saName := fmt.Sprintf("projects/%s/serviceAccounts/%s", project.ProjectId, saEmail)

	serviceAccount := &iam.CreateServiceAccountRequest{
		AccountId: oidcServiceAccountName,
		ServiceAccount: &iam.ServiceAccount{
			DisplayName: "Pulumi OIDC Service Account",
		},
	}

	_, err = c.iamClient.CreateServiceAccount(ctx, project.ProjectId, serviceAccount)
	if err != nil && !isAlreadyExistsError(err) {
		return cloudsetup.WrapSetupError(result, ResourceTypeGCPServiceAccount, err)
	}

	result.Resources = append(result.Resources, cloudsetup.CloudSetupResource{
		Type:   ResourceTypeGCPServiceAccount,
		ID:     saEmail,
		Name:   saID,
		Status: status(isAlreadyExistsError(err)),
	})

	// Update IAM policy for the Service Account
	policy, err := c.iamClient.GetServiceAccountPolicy(ctx, saName)
	if err != nil {
		return cloudsetup.WrapSetupError(result, ResourceTypeGCPIAMBinding, err)
	}

	// Grant the user-specified role to the Service Account
	roleBinding := &iam.Binding{
		Role:    role,
		Members: []string{"serviceAccount:" + saEmail},
	}

	rbStatus := cloudsetup.ResourceStatusExisting
	if !bindingExists(policy.Bindings, roleBinding) {
		rbStatus = cloudsetup.ResourceStatusCreated
		policy.Bindings = append(policy.Bindings, roleBinding)
	}

	// Allow the org-specific oidc provider to assume the Service Account
	workloadIdentityUser := &iam.Binding{
		Role: "roles/iam.workloadIdentityUser",
		Members: []string{
			// An oddity of IAM principals is that they require the project to be referenced by number, not by project ID
			// Restrict the principal to only the org-specific provider (by matching the audience attribute)
			fmt.Sprintf(
				"principalSet://iam.googleapis.com/projects/%d/locations/global/workloadIdentityPools/%s/attribute.oidc_aud/%s",
				project.ProjectNumber, poolID, audience),
		},
	}

	wiStatus := cloudsetup.ResourceStatusExisting
	if !bindingExists(policy.Bindings, workloadIdentityUser) {
		wiStatus = cloudsetup.ResourceStatusCreated
		policy.Bindings = append(policy.Bindings, workloadIdentityUser)
	}

	// Set updated IAM policy if necessary
	if rbStatus == cloudsetup.ResourceStatusCreated || wiStatus == cloudsetup.ResourceStatusCreated {
		setPolicy := func() error {
			_, err := c.iamClient.SetServiceAccountPolicy(ctx, saName, policy)
			return err
		}

		err = cloudsetup.RunWithRetries(ctx, c.maxRetryAttempts, 5*time.Second, setPolicy)
		if err != nil {
			return cloudsetup.WrapSetupError(result, ResourceTypeGCPIAMBinding, err)
		}
	}

	result.Resources = append(result.Resources, cloudsetup.CloudSetupResource{
		Type:   ResourceTypeGCPIAMBinding,
		ID:     fmt.Sprintf("%s/%s", saEmail, role),
		Name:   "RoleBinding",
		Status: rbStatus,
	})
	result.Resources = append(result.Resources, cloudsetup.CloudSetupResource{
		Type:   ResourceTypeGCPIAMBinding,
		ID:     saEmail + "/roles/iam.workloadIdentityUser",
		Name:   "WorkloadIdentityBinding",
		Status: wiStatus,
	})

	result.Success = true
	return result, nil
}

// provider id must be 4-32 characters long and can only contain lowercase letters, digits, or dashes.
func safeProviderID(orgName string) string {
	// Convert orgName to lowercase
	id := strings.ToLower(orgName)

	// Replace any non-allowed characters with dashes
	id = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, id)

	// Ensure length is between 4-32 characters
	if len(id) < 4 {
		id = "pulumi-" + id
	}
	if len(id) > 32 {
		id = id[:32]
	}

	return id
}

// Helper function to check if error is "already exists"
func isAlreadyExistsError(err error) bool {
	// Check for Google API error with 409 status code (conflict)
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		return apiErr.Code == http.StatusConflict
	}
	return false
}

// Helper function to check if a binding already exists in a list of IAM bindings
func bindingExists(bindings []*iam.Binding, newBinding *iam.Binding) bool {
	for _, b := range bindings {
		if b.Role == newBinding.Role {
			if slices.Contains(b.Members, newBinding.Members[0]) {
				return true
			}
		}
	}
	return false
}

// Helper function to return the status of a resource based on whether it already exists
func status(existing bool) string {
	if existing {
		return cloudsetup.ResourceStatusExisting
	}
	return cloudsetup.ResourceStatusCreated
}

func (c *client) ListAccounts(ctx context.Context) ([]cloudsetup.CloudAccount, error) {
	projects, err := c.crmClient.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	return slices.Collect(fxs.Map(projects, func(project *cloudresourcemanager.Project) cloudsetup.CloudAccount {
		return cloudsetup.CloudAccount{
			ID:     project.ProjectId,
			Name:   project.Name,
			Number: project.ProjectNumber,
		}
	})), nil
}
