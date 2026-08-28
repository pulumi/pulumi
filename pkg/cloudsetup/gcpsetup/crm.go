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

package gcpsetup

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/pgavlin/fx/v2"
	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v3"
	"google.golang.org/api/googleapi"
)

// crmClient is a thin wrapper around the GCP CRM API.
type crmClient interface {
	GetProject(ctx context.Context, projectID string) (*cloudresourcemanager.Project, error)
	// ListProjects returns projects for onboarding discovery. When gcpOrganizationID is
	// non-empty, results are scoped to that organization — both projects directly under
	// it and projects nested under its descendant folders — so users in large GCP
	// estates aren't shown every project the OAuth principal can see across every
	// organization they belong to. When gcpOrganizationID is empty, all projects the
	// principal can access are returned (users without a GCP organization have no ID to
	// scope by). In both cases, projects that aren't ACTIVE and system-generated
	// projects (IDs prefixed "sys-", e.g. Apps Script backing projects, which the GCP
	// console itself hides) are excluded.
	ListProjects(ctx context.Context, gcpOrganizationID string) ([]*cloudresourcemanager.Project, error)
	GetProjectPolicy(ctx context.Context, projectID string) (*cloudresourcemanager.Policy, error)
	SetProjectPolicy(
		ctx context.Context, projectID string, policy *cloudresourcemanager.Policy,
	) (*cloudresourcemanager.Policy, error)
}

type realCRMClient struct {
	crm *cloudresourcemanager.Service
}

func (c *realCRMClient) GetProject(ctx context.Context, projectID string) (*cloudresourcemanager.Project, error) {
	return c.crm.Projects.Get("projects/" + projectID).Context(ctx).Do()
}

func (c *realCRMClient) GetProjectPolicy(ctx context.Context, projectID string) (*cloudresourcemanager.Policy, error) {
	return c.crm.Projects.
		GetIamPolicy("projects/"+projectID, &cloudresourcemanager.GetIamPolicyRequest{}).
		Context(ctx).Do()
}

func (c *realCRMClient) SetProjectPolicy(
	ctx context.Context, projectID string, policy *cloudresourcemanager.Policy,
) (*cloudresourcemanager.Policy, error) {
	return c.crm.Projects.
		SetIamPolicy("projects/"+projectID, &cloudresourcemanager.SetIamPolicyRequest{Policy: policy}).
		Context(ctx).Do()
}

func (c *realCRMClient) ListProjects(
	ctx context.Context, gcpOrganizationID string,
) ([]*cloudresourcemanager.Project, error) {
	if gcpOrganizationID == "" {
		return c.searchAllProjects(ctx)
	}

	// CRM v3 Projects.List(parent=...) only returns direct children of the given parent.
	// To include projects nested under folders within the organization, enumerate the org's
	// folder hierarchy and list projects under each parent (org + every descendant folder),
	// then union the results.
	parents, err := c.collectOrgAndDescendantFolders(ctx, gcpOrganizationID)
	if err != nil {
		return nil, err
	}

	seen := fx.Set[string]{}
	var projects []*cloudresourcemanager.Project
	for _, parent := range parents {
		listProjects := c.crm.Projects.List().Parent(parent)
		err := listProjects.Pages(ctx, func(response *cloudresourcemanager.ListProjectsResponse) error {
			for _, p := range response.Projects {
				if seen.Has(p.ProjectId) || !isOnboardableProject(p) {
					continue
				}
				seen.Add(p.ProjectId)
				projects = append(projects, p)
			}
			return nil
		})
		if err != nil {
			if isPermissionOrNotFoundError(err) {
				// The principal can list this parent's folders (otherwise the BFS wouldn't
				// have visited it) but not its projects. Skip and let other parents
				// contribute what they can — same degradation as Folders.List failures.
				continue
			}
			return nil, fmt.Errorf("listing projects under %s: %w", parent, err)
		}
	}
	return projects, nil
}

// searchAllProjects returns every project the OAuth principal can access, regardless of
// organization. CRM v3 Projects.Search covers the same visibility as v1's unscoped
// Projects.List (everything the caller has resourcemanager.projects.get on), which is the
// only option when the user has no GCP organization to scope discovery by.
func (c *realCRMClient) searchAllProjects(ctx context.Context) ([]*cloudresourcemanager.Project, error) {
	var projects []*cloudresourcemanager.Project
	err := c.crm.Projects.Search().Pages(ctx, func(response *cloudresourcemanager.SearchProjectsResponse) error {
		for _, p := range response.Projects {
			if isOnboardableProject(p) {
				projects = append(projects, p)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("searching projects: %w", err)
	}
	return projects, nil
}

// isOnboardableProject filters discovery noise: projects pending deletion can't be
// onboarded, and "sys-" projects are system-generated (e.g. Apps Script backing
// projects) that the GCP console itself hides — surfacing hundreds of them makes the
// account picker unusable and every selection fails OIDC setup anyway.
func isOnboardableProject(p *cloudresourcemanager.Project) bool {
	return p.State == "ACTIVE" && !strings.HasPrefix(p.ProjectId, "sys-")
}

// collectOrgAndDescendantFolders returns the organization plus every folder that is a
// (transitive) descendant of it, as CRM v3 resource names ("organizations/<id>" and
// "folders/<id>"). Folder enumeration uses CRM v3 Folders.List, which requires
// resourcemanager.folders.list on each folder. Subtrees the principal cannot enumerate
// (401/403/404) are silently skipped — the projects beneath them won't be discovered, but
// the rest of the org's hierarchy is still returned, which is the correct degradation for
// a per-user UI.
func (c *realCRMClient) collectOrgAndDescendantFolders(
	ctx context.Context, gcpOrganizationID string,
) ([]string, error) {
	orgParent := "organizations/" + gcpOrganizationID
	parents := []string{orgParent}
	queue := []string{orgParent}
	visited := fx.NewSet(orgParent)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		listFolders := c.crm.Folders.List().Parent(current)
		err := listFolders.Pages(ctx, func(response *cloudresourcemanager.ListFoldersResponse) error {
			for _, folder := range response.Folders {
				if folder.State != "ACTIVE" {
					continue
				}
				if visited.Has(folder.Name) {
					continue
				}
				visited.Add(folder.Name)
				parents = append(parents, folder.Name)
				queue = append(queue, folder.Name)
			}
			return nil
		})
		if err != nil {
			if isPermissionOrNotFoundError(err) {
				continue
			}
			return nil, fmt.Errorf("listing folders under %s: %w", current, err)
		}
	}
	return parents, nil
}

// isPermissionOrNotFoundError reports whether err is a Google API 401/403/404, used to
// gracefully skip folder subtrees the principal cannot enumerate during discovery.
func isPermissionOrNotFoundError(err error) bool {
	var apiErr *googleapi.Error
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.Code {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return true
	}
	return false
}
