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

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// orgMemberGetRenderFunc renders a single organization member.
type orgMemberGetRenderFunc func(w io.Writer, member apitype.OrganizationMember) error

func defaultOrgMemberGetOutputFormat() outputflag.OutputFlag[orgMemberGetRenderFunc] {
	return outputflag.OutputFlag[orgMemberGetRenderFunc]{
		RenderForTerminal: renderOrgMemberGetText,
		RenderJSON:        renderOrgMemberGetJSON,
	}
}

func renderOrgMemberGetText(w io.Writer, member apitype.OrganizationMember) error {
	name := member.User.Name
	if name == "" {
		name = "-"
	}
	fmt.Fprintf(w, "%-16s %s\n", "User name:", name)
	fmt.Fprintf(w, "%-16s %s\n", "GitHub login:", member.User.GitHubLogin)
	if member.Role != "" {
		fmt.Fprintf(w, "%-16s %s\n", "Role:", member.Role)
	}
	if member.FGARole.Name != "" {
		fmt.Fprintf(w, "%-16s %s\n", "FGA role:", member.FGARole.Name)
	}
	if member.FGARole.ID != "" {
		fmt.Fprintf(w, "%-16s %s\n", "FGA role ID:", member.FGARole.ID)
	}
	if member.Created != "" {
		fmt.Fprintf(w, "%-16s %s\n", "Joined at:", member.Created)
	}
	return nil
}

type orgMemberGetJSON struct {
	Role          string               `json:"role"`
	User          apitype.UserInfo     `json:"user"`
	Created       string               `json:"created"`
	KnownToPulumi bool                 `json:"knownToPulumi"`
	VirtualAdmin  bool                 `json:"virtualAdmin"`
	Links         *apitype.MemberLinks `json:"links,omitempty"`
	FGARole       apitype.FGARole      `json:"fgaRole"`
}

func toOrgMemberGetJSON(member apitype.OrganizationMember) orgMemberGetJSON {
	return orgMemberGetJSON{
		Role:          member.Role,
		User:          member.User,
		Created:       member.Created,
		KnownToPulumi: member.KnownToPulumi,
		VirtualAdmin:  member.VirtualAdmin,
		Links:         member.Links,
		FGARole:       member.FGARole,
	}
}

func renderOrgMemberGetJSON(w io.Writer, member apitype.OrganizationMember) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(toOrgMemberGetJSON(member))
}

// orgMemberLister is the subset of the client needed by findOrganizationMember.
type orgMemberLister interface {
	ListOrganizationMembers(
		ctx context.Context, orgName, mode string, continuationToken *string,
	) (apitype.ListOrganizationMembersResponse, error)
}

// findOrganizationMember pages through organization members to find one by login.
func findOrganizationMember(
	ctx context.Context, c orgMemberLister, org, userLogin string,
) (apitype.OrganizationMember, error) {
	wantLogin := strings.ToLower(userLogin)
	for _, mode := range []string{"frontend", "backend"} {
		var continuationToken *string
		for {
			resp, err := c.ListOrganizationMembers(ctx, org, mode, continuationToken)
			if err != nil {
				return apitype.OrganizationMember{}, fmt.Errorf("getting organization member: %w", err)
			}
			for _, m := range resp.Members {
				if strings.EqualFold(m.User.GitHubLogin, wantLogin) {
					return m, nil
				}
			}
			if resp.ContinuationToken == "" {
				break
			}
			next := resp.ContinuationToken
			continuationToken = &next
		}
	}
	return apitype.OrganizationMember{}, fmt.Errorf("organization member %q not found in %s", userLogin, org)
}
