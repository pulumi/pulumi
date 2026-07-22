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

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	ssooidctypes "github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSOInstanceFromConfig(t *testing.T) {
	t.Parallel()

	t.Run("modern sso-session keys the cached token by session name", func(t *testing.T) {
		t.Parallel()
		inst, ok := ssoInstanceFromConfig(config.SharedConfig{
			SSOSession: &config.SSOSession{
				Name:        "my-sso",
				SSOStartURL: "https://my.awsapps.com/start",
				SSORegion:   "us-east-1",
			},
		})
		require.True(t, ok)
		assert.Equal(t, "my-sso", inst.cacheKey)
		assert.Equal(t, "https://my.awsapps.com/start", inst.startURL)
		assert.Equal(t, "us-east-1", inst.region)
	})

	t.Run("legacy inline config keys the cached token by start URL", func(t *testing.T) {
		t.Parallel()
		inst, ok := ssoInstanceFromConfig(config.SharedConfig{
			SSOStartURL: "https://legacy.awsapps.com/start",
			SSORegion:   "eu-west-1",
		})
		require.True(t, ok)
		assert.Equal(t, "https://legacy.awsapps.com/start", inst.cacheKey)
		assert.Equal(t, "https://legacy.awsapps.com/start", inst.startURL)
		assert.Equal(t, "eu-west-1", inst.region)
	})

	t.Run("no SSO configured reports false", func(t *testing.T) {
		t.Parallel()
		_, ok := ssoInstanceFromConfig(config.SharedConfig{})
		assert.False(t, ok)
	})
}

func TestClassifySSOTokenError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		wantRetry bool
		wantMsg   string
	}{
		{
			name:      "authorization pending keeps polling",
			err:       &ssooidctypes.AuthorizationPendingException{},
			wantRetry: true,
		},
		{
			name:      "slow down keeps polling",
			err:       &ssooidctypes.SlowDownException{},
			wantRetry: true,
		},
		{
			name:      "internal server error keeps polling",
			err:       &ssooidctypes.InternalServerException{},
			wantRetry: true,
		},
		{
			name:      "transport error keeps polling",
			err:       errors.New("connection reset by peer"),
			wantRetry: true,
		},
		{
			name:      "declined via modelled exception stops",
			err:       &ssooidctypes.AccessDeniedException{},
			wantRetry: false,
			wantMsg:   "the authorization request was denied",
		},
		{
			// CreateToken's deserializer falls through to a GenericAPIError for codes it
			// does not model, so a declined prompt can arrive as the OAuth code.
			name:      "declined via generic oauth code stops",
			err:       &smithy.GenericAPIError{Code: "access_denied", Message: "User declined"},
			wantRetry: false,
			wantMsg:   "the authorization request was denied",
		},
		{
			name:      "expired device code stops",
			err:       &ssooidctypes.ExpiredTokenException{},
			wantRetry: false,
			wantMsg:   "expired before it was approved",
		},
		{
			name:      "invalid grant stops",
			err:       &ssooidctypes.InvalidGrantException{},
			wantRetry: false,
			wantMsg:   "authorization failed",
		},
		{
			name:      "unauthorized client stops",
			err:       &ssooidctypes.UnauthorizedClientException{},
			wantRetry: false,
			wantMsg:   "authorization failed",
		},
		{
			name:      "unknown api error stops",
			err:       &smithy.GenericAPIError{Code: "SomethingNew", Message: "?"},
			wantRetry: false,
			wantMsg:   "authorization failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			retry, fatal := classifySSOTokenError(tt.err)
			assert.Equal(t, tt.wantRetry, retry)
			if tt.wantRetry {
				require.NoError(t, fatal)
				return
			}
			require.Error(t, fatal)
			assert.Contains(t, fatal.Error(), tt.wantMsg)
		})
	}
}

// The SDK wraps service errors in a smithy OperationError, so classification has to unwrap.
func TestClassifySSOTokenErrorUnwrapsOperationError(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("operation error SSO OIDC: CreateToken, %w", &ssooidctypes.AccessDeniedException{})
	retry, fatal := classifySSOTokenError(wrapped)
	assert.False(t, retry)
	require.Error(t, fatal)
	assert.Contains(t, fatal.Error(), "denied")

	wrapped = fmt.Errorf("operation error SSO OIDC: CreateToken, %w", &ssooidctypes.AuthorizationPendingException{})
	retry, fatal = classifySSOTokenError(wrapped)
	assert.True(t, retry)
	require.NoError(t, fatal)
}

func TestCredentialSourceLabel(t *testing.T) {
	t.Parallel()

	// Source strings observed from aws.Credentials.Retrieve.
	assert.Equal(t, "cached AWS SSO session", credentialSourceLabel("SSOProvider"))
	assert.Equal(t, "environment variables", credentialSourceLabel("EnvConfigCredentials"))
	assert.Equal(t, "EC2 instance role", credentialSourceLabel("EC2RoleProvider"))
	assert.Equal(t, "AWS credentials", credentialSourceLabel(""))
	// Unknown sources fall through verbatim rather than being mislabelled.
	assert.Equal(t, "SomeFutureProvider", credentialSourceLabel("SomeFutureProvider"))
}

func TestOIDCIssuer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		backendURL string
		want       string
		wantErr    string
	}{
		{backendURL: "https://api.pulumi.com", want: "https://api.pulumi.com/oidc"},
		{backendURL: "https://api.pulumi.com/", want: "https://api.pulumi.com/oidc"},
		// The org login in a self-hosted backend URL must be dropped from the issuer.
		{backendURL: "http://localhost:3000/syeh", want: "http://localhost:3000/oidc"},
		{backendURL: "https://api.pulumi-staging.io/myorg", want: "https://api.pulumi-staging.io/oidc"},
		{backendURL: "", wantErr: "could not determine the current backend"},
		{backendURL: "api.pulumi.com", wantErr: "not an absolute URL"},
	}

	for _, tt := range tests {
		t.Run(tt.backendURL, func(t *testing.T) {
			t.Parallel()
			setup := &setupCommand{env: &envCommand{esc: &escCommand{
				account: Account{BackendURL: tt.backendURL},
			}}}
			got, err := setup.oidcIssuer()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSanitizeEnvName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		accountName string
		accountID   string
		want        string
	}{
		{"Production", "111111111111", "production-env"},
		{"Acme Corp / Staging", "222222222222", "acme-corp---staging-env"},
		{"already-fine_1.0", "333333333333", "already-fine_1.0-env"},
		{"", "444444444444", "444444444444-env"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, sanitizeEnvName(tt.accountName, tt.accountID))
		})
	}
}

// `env setup aws` rejects a --policy that is not an ARN before creating anything, so the presets
// must themselves parse and a bare policy name must not.
func TestAWSPolicyPresetsAreValidARNs(t *testing.T) {
	t.Parallel()

	for _, choice := range awsPolicyChoices {
		parsed, err := arn.Parse(choice.id)
		require.NoError(t, err, "preset %s", choice.name)
		assert.Equal(t, "iam", parsed.Service)
		// path.Base of the ARN is what names the OIDC role, so the resource must carry the
		// policy name rather than being empty.
		assert.Equal(t, "policy/"+choice.name, parsed.Resource)
	}

	// The case the early check exists for: an official name passed where an ARN is expected.
	_, err := arn.Parse("AdministratorAccess")
	assert.Error(t, err)
}

// resolvePolicy hands back the AWS presets as policy ARNs, and a custom policy untouched.
func TestResolvePolicy_AWSResolvesToPolicyARNs(t *testing.T) {
	t.Parallel()

	s := &setupCommand{}

	got, err := s.resolvePolicy("admin", awsPolicyChoices, false)
	require.NoError(t, err)
	assert.Equal(t, "arn:aws:iam::aws:policy/AdministratorAccess", got)

	got, err = s.resolvePolicy("ReadOnlyAccess", awsPolicyChoices, false)
	require.NoError(t, err)
	assert.Equal(t, "arn:aws:iam::aws:policy/ReadOnlyAccess", got)

	// A customer-managed policy lives under the account ID, not "aws", so it can only be
	// selected by passing its ARN.
	custom := "arn:aws:iam::123456789012:policy/MyPolicy"
	got, err = s.resolvePolicy(custom, awsPolicyChoices, false)
	require.NoError(t, err)
	assert.Equal(t, custom, got)
}
