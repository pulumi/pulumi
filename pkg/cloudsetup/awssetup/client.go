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

// Package awssetup provides AWS-specific cloud setup functionality
package awssetup

import (
	"context"
	//nolint:gosec // sha1 used for non-cryptographic fingerprinting only
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	ssooidc_types "github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	awssetup_types "github.com/pulumi/pulumi/pkg/v3/cloudsetup/awssetup/types"
	cloudsetup "github.com/pulumi/pulumi/pkg/v3/cloudsetup/common"
)

const (
	ResourceTypeAWSOIDCProvider         = "aws:iam:oidc-provider"
	ResourceTypeAWSRole                 = "aws:iam:role"
	ResourceTypeAWSRolePolicyAttachment = "aws:iam:role-policy-attachment"
)

// AWS IAM throttles per-account at low rates (e.g. CreateOpenIDConnectProvider at ~5 req/sec),
// which can be exceeded during bulk onboarding flows. Bumping the SDK's default retry budget
// (3 attempts / 20s max backoff) gives the standard exponential-backoff retryer enough headroom
// to absorb realistic ThrottlingException bursts.
const (
	awsMaxRetryAttempts = 6
	awsMaxRetryBackoff  = 30 * time.Second
)

type Client interface {
	SetupOIDCInfrastructure(
		ctx context.Context, orgName string, oidcRoleName string, policyArn string,
	) (*cloudsetup.CloudSetupResult, error)
}

type SSOClient interface {
	Initiate(ctx context.Context, startURL string) (*awssetup_types.SSOConfig, error)
	ExchangeAccessToken(ctx context.Context, ssoCfg *awssetup_types.SSOConfig) (string, error)
	ListAccounts(ctx context.Context, accessToken string) ([]cloudsetup.CloudAccount, error)
	ExchangeCredentials(
		ctx context.Context, accessToken string, accountID string, accountRoleName string,
	) (awssetup_types.Config, error)
}

type IamClient interface {
	CreateOpenIDConnectProvider(
		ctx context.Context, input *iam.CreateOpenIDConnectProviderInput, options ...func(*iam.Options),
	) (*iam.CreateOpenIDConnectProviderOutput, error)
	AddClientIDToOpenIDConnectProvider(
		ctx context.Context, params *iam.AddClientIDToOpenIDConnectProviderInput, optFns ...func(*iam.Options),
	) (*iam.AddClientIDToOpenIDConnectProviderOutput, error)
	CreateRole(
		ctx context.Context, input *iam.CreateRoleInput, options ...func(*iam.Options),
	) (*iam.CreateRoleOutput, error)
	AttachRolePolicy(
		ctx context.Context, input *iam.AttachRolePolicyInput, options ...func(*iam.Options),
	) (*iam.AttachRolePolicyOutput, error)
}

type StsClient interface {
	GetCallerIdentity(
		ctx context.Context, input *sts.GetCallerIdentityInput, options ...func(*sts.Options),
	) (*sts.GetCallerIdentityOutput, error)
}

type SsoOidcClient interface {
	RegisterClient(
		ctx context.Context, input *ssooidc.RegisterClientInput, options ...func(*ssooidc.Options),
	) (*ssooidc.RegisterClientOutput, error)
	StartDeviceAuthorization(
		ctx context.Context, input *ssooidc.StartDeviceAuthorizationInput, options ...func(*ssooidc.Options),
	) (*ssooidc.StartDeviceAuthorizationOutput, error)
	CreateToken(
		ctx context.Context, input *ssooidc.CreateTokenInput, options ...func(*ssooidc.Options),
	) (*ssooidc.CreateTokenOutput, error)
}

type SsoClient interface {
	ListAccounts(
		ctx context.Context, input *sso.ListAccountsInput, options ...func(*sso.Options),
	) (*sso.ListAccountsOutput, error)
	ListAccountRoles(
		ctx context.Context, input *sso.ListAccountRolesInput, options ...func(*sso.Options),
	) (*sso.ListAccountRolesOutput, error)
	GetRoleCredentials(
		ctx context.Context, input *sso.GetRoleCredentialsInput, options ...func(*sso.Options),
	) (*sso.GetRoleCredentialsOutput, error)
}

type client struct {
	awsCfg             aws.Config
	iamClient          IamClient
	stsClient          StsClient
	oidcIssuer         string
	thumbprintProvider ThumbprintProvider
}

type ssoClient struct {
	awsCfg        aws.Config
	ssoOidcClient SsoOidcClient
	ssoClient     SsoClient
}

func NewClient(ctx context.Context, cfg *awssetup_types.Config, oidcIssuer string) (Client, error) {
	if cfg == nil {
		return nil, cloudsetup.NewSetupError(cloudsetup.ErrorCodeInvalidCredentials, "config cannot be nil", nil)
	}

	var awsCfg aws.Config
	var err error
	if cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" || cfg.SessionToken == "" || cfg.Region == "" {
		return nil, cloudsetup.NewSetupError(cloudsetup.ErrorCodeInvalidCredentials, "credentials not found", err)
	}

	creds := credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken)
	awsCfg, err = config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(creds),
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, cloudsetup.NewSetupError(cloudsetup.ErrorCodeInvalidCredentials, "failed to load AWS config", err)
	}

	return NewClientFromConfig(awsCfg, oidcIssuer), nil
}

// NewClientFromConfig builds a Client from an already-resolved AWS config, for callers whose
// credentials come from the ambient provider chain rather than an explicit key pair.
func NewClientFromConfig(awsCfg aws.Config, oidcIssuer string) Client {
	awsCfg.Retryer = func() aws.Retryer {
		return retry.NewStandard(func(o *retry.StandardOptions) {
			o.MaxAttempts = awsMaxRetryAttempts
			o.MaxBackoff = awsMaxRetryBackoff
		})
	}

	return &client{
		awsCfg:             awsCfg,
		iamClient:          iam.NewFromConfig(awsCfg),
		stsClient:          sts.NewFromConfig(awsCfg),
		oidcIssuer:         oidcIssuer,
		thumbprintProvider: DefaultThumbprintProvider{},
	}
}

func NewSSOClient(ctx context.Context, region string) (SSOClient, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}
	return &ssoClient{
		awsCfg:        awsCfg,
		ssoOidcClient: ssooidc.NewFromConfig(awsCfg),
		ssoClient:     sso.NewFromConfig(awsCfg),
	}, nil
}

func (c *ssoClient) Initiate(ctx context.Context, startURL string) (*awssetup_types.SSOConfig, error) {
	regResp, err := c.ssoOidcClient.RegisterClient(ctx, &ssooidc.RegisterClientInput{
		ClientName: aws.String("pulumi-esc-setup"),
		ClientType: aws.String("public"),
		Scopes:     []string{"sso:account:access"},
	})
	if err != nil {
		return nil, err
	}

	authResp, err := c.ssoOidcClient.StartDeviceAuthorization(ctx, &ssooidc.StartDeviceAuthorizationInput{
		ClientId:     regResp.ClientId,
		ClientSecret: regResp.ClientSecret,
		StartUrl:     aws.String(startURL),
	})
	if err != nil {
		var invalidRequestErr *ssooidc_types.InvalidRequestException
		if errors.As(err, &invalidRequestErr) && invalidRequestErr.Error_description != nil {
			// Provide a clearer error message when for example the start URL is invalid
			return nil, errors.New(*invalidRequestErr.Error_description)
		}
		return nil, err
	}

	return &awssetup_types.SSOConfig{
		VerificationURL: *authResp.VerificationUriComplete,
		ClientID:        *regResp.ClientId,
		ClientSecret:    *regResp.ClientSecret,
		DeviceCode:      *authResp.DeviceCode,
		Interval:        authResp.Interval,
		UserCode:        *authResp.UserCode,
		ExpiresIn:       authResp.ExpiresIn,
	}, nil
}

func (c *ssoClient) ExchangeAccessToken(ctx context.Context, ssoCfg *awssetup_types.SSOConfig) (string, error) {
	tokenResp, err := c.ssoOidcClient.CreateToken(ctx, &ssooidc.CreateTokenInput{
		ClientId:     aws.String(ssoCfg.ClientID),
		ClientSecret: aws.String(ssoCfg.ClientSecret),
		DeviceCode:   aws.String(ssoCfg.DeviceCode),
		GrantType:    aws.String("urn:ietf:params:oauth:grant-type:device_code"),
	})
	if err != nil {
		return "", err
	}
	return *tokenResp.AccessToken, nil
}

func (c *ssoClient) ListAccounts(ctx context.Context, accessToken string) ([]cloudsetup.CloudAccount, error) {
	var accounts []cloudsetup.CloudAccount
	var accountsNextToken *string
	for {
		listAccountsOutput, err := c.ssoClient.ListAccounts(ctx, &sso.ListAccountsInput{
			AccessToken: aws.String(accessToken),
			NextToken:   accountsNextToken,
			MaxResults:  aws.Int32(100),
		})
		if err != nil {
			return nil, err
		}
		for _, account := range listAccountsOutput.AccountList {
			var roles []string
			var rolesNextToken *string
			for {
				rolesOutput, err := c.ssoClient.ListAccountRoles(ctx, &sso.ListAccountRolesInput{
					AccessToken: aws.String(accessToken),
					AccountId:   account.AccountId,
					NextToken:   rolesNextToken,
					MaxResults:  aws.Int32(100),
				})
				if err != nil {
					return nil, err
				}
				for _, role := range rolesOutput.RoleList {
					roles = append(roles, aws.ToString(role.RoleName))
				}
				if rolesOutput.NextToken == nil {
					break
				}
				rolesNextToken = rolesOutput.NextToken
			}
			accounts = append(accounts, cloudsetup.CloudAccount{
				ID:    aws.ToString(account.AccountId),
				Name:  aws.ToString(account.AccountName),
				Roles: roles,
			})
		}
		if listAccountsOutput.NextToken == nil {
			break
		}
		accountsNextToken = listAccountsOutput.NextToken
	}

	return accounts, nil
}

func (c *ssoClient) ExchangeCredentials(
	ctx context.Context, accessToken string, accountID string, accountRoleName string,
) (awssetup_types.Config, error) {
	resp, err := c.ssoClient.GetRoleCredentials(ctx, &sso.GetRoleCredentialsInput{
		AccessToken: aws.String(accessToken),
		AccountId:   aws.String(accountID),
		RoleName:    aws.String(accountRoleName),
	})
	if err != nil {
		return awssetup_types.Config{}, err
	}
	return awssetup_types.Config{
		AccessKeyID:     *resp.RoleCredentials.AccessKeyId,
		SecretAccessKey: *resp.RoleCredentials.SecretAccessKey,
		SessionToken:    *resp.RoleCredentials.SessionToken,
		Region:          c.awsCfg.Region,
	}, nil
}

func (c *client) SetupOIDCInfrastructure(
	ctx context.Context, orgName string, oidcRoleName string, policyArn string,
) (*cloudsetup.CloudSetupResult, error) {
	// Get account ID and partition from caller identity
	identity, err := c.stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, cloudsetup.NewSetupError(cloudsetup.ErrorCodeInvalidCredentials, "failed to get account ID", err)
	}
	accountID := *identity.Account

	// Extract partition from the caller's ARN (e.g., "aws", "aws-us-gov", "aws-iso", "aws-iso-b")
	// The ARN format is: arn:partition:service:region:account-id:resource
	partition := "aws" // default to standard AWS partition
	if identity.Arn != nil {
		if parsedPartition, err := parsePartitionFromARN(*identity.Arn); err == nil {
			partition = parsedPartition
		}
	}

	result := &cloudsetup.CloudSetupResult{
		Success:   false,
		Resources: []cloudsetup.CloudSetupResource{},
	}

	parsedURL, err := url.Parse(c.oidcIssuer)
	if err != nil {
		return nil, cloudsetup.NewSetupError(cloudsetup.ErrorCodeSetupFailed, "invalid OIDC URL", err)
	}
	hostname := parsedURL.Hostname()

	// Create OIDC provider
	thumbprint, err := c.thumbprintProvider.GetCertThumbprint(hostname)
	if err != nil {
		return nil, cloudsetup.NewSetupError(cloudsetup.ErrorCodeSetupFailed, "failed to create thumbprint", err)
	}

	audience := "aws:" + orgName
	oidcProviderArn := fmt.Sprintf("arn:%s:iam::%s:oidc-provider/%s/oidc", partition, accountID, parsedURL.Host)
	_, err = c.iamClient.CreateOpenIDConnectProvider(ctx, &iam.CreateOpenIDConnectProviderInput{
		Url:            aws.String(c.oidcIssuer),
		ClientIDList:   []string{audience},
		ThumbprintList: []string{thumbprint},
	})
	if err != nil && !isEntityExists(err) {
		return cloudsetup.WrapSetupError(result, ResourceTypeAWSOIDCProvider, err)
	}

	oidcProviderStatus := cloudsetup.ResourceStatusCreated
	if isEntityExists(err) {
		oidcProviderStatus = cloudsetup.ResourceStatusExisting

		// An OIDC provider for our issuer already exists, so just add this org to its allowed audiences
		// (This operation is idempotent)
		_, err := c.iamClient.AddClientIDToOpenIDConnectProvider(ctx, &iam.AddClientIDToOpenIDConnectProviderInput{
			OpenIDConnectProviderArn: aws.String(oidcProviderArn),
			ClientID:                 aws.String(audience),
		})
		if err != nil {
			return cloudsetup.WrapSetupError(result, ResourceTypeAWSOIDCProvider, err)
		}
	}
	result.Resources = append(result.Resources, cloudsetup.CloudSetupResource{
		Type:   ResourceTypeAWSOIDCProvider,
		ID:     oidcProviderArn,
		Name:   "PulumiServiceOIDCProvider",
		Status: oidcProviderStatus,
	})

	// Create role trust policy
	oidcArn := fmt.Sprintf("arn:%s:iam::%s:oidc-provider/%s/oidc", partition, accountID, parsedURL.Host)
	trustPolicy := map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Effect": "Allow",
				"Principal": map[string]any{
					"Federated": oidcArn,
				},
				"Action": "sts:AssumeRoleWithWebIdentity",
				"Condition": map[string]any{
					"StringEquals": map[string]any{
						parsedURL.Host + "/oidc:aud": audience,
					},
				},
			},
		},
	}
	trustPolicyJSON, err := json.Marshal(trustPolicy)
	if err != nil {
		return result, cloudsetup.NewSetupError(cloudsetup.ErrorCodeSetupFailed, "failed to create trust policy", err)
	}

	// Create role
	roleArn := fmt.Sprintf("arn:%s:iam::%s:role/%s", partition, accountID, oidcRoleName)
	_, err = c.iamClient.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName:                 aws.String(oidcRoleName),
		AssumeRolePolicyDocument: aws.String(string(trustPolicyJSON)),
	})
	if err != nil {
		if isEntityExists(err) {
			result.Resources = append(result.Resources, cloudsetup.CloudSetupResource{
				Type:   ResourceTypeAWSRole,
				ID:     roleArn,
				Name:   oidcRoleName,
				Status: cloudsetup.ResourceStatusExisting,
			})
		} else {
			return cloudsetup.WrapSetupError(result, ResourceTypeAWSRole, err)
		}
	} else {
		result.Resources = append(result.Resources, cloudsetup.CloudSetupResource{
			Type:   ResourceTypeAWSRole,
			ID:     roleArn,
			Name:   oidcRoleName,
			Status: cloudsetup.ResourceStatusCreated,
		})
	}

	// Attach AdministratorAccess policy
	_, err = c.iamClient.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
		RoleName:  aws.String(oidcRoleName),
		PolicyArn: aws.String(policyArn),
	})
	if err != nil {
		return cloudsetup.WrapSetupError(result, ResourceTypeAWSRolePolicyAttachment, err)
	}

	result.Resources = append(result.Resources, cloudsetup.CloudSetupResource{
		Type:   ResourceTypeAWSRolePolicyAttachment,
		ID:     fmt.Sprintf("%s/%s", roleArn, policyArn),
		Name:   "AccessPolicyAttachment",
		Status: cloudsetup.ResourceStatusCreated,
	})

	result.Success = true

	return result, nil
}

// Helper to check if error is "EntityAlreadyExists"
func isEntityExists(err error) bool {
	var e *types.EntityAlreadyExistsException
	return errors.As(err, &e)
}

// parsePartitionFromARN extracts the AWS partition from an ARN.
// ARN format: arn:partition:service:region:account-id:resource
// Returns "aws" for standard, "aws-us-gov" for GovCloud, "aws-iso" for ISO, etc.
func parsePartitionFromARN(arn string) (string, error) {
	// Split by colons to extract the partition field
	parts := strings.SplitN(arn, ":", 3)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid ARN format: %s", arn)
	}
	if parts[0] != "arn" {
		return "", fmt.Errorf("ARN does not start with 'arn:': %s", arn)
	}
	return parts[1], nil
}

type ThumbprintProvider interface {
	GetCertThumbprint(host string) (string, error)
}

type DefaultThumbprintProvider struct{}

// GetRootCertThumbprint retrieves the SHA1 thumbprint of the root CA certificate
// for the given hostname.
func (DefaultThumbprintProvider) GetCertThumbprint(host string) (string, error) {
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 10 * time.Second},
		"tcp",
		host+":443",
		&tls.Config{ServerName: host},
	)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "", errors.New("no certificates found")
	}

	// Use the top intermediate cert if possible, which should be 2nd to last
	// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_create_oidc_verify-thumbprint.html
	var cert *x509.Certificate
	if len(certs) == 1 {
		cert = certs[0]
	} else {
		cert = certs[len(certs)-2]
	}

	// SHA-1 is required by AWS for certificate thumbprints
	//nolint:gosec // G401: SHA-1 required by AWS OIDC thumbprint API
	sha := sha1.Sum(cert.Raw)
	return hex.EncodeToString(sha[:]), nil
}
