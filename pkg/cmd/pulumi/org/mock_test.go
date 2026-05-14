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

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// mockOrgRoleClient is a fake orgRoleClient used by tests across the
// `pulumi org role ...` commands. Tests may set the returned values directly
// and read back the captured request fields after invocation.
type mockOrgRoleClient struct {
	roles           []apitype.Role
	listErr         error
	capturedOrg     string
	capturedPurpose string

	createResp apitype.Role
	createErr  error
	createReq  apitype.CreateRoleRequest
}

func (m *mockOrgRoleClient) ListOrgRoles(_ context.Context, orgName, uxPurpose string) ([]apitype.Role, error) {
	m.capturedOrg = orgName
	m.capturedPurpose = uxPurpose
	return m.roles, m.listErr
}

func (m *mockOrgRoleClient) CreateOrgRole(
	_ context.Context, orgName string, req apitype.CreateRoleRequest,
) (apitype.Role, error) {
	m.capturedOrg = orgName
	m.createReq = req
	return m.createResp, m.createErr
}

func stubRoleFactory(c orgRoleClient, orgName string) orgRoleClientFactory {
	return func(_ context.Context, _ string) (orgRoleClient, string, error) {
		return c, orgName, nil
	}
}

func failingRoleFactory(err error) orgRoleClientFactory {
	return func(_ context.Context, _ string) (orgRoleClient, string, error) {
		return nil, "", err
	}
}
