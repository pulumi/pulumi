package backend

import backend "github.com/pulumi/pulumi/sdk/v3/pkg/backend"

// GetDefaultOrg returns a user's default organization, if configured.
// It will prefer the organization that the user has configured locally, falling back to making an API
// call to the backend for the backend opinion on default organization if not manually set by the user.
// Returns an empty string if the user does not have a default org explicitly configured and if the backend
// does not have an opinion on user organizations.
func GetDefaultOrg(ctx context.Context, b Backend, currentProject *workspace.Project) (string, error) {
	return backend.GetDefaultOrg(ctx, b, currentProject)
}

// GetLegacyDefaultOrgFallback returns the current user name as an org, if the user does not have
// a default org locally configured. Returns empty string otherwise, or if the backend does not support
// organizations.
// 
// IMPORTANT NOTE: This function does not return a user's default org; callers should use `GetDefaultOrg`
// instead. `GetLegacyDefaultOrgFallback` emulates legacy fall back behavior, if a default org is not set.
// 
// We preserve parts of this behavior in the interest of backwards compatibility, for users who are migrating
// from older versions of the Pulumi CLI that did not always store the current selected stack with a fully qualified
// stack name. For this class of existing users, we want to ensure that we are selecting the correct organization
// as their CLI is brought up-to-date.
func GetLegacyDefaultOrgFallback(b Backend, currentProject *workspace.Project) (string, error) {
	return backend.GetLegacyDefaultOrgFallback(b, currentProject)
}

