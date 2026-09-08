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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/iam/v1"

	cloudsetup "github.com/pulumi/pulumi/pkg/v3/cloudsetup/common"
)

func TestOIDCConfigMatches(t *testing.T) {
	t.Parallel()

	desired := &iam.Oidc{
		IssuerUri:        "https://fake-issuer.tld/oidc",
		AllowedAudiences: []string{"gcp:test-org"},
	}

	t.Run("matching issuer and audience", func(t *testing.T) {
		t.Parallel()
		assert.True(t, oidcConfigMatches(&iam.Oidc{
			IssuerUri:        "https://fake-issuer.tld/oidc",
			AllowedAudiences: []string{"gcp:test-org"},
		}, desired))
	})

	t.Run("extra allowed audiences are fine", func(t *testing.T) {
		t.Parallel()
		assert.True(t, oidcConfigMatches(&iam.Oidc{
			IssuerUri:        "https://fake-issuer.tld/oidc",
			AllowedAudiences: []string{"gcp:other-org", "gcp:test-org"},
		}, desired))
	})

	t.Run("mismatched issuer", func(t *testing.T) {
		t.Parallel()
		assert.False(t, oidcConfigMatches(&iam.Oidc{
			IssuerUri:        "https://other-backend.tld/oidc",
			AllowedAudiences: []string{"gcp:test-org"},
		}, desired))
	})

	t.Run("missing audience", func(t *testing.T) {
		t.Parallel()
		assert.False(t, oidcConfigMatches(&iam.Oidc{
			IssuerUri:        "https://fake-issuer.tld/oidc",
			AllowedAudiences: []string{"gcp:other-org"},
		}, desired))
	})

	t.Run("no oidc config on existing provider", func(t *testing.T) {
		t.Parallel()
		assert.False(t, oidcConfigMatches(nil, desired))
	})
}

// fakeIAMClient stubs the two calls reconcileWorkloadIdentityProvider makes. The embedded
// interface is nil, so any other call panics rather than passing silently.
type fakeIAMClient struct {
	iamClient

	getProvider *iam.WorkloadIdentityPoolProvider
	getErr      error

	updateErr    error
	updateCalls  int
	updatedPatch *iam.WorkloadIdentityPoolProvider
	updatedMask  string
}

func (f *fakeIAMClient) GetWorkloadIdentityProvider(
	context.Context, string, string, string,
) (*iam.WorkloadIdentityPoolProvider, error) {
	return f.getProvider, f.getErr
}

func (f *fakeIAMClient) UpdateWorkloadIdentityProvider(
	_ context.Context, _, _, _ string, provider *iam.WorkloadIdentityPoolProvider, updateMask string,
) error {
	f.updateCalls++
	f.updatedPatch = provider
	f.updatedMask = updateMask
	return f.updateErr
}

func TestReconcileWorkloadIdentityProvider(t *testing.T) {
	t.Parallel()

	desired := &iam.WorkloadIdentityPoolProvider{
		Oidc: &iam.Oidc{
			IssuerUri:        "https://fake-issuer.tld/oidc",
			AllowedAudiences: []string{"gcp:test-org"},
		},
	}
	reconcile := func(t *testing.T, fake *fakeIAMClient) (string, error) {
		t.Helper()
		c := &client{iamClient: fake}
		return c.reconcileWorkloadIdentityProvider(
			t.Context(), "test-project", "pulumi-cloud", "pulumi-abcd1234", desired)
	}

	t.Run("matching config is reported as existing without a patch", func(t *testing.T) {
		t.Parallel()
		fake := &fakeIAMClient{getProvider: &iam.WorkloadIdentityPoolProvider{Oidc: desired.Oidc}}

		status, err := reconcile(t, fake)
		require.NoError(t, err)
		assert.Equal(t, cloudsetup.ResourceStatusExisting, status)
		assert.Zero(t, fake.updateCalls)
	})

	t.Run("mismatched issuer is patched and audiences are merged", func(t *testing.T) {
		t.Parallel()
		fake := &fakeIAMClient{getProvider: &iam.WorkloadIdentityPoolProvider{
			Oidc: &iam.Oidc{
				IssuerUri:        "https://other-backend.tld/oidc",
				AllowedAudiences: []string{"gcp:other-org"},
			},
		}}

		status, err := reconcile(t, fake)
		require.NoError(t, err)
		assert.Equal(t, cloudsetup.ResourceStatusUpdated, status)
		assert.Equal(t, "oidc", fake.updatedMask)
		assert.Equal(t, "https://fake-issuer.tld/oidc", fake.updatedPatch.Oidc.IssuerUri)
		// The other org's audience is kept, so reconciling does not revoke its access.
		assert.Equal(t, []string{"gcp:other-org", "gcp:test-org"}, fake.updatedPatch.Oidc.AllowedAudiences)
	})

	t.Run("a provider pending GCP's soft deletion cannot be patched", func(t *testing.T) {
		t.Parallel()
		fake := &fakeIAMClient{getProvider: &iam.WorkloadIdentityPoolProvider{State: "DELETED"}}

		_, err := reconcile(t, fake)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "scheduled for deletion")
		assert.Contains(t, err.Error(),
			"gcloud iam workload-identity-pools providers undelete pulumi-abcd1234 "+
				"--workload-identity-pool=pulumi-cloud --location=global --project=test-project")
		assert.Zero(t, fake.updateCalls)
	})

	t.Run("a failed read-back is reported rather than assumed benign", func(t *testing.T) {
		t.Parallel()
		fake := &fakeIAMClient{getErr: errors.New("boom")}

		_, err := reconcile(t, fake)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reading its configuration back failed")
	})

	t.Run("a failed update names the existing issuer", func(t *testing.T) {
		t.Parallel()
		fake := &fakeIAMClient{
			getProvider: &iam.WorkloadIdentityPoolProvider{
				Oidc: &iam.Oidc{IssuerUri: "https://other-backend.tld/oidc"},
			},
			updateErr: errors.New("boom"),
		}

		_, err := reconcile(t, fake)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `existing issuer "https://other-backend.tld/oidc"`)
	})
}
