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

package authhelpers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"
)

// Represents a role to assume.
// Mirrors the structure used by pulumi-aws provider's assumeRoles option.
type AssumeRoleConfig struct {
	// RoleArn is the ARN of the role to assume (required).
	RoleArn string `json:"roleArn"`
	// ExternalID is an external ID to use when assuming the role (optional).
	ExternalID *string `json:"externalId,omitempty"`
	// SessionName is the session name to use when assuming the role (optional).
	SessionName string `json:"sessionName,omitempty"`
}

// Credentials provider that chains through a list of AWS credentials
// providers, returning the first successful result.
type assumeRolesChainedProvider struct {
	providers []awsv2.CredentialsProvider
}

// Retrieve attempts to retrieve credentials from each provider in
// order, returning the first successful result.
func (p *assumeRolesChainedProvider) Retrieve(ctx context.Context) (awsv2.Credentials, error) {
	var lastErr error
	for _, provider := range p.providers {
		creds, err := provider.Retrieve(ctx)
		if err == nil {
			return creds, nil
		}
		lastErr = err
	}
	return awsv2.Credentials{}, fmt.Errorf("all credentials providers failed: %w", lastErr)
}

// Credentials provider that uses stscreds.AssumeRoleProvider.
type assumeRoleProviderWithSTS struct {
	roleArn     string
	externalID  *string
	sessionName string
	baseCreds   awsv2.CredentialsProvider
	region      string
}

// Retrieve retrieves temporary credentials by assuming the role via STS.
func (p *assumeRoleProviderWithSTS) Retrieve(ctx context.Context) (awsv2.Credentials, error) {
	// STS is a global service, default to us-east-1 if no region specified.
	region := p.region
	if region == "" {
		region = "us-east-1"
	}

	// Create STS client with base credentials
	stsCfg := awsv2.Config{
		Credentials: p.baseCreds,
		Region:      region,
	}
	stsClient := sts.NewFromConfig(stsCfg)

	// Create assume role provider
	provider := stscreds.NewAssumeRoleProvider(stsClient, p.roleArn, func(o *stscreds.AssumeRoleOptions) {
		if p.externalID != nil {
			o.ExternalID = p.externalID
		}
		if p.sessionName != "" {
			o.RoleSessionName = p.sessionName
		}
	})

	creds, err := provider.Retrieve(ctx)
	return creds, err
}

// Create a credentials provider that attempts to assume the specified roles in
// order. If assumeRoles is empty, it returns a provider that uses the default
// AWS credentials chain.
// When assumeRoles is specified, assume role providers are tried first (in order),
// and the default credentials chain is used as a fallback if all assume roles fail.
func NewAssumeRoleChainedCredentials(
	ctx context.Context,
	assumeRoles []AssumeRoleConfig,
	region string,
) (awsv2.CredentialsProvider, error) {
	// Base provider: always start with the default credentials chain
	base, err := newDefaultCredChain(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("failed to create default credentials provider: %w", err)
	}

	providers := make([]awsv2.CredentialsProvider, 0, len(assumeRoles)+1)

	// Add assume role providers for each role (tried first, in order)
	for _, role := range assumeRoles {
		provider := &assumeRoleProviderWithSTS{
			roleArn:     role.RoleArn,
			externalID:  role.ExternalID,
			sessionName: role.SessionName,
			baseCreds:   base,
			region:      region,
		}
		providers = append(providers, provider)
	}

	// Default credentials chain is the fallback (tried last)
	providers = append(providers, base)

	return &assumeRolesChainedProvider{providers: providers}, nil
}

// Create a credentials provider that uses the default AWS credentials
// chain (env vars, shared credentials file, EC2/ECS instance roles,
// etc.).
func newDefaultCredChain(ctx context.Context, region string) (awsv2.CredentialsProvider, error) {
	var opts []func(*config.LoadOptions) error
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load default AWS config: %w", err)
	}

	return cfg.Credentials, nil
}

// Parse the assumeRoles query parameter from a URL and return
// the list of role configs, or nil if not specified.
// The assumeRoles parameter is expected to be a JSON array of objects,
// URL-encoded. For example:
//
//	s3://mybucket?assumeRoles=[{"roleArn":"arn:aws:iam::123456789012:role/Role1","externalId":"ext1"}]
func AssumeRoleFromURLParams(q url.Values) ([]AssumeRoleConfig, error) {
	rolesParam := q.Get("assumeRoles")
	if rolesParam == "" {
		return nil, nil
	}

	// URL decode the parameter (it may be URL-encoded by the URL parser)
	decoded, err := url.QueryUnescape(rolesParam)
	if err != nil {
		return nil, fmt.Errorf("failed to URL-decode assumeRoles parameter: %w", err)
	}

	// Parse JSON
	var roles []AssumeRoleConfig
	if err := json.Unmarshal([]byte(decoded), &roles); err != nil {
		return nil, fmt.Errorf("failed to parse assumeRoles as JSON array: %w (value: %q)", err, decoded)
	}

	// Validate that each role has a roleArn
	for i, role := range roles {
		if role.RoleArn == "" {
			return nil, fmt.Errorf("assumeRoles[%d]: roleArn is required", i)
		}
		if !strings.HasPrefix(role.RoleArn, "arn:") {
			return nil, fmt.Errorf("assumeRoles[%d]: roleArn must be a valid ARN (starts with 'arn:'): %s", i, role.RoleArn)
		}
	}

	return roles, nil
}

// Create a blob.URLMux configured for S3 with optional assume role
// support. If assumeRoles is specified, credentials will be obtained
// by assuming roles in order (starting with the default chain).
// If assumeRoles is empty, the default AWS credentials chain is used.
func AWSCredentialsMux(ctx context.Context, assumeRoles []AssumeRoleConfig, region string) (*blob.URLMux, error) {
	credsProvider, err := NewAssumeRoleChainedCredentials(ctx, assumeRoles, region)
	if err != nil {
		return nil, fmt.Errorf("failed to create credentials provider: %w", err)
	}

	// Create an S3 client with the credentials provider
	s3Config := awsv2.Config{
		Credentials: credsProvider,
		Region:      region,
	}

	s3Client := s3.NewFromConfig(s3Config)

	// Create the URL mux
	bm := &blob.URLMux{}
	bm.RegisterBucket(s3blob.Scheme, &s3blobURLOpener{
		Client:      s3Client,
		AssumeRoles: assumeRoles,
	})

	return bm, nil
}

// Custom opener for S3 URLs that supports assume role credentials via
// the assumeRoles parameter.
type s3blobURLOpener struct {
	Client      *s3.Client
	AssumeRoles []AssumeRoleConfig
}

// Open an S3 bucket URL, using assume role credentials if assumeRoles
// is specified.
func (o *s3blobURLOpener) OpenBucketURL(ctx context.Context, u *url.URL) (*blob.Bucket, error) {
	q := u.Query()

	// Parse server-side encryption parameters
	sseTypeParam := q.Get("ssetype")
	if sseTypeParam != "" {
		q.Del("ssetype")
	}
	kmsKeyID := q.Get("kmskeyid")
	if kmsKeyID != "" {
		q.Del("kmskeyid")
	}

	// Check if we need to use a different SDK version
	useV2 := true
	if val := q.Get("awssdk"); val == "v1" || val == "V1" {
		useV2 = false
	}

	if !useV2 {
		return nil, errors.New("AWS SDK v1 is not supported with assumeRoles")
	}

	// Create encryption options
	var opts s3blob.Options
	if sseTypeParam != "" {
		for _, sseType := range types.ServerSideEncryptionAes256.Values() {
			if strings.EqualFold(string(sseType), sseTypeParam) {
				opts.EncryptionType = sseType
				break
			}
		}
		for _, sseType := range types.ServerSideEncryptionAwsKms.Values() {
			if strings.EqualFold(string(sseType), sseTypeParam) {
				opts.EncryptionType = sseType
				break
			}
		}
	}
	if kmsKeyID != "" {
		opts.KMSEncryptionID = kmsKeyID
	}

	return s3blob.OpenBucketV2(ctx, o.Client, u.Host, &opts)
}
