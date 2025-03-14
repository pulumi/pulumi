package apitype

// GetDefaultOrganizationResponse returns the backend's opinion of which organization
// to default to for a given user, if a default organization has not been configured.
type GetDefaultOrganizationResponse struct {
	// Returns the Organization.GitHubLogin of the organization.
	// Can be an empty string, if the user is a member of no organizations
	OrganizationName string

	// Messages is a list of messages that should be displayed to the user.
	Messages []Message
}
