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
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v3"
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
		ctx context.Context, orgName string, orgID string, projectID string, oidcServiceAccountName string, role string,
		escEnvironmentName string,
	) (*cloudsetup.CloudSetupResult, error)
	// ListAccounts returns GCP projects for onboarding discovery. A non-empty
	// gcpOrganizationID scopes results to that organization — both projects directly
	// under it and projects nested under its descendant folders; callers must validate
	// non-empty IDs before invoking this method. An empty gcpOrganizationID returns
	// every project the OAuth principal can access.
	ListAccounts(ctx context.Context, gcpOrganizationID string) ([]cloudsetup.CloudAccount, error)
}

type client struct {
	crmClient          crmClient
	iamClient          iamClient
	serviceUsageClient serviceUsageClient
	oidcIssuer         string

	maxRetryAttempts int
	retryDelay       time.Duration
}

// NewClient builds a Client from a pre-fetched OAuth access token.
func NewClient(ctx context.Context, cfg Config, oidcIssuer string) (Client, error) {
	if cfg.AccessToken == "" {
		return nil, cloudsetup.NewSetupError(cloudsetup.ErrorCodeInvalidCredentials, "access token is required", nil)
	}

	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: cfg.AccessToken,
	})
	return newClient(ctx, oidcIssuer, option.WithTokenSource(tokenSource))
}

// NewClientFromADC builds a Client from Google Application Default Credentials.
func NewClientFromADC(ctx context.Context, oidcIssuer string) (Client, error) {
	return newClient(ctx, oidcIssuer)
}

func newClient(ctx context.Context, oidcIssuer string, authOpts ...option.ClientOption) (Client, error) {
	// Build a fresh option slice per call so append never aliases authOpts.
	// No explicit scopes: granting a role on the project policy needs cloud-platform, which is
	// each API's default scope, and a caller-supplied token carries whatever it was minted with.
	opts := func() []option.ClientOption {
		return append(append([]option.ClientOption{}, authOpts...), option.WithRequestReason("pulumi-oidc-setup"))
	}

	// look up the project number
	crmService, err := cloudresourcemanager.NewService(ctx, opts()...)
	if err != nil {
		return nil, cloudsetup.NewSetupError(cloudsetup.ErrorCodeInvalidCredentials, "failed to create CRM service", err)
	}

	iamService, err := iam.NewService(ctx, opts()...)
	if err != nil {
		return nil, cloudsetup.NewSetupError(cloudsetup.ErrorCodeInvalidCredentials, "failed to create IAM service", err)
	}

	serviceUsageService, err := serviceusage.NewService(ctx, opts()...)
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
		retryDelay:         5 * time.Second,
	}, nil
}

// projectNumberFromName extracts the numeric project number from a CRM v3 Project's
// resource name, which has the form "projects/<number>". Returning an error rather than a
// zero value prevents silent misconfiguration: the project number flows into IAM principal
// strings (`principalSet://.../projects/<num>/...`) and into `CloudAccount.Number`, so
// failing loudly is the correct behavior if the API ever returns an unexpected shape.
func projectNumberFromName(name string) (int64, error) {
	digits, ok := strings.CutPrefix(name, "projects/")
	if !ok {
		return 0, fmt.Errorf("unexpected project resource name %q: want \"projects/<number>\"", name)
	}
	num, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || num <= 0 {
		return 0, fmt.Errorf("unexpected project resource name %q: want \"projects/<number>\"", name)
	}
	return num, nil
}

func (c *client) SetupOIDCInfrastructure(
	ctx context.Context, orgName string, orgID string, projectID string, oidcServiceAccountName string, role string,
	escEnvironmentName string,
) (*cloudsetup.CloudSetupResult, error) {
	result := &cloudsetup.CloudSetupResult{
		Success:   false,
		Resources: []cloudsetup.CloudSetupResource{},
	}

	if escEnvironmentName == "" {
		return nil, cloudsetup.NewSetupError(cloudsetup.ErrorCodeSetupFailed,
			"an ESC environment name is required to scope the service account's impersonation binding", nil)
	}

	if !serviceAccountIDRE.MatchString(oidcServiceAccountName) {
		return nil, cloudsetup.NewSetupError(cloudsetup.ErrorCodeSetupFailed, fmt.Sprintf(
			"service account name %q is not a valid GCP service account ID: must be 6-30 characters "+
				"matching %s", oidcServiceAccountName, serviceAccountIDPattern), nil)
	}

	// Lookup project number
	project, err := c.crmClient.GetProject(ctx, projectID)
	if err != nil {
		return nil, cloudsetup.NewSetupError(cloudsetup.ErrorCodeInvalidCredentials, "failed to lookup project", err)
	}
	projectNumber, err := projectNumberFromName(project.Name)
	if err != nil {
		return nil, cloudsetup.NewSetupError(cloudsetup.ErrorCodeInvalidCredentials, "failed to parse project number", err)
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
	providerID := oidcProviderID(orgID)
	audience, subjectEnvName := cloudsetup.OIDCClaims("gcp", orgName, escEnvironmentName)

	provider := &iam.WorkloadIdentityPoolProvider{
		DisplayName: providerID,
		Description: "Allows Pulumi ESC to assume roles via OIDC for organization " + orgName,
		Oidc: &iam.Oidc{
			IssuerUri:        c.oidcIssuer,
			AllowedAudiences: []string{audience},
		},
		AttributeMapping: map[string]string{
			"google.subject":     "assertion.sub",
			"attribute.oidc_aud": "assertion.aud",
		},
	}

	providerStatus := cloudsetup.ResourceStatusCreated
	err = c.iamClient.CreateWorkloadIdentityProvider(ctx, project.ProjectId, poolID, providerID, provider)
	if isAlreadyExistsError(err) {
		providerStatus, err = c.reconcileWorkloadIdentityProvider(ctx, project.ProjectId, poolID, providerID, provider)
	}
	if err != nil {
		return cloudsetup.WrapSetupError(result, ResourceTypeGCPWorkloadIdentityProvider, err)
	}

	result.Resources = append(result.Resources, cloudsetup.CloudSetupResource{
		Type:   ResourceTypeGCPWorkloadIdentityProvider,
		ID:     providerID,
		Name:   "PulumiOIDCProvider",
		Status: providerStatus,
	})

	// Create Service Account
	saID := oidcServiceAccountName
	saDisplayName := oidcServiceAccountDisplayName(orgName)
	saEmail := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", saID, project.ProjectId)
	saName := fmt.Sprintf("projects/%s/serviceAccounts/%s", project.ProjectId, saEmail)

	serviceAccount := &iam.CreateServiceAccountRequest{
		AccountId: saID,
		ServiceAccount: &iam.ServiceAccount{
			DisplayName: saDisplayName,
			Description: oidcServiceAccountDescription(orgName, escEnvironmentName),
		},
	}

	_, err = c.iamClient.CreateServiceAccount(ctx, project.ProjectId, serviceAccount)
	saExisted := isAlreadyExistsError(err)
	switch {
	case saExisted:
		if err := c.verifyServiceAccountName(ctx, saName, saDisplayName); err != nil {
			return cloudsetup.WrapSetupError(result, ResourceTypeGCPServiceAccount, err)
		}
	case err != nil:
		return cloudsetup.WrapSetupError(result, ResourceTypeGCPServiceAccount, err)
	}

	result.Resources = append(result.Resources, cloudsetup.CloudSetupResource{
		Type:   ResourceTypeGCPServiceAccount,
		ID:     saEmail,
		Name:   saID,
		Status: status(saExisted),
	})

	// Update IAM policy for the Service Account
	policy, err := c.iamClient.GetServiceAccountPolicy(ctx, saName)
	if err != nil {
		return cloudsetup.WrapSetupError(result, ResourceTypeGCPIAMBinding, err)
	}

	workloadIdentityUser := &iam.Binding{
		Role:    "roles/iam.workloadIdentityUser",
		Members: []string{workloadIdentityMember(projectNumber, poolID, orgName, subjectEnvName)},
	}

	wiStatus := cloudsetup.ResourceStatusExisting
	if !bindingExists(policy.Bindings, workloadIdentityUser) {
		wiStatus = cloudsetup.ResourceStatusCreated
		policy.Bindings = append(policy.Bindings, workloadIdentityUser)

		setPolicy := func() error {
			_, err := c.iamClient.SetServiceAccountPolicy(ctx, saName, policy)
			return err
		}

		err = cloudsetup.RunWithRetries(ctx, c.maxRetryAttempts, c.retryDelay, setPolicy)
		if err != nil {
			return cloudsetup.WrapSetupError(result, ResourceTypeGCPIAMBinding, err)
		}
	}

	result.Resources = append(result.Resources, cloudsetup.CloudSetupResource{
		Type:   ResourceTypeGCPIAMBinding,
		ID:     saEmail + "/roles/iam.workloadIdentityUser",
		Name:   "WorkloadIdentityBinding",
		Status: wiStatus,
	})

	rbStatus, err := c.bindProjectRole(ctx, project.ProjectId, role, "serviceAccount:"+saEmail)
	if err != nil {
		return cloudsetup.WrapSetupError(result, ResourceTypeGCPIAMBinding, err)
	}

	result.Resources = append(result.Resources, cloudsetup.CloudSetupResource{
		Type:   ResourceTypeGCPIAMBinding,
		ID:     fmt.Sprintf("%s/%s", saEmail, role),
		Name:   "RoleBinding",
		Status: rbStatus,
	})

	result.Success = true
	return result, nil
}

// GCP's rule for a service account ID.
const serviceAccountIDPattern = `^[a-z][-a-z0-9]{4,28}[a-z0-9]$`

var serviceAccountIDRE = regexp.MustCompile(serviceAccountIDPattern)

// GCP caps a service account display name at 100 UTF-8 bytes. Org logins are ASCII and capped at
// 80 characters, so the fixed part must stay short enough to always fit.
func oidcServiceAccountDisplayName(orgName string) string {
	return fmt.Sprintf("Pulumi OIDC (%s)", orgName)
}

func oidcServiceAccountDescription(orgName, escEnvironmentName string) string {
	return fmt.Sprintf("Pulumi ESC OIDC for organization %s, environment %s", orgName, escEnvironmentName)
}

// IAM principals reference the project by number, not by project ID.
func workloadIdentityMember(projectNumber int64, poolID, orgName, subjectEnvName string) string {
	return fmt.Sprintf("principal://iam.googleapis.com/projects/%d/locations/global/workloadIdentityPools/%s/subject/%s",
		projectNumber, poolID, oidcSubject(orgName, subjectEnvName))
}

func oidcSubject(orgName, subjectEnvName string) string {
	return fmt.Sprintf("pulumi:environments:pulumi.organization.login:%s:currentEnvironment.name:%s",
		orgName, subjectEnvName)
}

// verifyServiceAccountName checks that an existing service account carries the display name
// Pulumi gives this organization's account.
func (c *client) verifyServiceAccountName(ctx context.Context, saName, wantDisplayName string) error {
	existing, err := c.iamClient.GetServiceAccount(ctx, saName)
	if err != nil {
		return fmt.Errorf("service account %q already exists, but reading it back failed: %w", saName, err)
	}
	if existing.DisplayName != wantDisplayName {
		return fmt.Errorf("service account %q already exists with display name %q, but Pulumi OIDC setup for "+
			"this organization expects %q. Refusing to grant it impersonation access — delete or rename the "+
			"existing account and retry", saName, existing.DisplayName, wantDisplayName)
	}
	return nil
}

// bindProjectRole grants role to member on the project's IAM policy. Each attempt re-reads the
// policy because SetIamPolicy is a compare-and-swap on the etag, and retries cover a new service
// account not yet visible to CRM.
func (c *client) bindProjectRole(ctx context.Context, projectID, role, member string) (string, error) {
	var lastErr error
	for attempt := range c.maxRetryAttempts {
		policy, err := c.crmClient.GetProjectPolicy(ctx, projectID)
		if err != nil {
			return "", explainProjectPolicyError(projectID, role, err)
		}

		if projectBindingExists(policy.Bindings, role, member) {
			return cloudsetup.ResourceStatusExisting, nil
		}
		policy.Bindings = append(policy.Bindings, &cloudresourcemanager.Binding{
			Role:    role,
			Members: []string{member},
		})

		_, err = c.crmClient.SetProjectPolicy(ctx, projectID, policy)
		if err == nil {
			return cloudsetup.ResourceStatusCreated, nil
		}
		lastErr = err
		if isPermissionDeniedError(err) {
			break
		}

		if attempt < c.maxRetryAttempts-1 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(c.retryDelay):
			}
		}
	}
	return "", explainProjectPolicyError(projectID, role, lastErr)
}

func explainProjectPolicyError(projectID, role string, err error) error {
	if isPermissionDeniedError(err) {
		return fmt.Errorf("granting %s on project %s was denied. Granting a project role requires the "+
			"resourcemanager.projects.setIamPolicy permission (roles/resourcemanager.projectIamAdmin or "+
			"roles/owner) on the project, and requires re-authorizing Pulumi so the Google session carries "+
			"the cloud-platform scope. The OIDC resources were created, but the role was not granted: %w",
			role, projectID, err)
	}
	return fmt.Errorf("granting %s on project %s failed: %w", role, projectID, err)
}

func isPermissionDeniedError(err error) bool {
	apiErr, ok := errors.AsType[*googleapi.Error](err)
	if !ok {
		return false
	}
	return apiErr.Code == http.StatusUnauthorized || apiErr.Code == http.StatusForbidden
}

func projectBindingExists(bindings []*cloudresourcemanager.Binding, role, member string) bool {
	for _, b := range bindings {
		if b.Role == role && slices.Contains(b.Members, member) {
			return true
		}
	}
	return false
}

// reconcileWorkloadIdentityProvider handles a create conflict for the workload identity
// provider. A pre-existing provider is not necessarily usable: the pool and provider IDs
// don't encode which Pulumi backend created them, so an onboarding run against a different
// backend (e.g. staging vs. production, or self-hosted vs. SaaS) leaves a provider that
// trusts a different OIDC issuer, and tokens issued by this backend can never authenticate.
// The existing provider's OIDC configuration is read back and, if it doesn't match the
// desired one, patched in place. Returns the resource status to report.
func (c *client) reconcileWorkloadIdentityProvider(
	ctx context.Context, projectID, poolID, providerID string, desired *iam.WorkloadIdentityPoolProvider,
) (string, error) {
	existing, err := c.iamClient.GetWorkloadIdentityProvider(ctx, projectID, poolID, providerID)
	if err != nil {
		return "", fmt.Errorf("workload identity provider %q already exists, but reading its configuration back failed: %w",
			providerID, err)
	}

	// GCP soft-deletes providers for 30 days and rejects creates with the same ID during
	// that window, so a create conflict can also mean the provider is pending deletion. It
	// can't be patched in that state; it has to be restored first.
	if existing.State == "DELETED" {
		return "", fmt.Errorf("workload identity provider %q is scheduled for deletion in GCP; restore it "+
			"(gcloud iam workload-identity-pools providers undelete %s --workload-identity-pool=%s "+
			"--location=global --project=%s) and retry", providerID, providerID, poolID, projectID)
	}

	if oidcConfigMatches(existing.Oidc, desired.Oidc) {
		return cloudsetup.ResourceStatusExisting, nil
	}

	// Patch only the OIDC block so the provider trusts tokens issued by this backend;
	// everything else about the provider is left untouched. Audiences already allowed on
	// the provider are preserved: distinct org names can sanitize to the same provider ID,
	// so replacing the list outright could revoke another org's working access.
	patch := &iam.WorkloadIdentityPoolProvider{
		Oidc: &iam.Oidc{
			IssuerUri:        desired.Oidc.IssuerUri,
			AllowedAudiences: mergedAllowedAudiences(existing.Oidc, desired.Oidc),
		},
	}
	if err := c.iamClient.UpdateWorkloadIdentityProvider(ctx, projectID, poolID, providerID, patch, "oidc"); err != nil {
		var existingIssuer string
		if existing.Oidc != nil {
			existingIssuer = existing.Oidc.IssuerUri
		}
		return "", fmt.Errorf("workload identity provider %q already exists with a different OIDC configuration "+
			"(existing issuer %q), and updating it failed: %w", providerID, existingIssuer, err)
	}
	return cloudsetup.ResourceStatusUpdated, nil
}

// mergedAllowedAudiences returns the existing provider's allowed audiences plus any
// desired ones not already present.
func mergedAllowedAudiences(existing, desired *iam.Oidc) []string {
	var merged []string
	if existing != nil {
		merged = slices.Clone(existing.AllowedAudiences)
	}
	for _, aud := range desired.AllowedAudiences {
		if !slices.Contains(merged, aud) {
			merged = append(merged, aud)
		}
	}
	return merged
}

// oidcConfigMatches reports whether an existing provider's OIDC configuration already
// trusts tokens issued by this backend: the issuer must match exactly and every desired
// audience must be allowed. Extra allowed audiences on the existing provider are fine.
func oidcConfigMatches(existing, desired *iam.Oidc) bool {
	if existing == nil || existing.IssuerUri != desired.IssuerUri {
		return false
	}
	for _, aud := range desired.AllowedAudiences {
		if !slices.Contains(existing.AllowedAudiences, aud) {
			return false
		}
	}
	return true
}

// GCP caps a workload identity provider ID at 32 characters, so the organization is named
// by an ID prefix and spelled out in the provider's description instead.
const orgIDPrefixLen = 8

func oidcProviderID(orgID string) string {
	return "pulumi-" + orgID[:min(len(orgID), orgIDPrefixLen)]
}

// Helper function to check if error is "already exists"
func isAlreadyExistsError(err error) bool {
	// Check for Google API error with 409 status code (conflict)
	if apiErr, ok := errors.AsType[*googleapi.Error](err); ok {
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

func (c *client) ListAccounts(ctx context.Context, gcpOrganizationID string) ([]cloudsetup.CloudAccount, error) {
	projects, err := c.crmClient.ListProjects(ctx, gcpOrganizationID)
	if err != nil {
		return nil, err
	}

	accounts := make([]cloudsetup.CloudAccount, 0, len(projects))
	for _, project := range projects {
		number, err := projectNumberFromName(project.Name)
		if err != nil {
			return nil, fmt.Errorf("listing GCP projects: %w", err)
		}
		// CRM v3 allows an empty DisplayName (v1's Name was always set); fall back
		// to the project ID so the picker never shows a blank row.
		displayName := project.DisplayName
		if displayName == "" {
			displayName = project.ProjectId
		}
		accounts = append(accounts, cloudsetup.CloudAccount{
			ID:     project.ProjectId,
			Name:   displayName,
			Number: number,
		})
	}
	return accounts, nil
}
