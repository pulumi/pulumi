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

package insights

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWSConfig_DefaultRoleName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		accountID string
		expected  string
	}{
		{
			name:      "standard account",
			accountID: "123456789012",
			expected:  "pulumi-insights-123456789012",
		},
		{
			name:      "different account",
			accountID: "987654321098",
			expected:  "pulumi-insights-987654321098",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &AWSConfig{
				AccountID: tt.accountID,
			}
			result := cfg.DefaultRoleName()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParsePartitionFromARN(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		arn      string
		expected string
	}{
		{
			name:     "standard AWS partition",
			arn:      "arn:aws:iam::123456789012:user/test",
			expected: "aws",
		},
		{
			name:     "GovCloud partition",
			arn:      "arn:aws-us-gov:iam::123456789012:user/test",
			expected: "aws-us-gov",
		},
		{
			name:     "China partition",
			arn:      "arn:aws-cn:iam::123456789012:user/test",
			expected: "aws-cn",
		},
		{
			name:     "malformed ARN",
			arn:      "not-an-arn",
			expected: "aws",
		},
		{
			name:     "empty ARN",
			arn:      "",
			expected: "aws",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := parsePartitionFromARN(tt.arn)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseAWSConfigProfiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name: "config with default and named profiles",
			content: `[default]
region = us-east-1

[profile dev]
region = us-west-2

[profile prod]
region = eu-west-1
`,
			expected: []string{"default", "dev", "prod"},
		},
		{
			name: "config with only default",
			content: `[default]
region = us-east-1
`,
			expected: []string{"default"},
		},
		{
			name: "config with only named profiles",
			content: `[profile staging]
region = us-east-1

[profile production]
region = eu-west-1
`,
			expected: []string{"staging", "production"},
		},
		{
			name:     "empty config",
			content:  "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create temp file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config")
			err := os.WriteFile(configPath, []byte(tt.content), 0600)
			require.NoError(t, err)

			// Parse profiles
			profiles, err := parseAWSConfigProfiles(configPath)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, profiles)
		})
	}
}

func TestParseAWSConfigProfiles_NonExistent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent")

	_, err := parseAWSConfigProfiles(configPath)
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestParseAWSCredentialProfiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name: "credentials with multiple profiles",
			content: `[default]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

[dev]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
`,
			expected: []string{"default", "dev"},
		},
		{
			name: "credentials with only default",
			content: `[default]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
`,
			expected: []string{"default"},
		},
		{
			name:     "empty credentials",
			content:  "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create temp file
			tmpDir := t.TempDir()
			credPath := filepath.Join(tmpDir, "credentials")
			err := os.WriteFile(credPath, []byte(tt.content), 0600)
			require.NoError(t, err)

			// Parse profiles
			profiles, err := parseAWSCredentialProfiles(credPath)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, profiles)
		})
	}
}

// mockIAMClient implements the iamClient interface for testing.
type mockIAMClient struct {
	CreateOpenIDConnectProviderF        func(*iam.CreateOpenIDConnectProviderInput) (*iam.CreateOpenIDConnectProviderOutput, error)
	AddClientIDToOpenIDConnectProviderF func(*iam.AddClientIDToOpenIDConnectProviderInput) (*iam.AddClientIDToOpenIDConnectProviderOutput, error)
	GetOpenIDConnectProviderF           func(*iam.GetOpenIDConnectProviderInput) (*iam.GetOpenIDConnectProviderOutput, error)
	CreateRoleF                         func(*iam.CreateRoleInput) (*iam.CreateRoleOutput, error)
	GetRoleF                            func(*iam.GetRoleInput) (*iam.GetRoleOutput, error)
	AttachRolePolicyF                   func(*iam.AttachRolePolicyInput) (*iam.AttachRolePolicyOutput, error)
}

func (m *mockIAMClient) CreateOpenIDConnectProvider(ctx context.Context, params *iam.CreateOpenIDConnectProviderInput,
	optFns ...func(*iam.Options)) (*iam.CreateOpenIDConnectProviderOutput, error) {
	if m.CreateOpenIDConnectProviderF != nil {
		return m.CreateOpenIDConnectProviderF(params)
	}
	return nil, nil
}

func (m *mockIAMClient) AddClientIDToOpenIDConnectProvider(ctx context.Context, params *iam.AddClientIDToOpenIDConnectProviderInput,
	optFns ...func(*iam.Options)) (*iam.AddClientIDToOpenIDConnectProviderOutput, error) {
	if m.AddClientIDToOpenIDConnectProviderF != nil {
		return m.AddClientIDToOpenIDConnectProviderF(params)
	}
	return nil, nil
}

func (m *mockIAMClient) GetOpenIDConnectProvider(ctx context.Context, params *iam.GetOpenIDConnectProviderInput,
	optFns ...func(*iam.Options)) (*iam.GetOpenIDConnectProviderOutput, error) {
	if m.GetOpenIDConnectProviderF != nil {
		return m.GetOpenIDConnectProviderF(params)
	}
	return nil, nil
}

func (m *mockIAMClient) CreateRole(ctx context.Context, params *iam.CreateRoleInput,
	optFns ...func(*iam.Options)) (*iam.CreateRoleOutput, error) {
	if m.CreateRoleF != nil {
		return m.CreateRoleF(params)
	}
	return nil, nil
}

func (m *mockIAMClient) GetRole(ctx context.Context, params *iam.GetRoleInput,
	optFns ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	if m.GetRoleF != nil {
		return m.GetRoleF(params)
	}
	return nil, nil
}

func (m *mockIAMClient) AttachRolePolicy(ctx context.Context, params *iam.AttachRolePolicyInput,
	optFns ...func(*iam.Options)) (*iam.AttachRolePolicyOutput, error) {
	if m.AttachRolePolicyF != nil {
		return m.AttachRolePolicyF(params)
	}
	return nil, nil
}

func TestSetupOIDC_CreateNewProviderAndRole(t *testing.T) {
	t.Parallel()

	cfg := &AWSConfig{
		AccountID: "123456789012",
		Partition: "aws",
	}
	orgName := "test-org"
	roleName := "pulumi-insights-123456789012"

	mock := &mockIAMClient{
		CreateOpenIDConnectProviderF: func(params *iam.CreateOpenIDConnectProviderInput) (*iam.CreateOpenIDConnectProviderOutput, error) {
			assert.Equal(t, oidcIssuerURL, aws.ToString(params.Url))
			assert.Contains(t, params.ClientIDList, "aws:test-org")
			return &iam.CreateOpenIDConnectProviderOutput{
				OpenIDConnectProviderArn: aws.String("arn:aws:iam::123456789012:oidc-provider/api.pulumi.com/oidc"),
			}, nil
		},
		CreateRoleF: func(params *iam.CreateRoleInput) (*iam.CreateRoleOutput, error) {
			assert.Equal(t, roleName, aws.ToString(params.RoleName))
			assert.NotEmpty(t, params.AssumeRolePolicyDocument)
			return &iam.CreateRoleOutput{
				Role: &iamtypes.Role{
					Arn: aws.String("arn:aws:iam::123456789012:role/" + roleName),
				},
			}, nil
		},
		AttachRolePolicyF: func(params *iam.AttachRolePolicyInput) (*iam.AttachRolePolicyOutput, error) {
			assert.Equal(t, roleName, aws.ToString(params.RoleName))
			assert.Equal(t, readOnlyAccessPolicyARN, aws.ToString(params.PolicyArn))
			return &iam.AttachRolePolicyOutput{}, nil
		},
	}

	ctx := context.Background()
	result, err := setupOIDC(ctx, mock, cfg, orgName, roleName)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.OIDCProviderNew)
	assert.True(t, result.RoleNew)
	assert.Equal(t, "arn:aws:iam::123456789012:oidc-provider/api.pulumi.com/oidc", result.OIDCProviderARN)
	assert.Equal(t, "arn:aws:iam::123456789012:role/"+roleName, result.RoleARN)
}

func TestSetupOIDC_ExistingProvider(t *testing.T) {
	t.Parallel()

	cfg := &AWSConfig{
		AccountID: "123456789012",
		Partition: "aws",
	}
	orgName := "test-org"
	roleName := "pulumi-insights-123456789012"

	mock := &mockIAMClient{
		CreateOpenIDConnectProviderF: func(params *iam.CreateOpenIDConnectProviderInput) (*iam.CreateOpenIDConnectProviderOutput, error) {
			return nil, &iamtypes.EntityAlreadyExistsException{
				Message: aws.String("Provider already exists"),
			}
		},
		GetOpenIDConnectProviderF: func(params *iam.GetOpenIDConnectProviderInput) (*iam.GetOpenIDConnectProviderOutput, error) {
			return &iam.GetOpenIDConnectProviderOutput{
				ClientIDList: []string{"aws:other-org"},
			}, nil
		},
		AddClientIDToOpenIDConnectProviderF: func(params *iam.AddClientIDToOpenIDConnectProviderInput) (*iam.AddClientIDToOpenIDConnectProviderOutput, error) {
			return &iam.AddClientIDToOpenIDConnectProviderOutput{}, nil
		},
		CreateRoleF: func(params *iam.CreateRoleInput) (*iam.CreateRoleOutput, error) {
			return &iam.CreateRoleOutput{
				Role: &iamtypes.Role{
					Arn: aws.String("arn:aws:iam::123456789012:role/" + roleName),
				},
			}, nil
		},
		AttachRolePolicyF: func(params *iam.AttachRolePolicyInput) (*iam.AttachRolePolicyOutput, error) {
			return &iam.AttachRolePolicyOutput{}, nil
		},
	}

	ctx := context.Background()
	result, err := setupOIDC(ctx, mock, cfg, orgName, roleName)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.OIDCProviderNew)
	assert.True(t, result.RoleNew)
}

func TestSetupOIDC_GovCloudPartition(t *testing.T) {
	t.Parallel()

	cfg := &AWSConfig{
		AccountID: "123456789012",
		Partition: "aws-us-gov",
	}
	orgName := "test-org"
	roleName := "pulumi-insights-123456789012"

	var attachedPolicyARN string
	mock := &mockIAMClient{
		CreateOpenIDConnectProviderF: func(params *iam.CreateOpenIDConnectProviderInput) (*iam.CreateOpenIDConnectProviderOutput, error) {
			return &iam.CreateOpenIDConnectProviderOutput{
				OpenIDConnectProviderArn: aws.String("arn:aws-us-gov:iam::123456789012:oidc-provider/api.pulumi.com/oidc"),
			}, nil
		},
		CreateRoleF: func(params *iam.CreateRoleInput) (*iam.CreateRoleOutput, error) {
			return &iam.CreateRoleOutput{
				Role: &iamtypes.Role{
					Arn: aws.String("arn:aws-us-gov:iam::123456789012:role/" + roleName),
				},
			}, nil
		},
		AttachRolePolicyF: func(params *iam.AttachRolePolicyInput) (*iam.AttachRolePolicyOutput, error) {
			attachedPolicyARN = aws.ToString(params.PolicyArn)
			return &iam.AttachRolePolicyOutput{}, nil
		},
	}

	ctx := context.Background()
	result, err := setupOIDC(ctx, mock, cfg, orgName, roleName)

	require.NoError(t, err)
	assert.NotNil(t, result)
	// Verify the policy ARN uses the correct partition
	assert.Equal(t, "arn:aws-us-gov:iam::aws:policy/ReadOnlyAccess", attachedPolicyARN)
}

func TestRoleExistsError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		err              *roleExistsError
		expectedContains string
	}{
		{
			name: "with audiences",
			err: &roleExistsError{
				roleName:         "test-role",
				accountID:        "123456789012",
				audience:         "aws:org1",
				currentAudiences: []string{"aws:org2", "aws:org3"},
			},
			expectedContains: "already exists",
		},
		{
			name: "without audiences",
			err: &roleExistsError{
				roleName:  "test-role",
				accountID: "123456789012",
				audience:  "aws:org1",
			},
			expectedContains: "already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			errMsg := tt.err.Error()
			assert.Contains(t, errMsg, tt.expectedContains)
			assert.Contains(t, errMsg, tt.err.roleName)
			assert.Contains(t, errMsg, tt.err.accountID)
		})
	}
}
