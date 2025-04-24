// Copyright 2016-2025, Pulumi Corporation.
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
