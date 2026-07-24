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

	"google.golang.org/api/cloudresourcemanager/v1"
)

// crmClient is a thin wrapper around the GCP CRM API.
type crmClient interface {
	GetProject(ctx context.Context, projectID string) (*cloudresourcemanager.Project, error)
	ListProjects(ctx context.Context) ([]*cloudresourcemanager.Project, error)
}

type realCRMClient struct {
	crm *cloudresourcemanager.Service
}

func (c *realCRMClient) GetProject(ctx context.Context, projectID string) (*cloudresourcemanager.Project, error) {
	return c.crm.Projects.Get(projectID).Context(ctx).Do()
}

func (c *realCRMClient) ListProjects(ctx context.Context) ([]*cloudresourcemanager.Project, error) {
	var projects []*cloudresourcemanager.Project
	err := c.crm.Projects.List().Pages(ctx, func(response *cloudresourcemanager.ListProjectsResponse) error {
		projects = append(projects, response.Projects...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return projects, nil
}
