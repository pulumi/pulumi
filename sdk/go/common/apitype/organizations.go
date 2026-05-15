// Copyright 2016, Pulumi Corporation.
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

package apitype

// GetDefaultOrganizationResponse returns the backend's opinion of which organization
// to default to for a given user, if a default organization has not been configured.
type GetDefaultOrganizationResponse struct {
	// Returns the organization name.
	// Can be an empty string, if the user is a member of no organizations
	GitHubLogin string

	// Messages is a list of messages that should be displayed to the user that contextualize
	// the default org; for example: warning new users if their default org as returned by the
	// service is on an expiring trial and not free tier, with possible recommendations
	// on how to configure their default org locally.
	// Can be possibly empty.
	Messages []Message
}

// FGARole describes the role currently assigned to an organization member.
// It is either a built-in role (member, admin, billingManager) or a custom
// role defined in the organization.
type FGARole struct {
	// ID is the role identifier; for built-in roles this is the role name,
	// for custom roles it is an opaque identifier.
	ID string `json:"id"`
	// Name is the human-readable role name.
	Name string `json:"name"`
	// ModifiedAt is when the role was last modified.
	ModifiedAt string `json:"modifiedAt"`
}

// MemberLinks contains links to the member's page in the Pulumi Console.
type MemberLinks struct {
	// Self is the URL of the member's page in the Pulumi Console.
	Self string `json:"self,omitempty"`
}

// OrganizationMember describes a single member of a Pulumi organization,
// as returned by the ListOrganizationMembers Pulumi Cloud REST endpoint
// (GET /api/orgs/{orgName}/members).
type OrganizationMember struct {
	// Role is the member's built-in role within the organization. Deprecated
	// by the service in favour of FGARole; retained because the list endpoint
	// continues to return it and tooling may want to display it.
	Role string `json:"role"`
	// User is the underlying user record. Email is typically empty on this
	// endpoint.
	User UserInfo `json:"user"`
	// Created is when the member joined the organization.
	Created string `json:"created"`
	// KnownToPulumi indicates whether the organization member has a Pulumi
	// account.
	KnownToPulumi bool `json:"knownToPulumi"`
	// VirtualAdmin indicates that the member does not have admin access on
	// the backing identity provider, but does have admin access to the
	// Pulumi organization.
	VirtualAdmin bool `json:"virtualAdmin"`
	// Links optionally contains URLs to the member in the Pulumi Console.
	Links *MemberLinks `json:"links,omitempty"`
	// FGARole is the role currently assigned to this member — either a
	// built-in role or a custom role.
	FGARole FGARole `json:"fgaRole"`
}

// ListOrganizationMembersResponse is the response body returned by the
// ListOrganizationMembers Pulumi Cloud REST endpoint.
type ListOrganizationMembersResponse struct {
	// Members is the page of organization members.
	Members []OrganizationMember `json:"members"`
	// ContinuationToken is an opaque token for fetching the next page of
	// members; empty when there are no more pages.
	ContinuationToken string `json:"continuationToken,omitempty"`
}

// AuditLogEvent describes a single entry in an organization's audit log, as
// returned by the ListAuditLogEvents Pulumi Cloud REST endpoint
// (GET /api/orgs/{orgName}/auditlogs).
type AuditLogEvent struct {
	// Timestamp is the Unix epoch (in seconds) at which the event occurred.
	Timestamp int64 `json:"timestamp"`
	// Name is the short, machine-readable identifier of the event (e.g.
	// "stack.create").
	Name string `json:"name"`
	// SourceIP is the IP address from which the event originated.
	SourceIP string `json:"sourceIP"`
	// Description is the human-readable description of the event.
	Description string `json:"description"`
	// User is the user that triggered the event.
	User UserInfo `json:"user"`
	// Event is the event-type bucket (e.g. "auth", "stack"). The cloud docs
	// distinguish this from Name.
	Event string `json:"event"`
}

// ListAuditLogEventsResponse is the response body returned by the
// ListAuditLogEvents Pulumi Cloud REST endpoint.
type ListAuditLogEventsResponse struct {
	// AuditLogEvents is the page of audit log events.
	AuditLogEvents []AuditLogEvent `json:"auditLogEvents"`
	// ContinuationToken is an opaque token for fetching the next page of
	// events; empty when there are no more pages.
	ContinuationToken string `json:"continuationToken,omitempty"`
}

// UpdateOrganizationMemberRequest modifies a member's role within an
// organization. It is the body of the `UpdateOrganizationMember` Pulumi Cloud
// REST endpoint (PATCH /api/orgs/{orgName}/members/{userLogin}). Set Role to
// assign a built-in role (`member`, `admin`, or `billingManager`); set
// FgaRoleId to assign a custom role. If both are provided FgaRoleId takes
// precedence on the service.
type UpdateOrganizationMemberRequest struct {
	Role      *string `json:"role,omitempty"`
	FgaRoleId *string `json:"fgaRoleId,omitempty"`
}
