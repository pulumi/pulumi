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
	"fmt"
	"time"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/iam/v1"
)

// iamClient is a thin wrapper around the GCP IAM API.
// it provides a synchronous interface for creating the oidc resources we need to avoid race conditions during setup,
// and enables easier mocking for tests by papering over the concrete fluent interface.
type iamClient interface {
	CreateWorkloadIdentityPool(ctx context.Context, projectID, poolID string, pool *iam.WorkloadIdentityPool) error
	CreateWorkloadIdentityProvider(
		ctx context.Context, projectID, poolID, providerID string, provider *iam.WorkloadIdentityPoolProvider,
	) error
	CreateServiceAccount(
		ctx context.Context, projectID string, req *iam.CreateServiceAccountRequest,
	) (*iam.ServiceAccount, error)
	GetServiceAccountPolicy(ctx context.Context, saResource string) (*iam.Policy, error)
	SetServiceAccountPolicy(ctx context.Context, saResource string, policy *iam.Policy) (*iam.Policy, error)
}

type realIAMClient struct {
	iam *iam.Service
}

func (c *realIAMClient) CreateWorkloadIdentityPool(
	ctx context.Context, projectID, poolID string, pool *iam.WorkloadIdentityPool,
) error {
	parent := fmt.Sprintf("projects/%s/locations/global", projectID)
	op, err := c.iam.Projects.Locations.WorkloadIdentityPools.
		Create(parent, pool).
		WorkloadIdentityPoolId(poolID).
		Context(ctx).Do()
	if err != nil {
		return err
	}
	return c.await(ctx, iam.NewProjectsLocationsWorkloadIdentityPoolsOperationsService(c.iam).Get(op.Name).Context(ctx))
}

func (c *realIAMClient) CreateWorkloadIdentityProvider(
	ctx context.Context, projectID, poolID, providerID string, provider *iam.WorkloadIdentityPoolProvider,
) error {
	parent := fmt.Sprintf("projects/%s/locations/global/workloadIdentityPools/%s", projectID, poolID)
	op, err := c.iam.Projects.Locations.WorkloadIdentityPools.Providers.
		Create(parent, provider).
		WorkloadIdentityPoolProviderId(providerID).
		Context(ctx).Do()
	if err != nil {
		return err
	}
	return c.await(ctx, iam.NewProjectsLocationsWorkloadIdentityPoolsOperationsService(c.iam).Get(op.Name).Context(ctx))
}

func (c *realIAMClient) CreateServiceAccount(
	ctx context.Context, projectID string, req *iam.CreateServiceAccountRequest,
) (*iam.ServiceAccount, error) {
	parent := "projects/" + projectID
	return c.iam.Projects.ServiceAccounts.
		Create(parent, req).
		Context(ctx).Do()
}

func (c *realIAMClient) GetServiceAccountPolicy(ctx context.Context, saResource string) (*iam.Policy, error) {
	return c.iam.Projects.ServiceAccounts.
		GetIamPolicy(saResource).
		Context(ctx).Do()
}

func (c *realIAMClient) SetServiceAccountPolicy(
	ctx context.Context, saResource string, policy *iam.Policy,
) (*iam.Policy, error) {
	return c.iam.Projects.ServiceAccounts.
		SetIamPolicy(saResource, &iam.SetIamPolicyRequest{Policy: policy}).
		Context(ctx).Do()
}

// await polls the given operation until it is done or the context is canceled.
// see https://pkg.go.dev/google.golang.org/api#hdr-Polling_Operations
func (c *realIAMClient) await(ctx context.Context, pollOperation interface {
	Do(opts ...googleapi.CallOption) (*iam.Operation, error)
},
) error {
	for {
		op, err := pollOperation.Do()
		if err != nil {
			return err
		}
		if op.Done {
			if op.Error != nil {
				return fmt.Errorf("operation failed: %s", op.Error.Message)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
