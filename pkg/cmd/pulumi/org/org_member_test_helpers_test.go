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

package org

import "github.com/pulumi/pulumi/sdk/v3/go/common/apitype"

// Shared test helpers used by org member edit tests.

type orgMemberGetCall struct {
	org               string
	mode              string
	continuationToken *string
}

type orgMemberGetPage struct {
	resp apitype.ListOrganizationMembersResponse
	err  error
}

func aliceMember() apitype.OrganizationMember {
	return apitype.OrganizationMember{
		Role: "admin",
		User: apitype.UserInfo{
			Name:        "Alice Example",
			GitHubLogin: "alice",
			AvatarURL:   "https://example.com/alice.png",
		},
		Created:       "2025-01-02T03:04:05Z",
		KnownToPulumi: true,
		FGARole: apitype.FGARole{
			ID:         "admin",
			Name:       "Admin",
			ModifiedAt: "2025-01-02T03:04:05Z",
		},
	}
}
