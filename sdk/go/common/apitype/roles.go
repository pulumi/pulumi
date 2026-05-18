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

package apitype

import "encoding/json"

// Role represents a custom role in an organization, as returned by the Pulumi
// Cloud REST API. It mirrors the service's PermissionDescriptorRecord schema.
type Role struct {
	ID                string          `json:"id"`
	OrgID             string          `json:"orgId"`
	Name              string          `json:"name,omitempty"`
	Description       string          `json:"description,omitempty"`
	ResourceType      string          `json:"resourceType,omitempty"`
	UXPurpose         string          `json:"uxPurpose,omitempty"`
	DefaultIdentifier string          `json:"defaultIdentifier,omitempty"`
	Details           json.RawMessage `json:"details,omitempty"`
	Version           int             `json:"version"`
	IsOrgDefault      bool            `json:"isOrgDefault"`
	Created           string          `json:"created"`
	Modified          string          `json:"modified"`
}

// ListRolesResponse is the response body for GET /api/orgs/{orgName}/roles.
type ListRolesResponse struct {
	Roles []Role `json:"roles"`
}

// CreateRoleRequest is the request body for POST /api/orgs/{orgName}/roles.
// It mirrors the service's PermissionDescriptorBase schema.
type CreateRoleRequest struct {
	Name         string          `json:"name,omitempty"`
	Description  string          `json:"description,omitempty"`
	ResourceType string          `json:"resourceType,omitempty"`
	UXPurpose    string          `json:"uxPurpose,omitempty"`
	Details      json.RawMessage `json:"details,omitempty"`
}

// UpdateRoleRequest is the request body for PATCH /api/orgs/{orgName}/roles/{roleID}.
type UpdateRoleRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Details     json.RawMessage `json:"details"`
}
