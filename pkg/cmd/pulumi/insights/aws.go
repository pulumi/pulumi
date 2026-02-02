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
	"bufio"
	"context"
	"crypto/sha1" //#nosec G505 -- SHA1 is required for OIDC thumbprint calculation per AWS spec
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const (
	// oidcIssuerURL is the Pulumi OIDC issuer URL.
	oidcIssuerURL = "https://api.pulumi.com/oidc"

	// oidcIssuerHost is the hostname for the Pulumi OIDC provider.
	oidcIssuerHost = "api.pulumi.com"

	// oidcIssuerPath is the path component of the OIDC issuer.
	oidcIssuerPath = "/oidc"

	// readOnlyAccessPolicyARN is the AWS managed ReadOnlyAccess policy.
	readOnlyAccessPolicyARN = "arn:aws:iam::aws:policy/ReadOnlyAccess"

	// defaultRolePrefix is the default prefix for OIDC role names.
	defaultRolePrefix = "pulumi-insights"
)

// defaultRegions are the default AWS regions to scan.
var defaultRegions = []string{"us-east-1", "us-west-2", "eu-west-1"}

// allAWSRegions is a comprehensive list of all AWS regions.
var allAWSRegions = []string{
	"us-east-1", "us-east-2", "us-west-1", "us-west-2",
	"af-south-1",
	"ap-east-1", "ap-south-1", "ap-south-2", "ap-southeast-1", "ap-southeast-2",
	"ap-southeast-3", "ap-southeast-4", "ap-northeast-1", "ap-northeast-2", "ap-northeast-3",
	"ca-central-1", "ca-west-1",
	"eu-central-1", "eu-central-2", "eu-west-1", "eu-west-2", "eu-west-3",
	"eu-south-1", "eu-south-2", "eu-north-1",
	"il-central-1",
	"me-south-1", "me-central-1",
	"sa-east-1",
}

// AWSConfig holds the detected AWS configuration.
type AWSConfig struct {
	Profile         string
	AccountID       string
	Partition       string // aws, aws-cn, aws-us-gov
	Regions         []string
	RoleARN         string
	OIDCProviderARN string
}

// DefaultRoleName returns the default role name for the given account ID.
func (c *AWSConfig) DefaultRoleName() string {
	return fmt.Sprintf("%s-%s", defaultRolePrefix, c.AccountID)
}

// listAWSProfiles reads available profiles from the AWS shared config files.
func listAWSProfiles() ([]string, error) {
	var profiles []string

	// Check ~/.aws/config
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".aws", "config")
	credentialsPath := filepath.Join(homeDir, ".aws", "credentials")

	// Parse profiles from config file
	configProfiles, err := parseAWSConfigProfiles(configPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading AWS config: %w", err)
	}
	profiles = append(profiles, configProfiles...)

	// Parse profiles from credentials file
	credProfiles, err := parseAWSCredentialProfiles(credentialsPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading AWS credentials: %w", err)
	}

	// Merge, avoiding duplicates
	seen := make(map[string]bool)
	for _, p := range profiles {
		seen[p] = true
	}
	for _, p := range credProfiles {
		if !seen[p] {
			profiles = append(profiles, p)
			seen[p] = true
		}
	}

	return profiles, nil
}

// parseAWSConfigProfiles parses profile names from an AWS config file.
// Config file uses [profile name] format (except [default]).
func parseAWSConfigProfiles(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var profiles []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := line[1 : len(line)-1]
			if section == "default" {
				profiles = append(profiles, "default")
			} else if strings.HasPrefix(section, "profile ") {
				profiles = append(profiles, strings.TrimPrefix(section, "profile "))
			}
		}
	}
	return profiles, scanner.Err()
}

// parseAWSCredentialProfiles parses profile names from an AWS credentials file.
// Credentials file uses [name] format directly.
func parseAWSCredentialProfiles(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var profiles []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := line[1 : len(line)-1]
			profiles = append(profiles, section)
		}
	}
	return profiles, scanner.Err()
}

// detectAWSCredentials loads AWS config for the given profile and detects account info.
func detectAWSCredentials(ctx context.Context, profile string) (*AWSConfig, error) {
	var opts []func(*awsconfig.LoadOptions) error
	if profile != "" && profile != "default" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w\n\n"+
			"Ensure AWS credentials are configured:\n"+
			"  1. Run 'aws configure' to set up credentials\n"+
			"  2. Or set AWS_PROFILE environment variable\n"+
			"  3. Or use --profile flag\n\n"+
			"Learn more: https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html", err)
	}

	stsClient := sts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, fmt.Errorf("getting AWS caller identity: %w\n\n"+
			"Ensure your AWS credentials are valid and not expired", err)
	}

	accountID := aws.ToString(identity.Account)
	partition := parsePartitionFromARN(aws.ToString(identity.Arn))

	return &AWSConfig{
		Profile:   profile,
		AccountID: accountID,
		Partition: partition,
		Regions:   defaultRegions,
	}, nil
}

// parsePartitionFromARN extracts the partition from an ARN string.
func parsePartitionFromARN(arn string) string {
	// ARN format: arn:partition:service:region:account:resource
	parts := strings.SplitN(arn, ":", 6)
	if len(parts) >= 2 {
		return parts[1]
	}
	return "aws"
}

// iamClient defines the interface for IAM operations needed for OIDC setup.
// This enables testing with mocks.
type iamClient interface {
	CreateOpenIDConnectProvider(ctx context.Context, params *iam.CreateOpenIDConnectProviderInput,
		optFns ...func(*iam.Options)) (*iam.CreateOpenIDConnectProviderOutput, error)
	AddClientIDToOpenIDConnectProvider(ctx context.Context, params *iam.AddClientIDToOpenIDConnectProviderInput,
		optFns ...func(*iam.Options)) (*iam.AddClientIDToOpenIDConnectProviderOutput, error)
	GetOpenIDConnectProvider(ctx context.Context, params *iam.GetOpenIDConnectProviderInput,
		optFns ...func(*iam.Options)) (*iam.GetOpenIDConnectProviderOutput, error)
	CreateRole(ctx context.Context, params *iam.CreateRoleInput,
		optFns ...func(*iam.Options)) (*iam.CreateRoleOutput, error)
	GetRole(ctx context.Context, params *iam.GetRoleInput,
		optFns ...func(*iam.Options)) (*iam.GetRoleOutput, error)
	AttachRolePolicy(ctx context.Context, params *iam.AttachRolePolicyInput,
		optFns ...func(*iam.Options)) (*iam.AttachRolePolicyOutput, error)
}

// OIDCSetupResult contains the result of OIDC provider and role setup.
type OIDCSetupResult struct {
	OIDCProviderARN string
	OIDCProviderNew bool
	RoleARN         string
	RoleNew         bool
}

// roleExistsError is returned when an IAM role already exists and has a trust policy that doesn't match.
type roleExistsError struct {
	roleName         string
	accountID        string
	audience         string
	currentAudiences []string // Audiences found in the existing trust policy
}

func (e *roleExistsError) Error() string {
	if len(e.currentAudiences) > 0 {
		return fmt.Sprintf("IAM role %q already exists in AWS account %s with audiences: %v",
			e.roleName, e.accountID, e.currentAudiences)
	}
	return fmt.Sprintf("IAM role %q already exists in AWS account %s", e.roleName, e.accountID)
}

// setupOIDC creates or updates the OIDC provider and IAM role for Pulumi Insights.
func setupOIDC(ctx context.Context, iamCli iamClient, cfg *AWSConfig, orgName, roleName string) (*OIDCSetupResult, error) {
	result := &OIDCSetupResult{}

	// Step 1: Calculate OIDC thumbprint
	thumbprint, err := getOIDCThumbprint(oidcIssuerHost)
	if err != nil {
		return nil, fmt.Errorf("calculating OIDC thumbprint: %w", err)
	}

	// Step 2: Create or update OIDC provider
	audience := fmt.Sprintf("aws:%s", orgName)
	providerARN := fmt.Sprintf("arn:%s:iam::%s:oidc-provider/%s%s",
		cfg.Partition, cfg.AccountID, oidcIssuerHost, oidcIssuerPath)

	providerNew, err := createOrUpdateOIDCProvider(ctx, iamCli, providerARN, audience, thumbprint)
	if err != nil {
		return nil, fmt.Errorf("setting up OIDC provider: %w", err)
	}
	result.OIDCProviderARN = providerARN
	result.OIDCProviderNew = providerNew

	// Step 3: Create IAM role with trust policy
	roleARN, roleNew, err := createOIDCRole(ctx, iamCli, cfg, orgName, roleName, providerARN)
	if err != nil {
		return nil, fmt.Errorf("creating IAM role: %w", err)
	}
	result.RoleARN = roleARN
	result.RoleNew = roleNew

	// Step 4: Attach ReadOnlyAccess policy
	policyARN := readOnlyAccessPolicyARN
	// Fix the policy ARN for non-standard partitions
	if cfg.Partition != "aws" {
		policyARN = fmt.Sprintf("arn:%s:iam::aws:policy/ReadOnlyAccess", cfg.Partition)
	}

	if roleNew {
		_, err = iamCli.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
			RoleName:  aws.String(roleName),
			PolicyArn: aws.String(policyARN),
		})
		if err != nil {
			return nil, fmt.Errorf("attaching ReadOnlyAccess policy: %w", err)
		}
	}

	return result, nil
}

// getOIDCThumbprint calculates the TLS certificate thumbprint for the OIDC issuer.
func getOIDCThumbprint(host string) (string, error) {
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 10 * time.Second},
		"tcp",
		host+":443",
		&tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	)
	if err != nil {
		return "", fmt.Errorf("connecting to %s: %w", host, err)
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "", fmt.Errorf("no certificates found for %s", host)
	}

	// Use the root certificate (last in the chain)
	rootCert := certs[len(certs)-1]
	thumbprint := sha1.Sum(rootCert.Raw) //#nosec G401 -- SHA1 is required per AWS OIDC spec
	return fmt.Sprintf("%x", thumbprint), nil
}

// createOrUpdateOIDCProvider creates a new OIDC provider or adds the audience to an existing one.
// Returns true if the provider was newly created.
func createOrUpdateOIDCProvider(
	ctx context.Context, iamCli iamClient, providerARN, audience, thumbprint string,
) (bool, error) {
	_, err := iamCli.CreateOpenIDConnectProvider(ctx, &iam.CreateOpenIDConnectProviderInput{
		Url:            aws.String(oidcIssuerURL),
		ClientIDList:   []string{audience},
		ThumbprintList: []string{thumbprint},
	})
	if err != nil {
		var entityExists *iamtypes.EntityAlreadyExistsException
		if errors.As(err, &entityExists) {
			// Provider exists, check if audience is already configured
			existing, getErr := iamCli.GetOpenIDConnectProvider(ctx, &iam.GetOpenIDConnectProviderInput{
				OpenIDConnectProviderArn: aws.String(providerARN),
			})
			if getErr != nil {
				return false, fmt.Errorf("getting existing OIDC provider: %w", getErr)
			}

			// Check if audience already registered
			for _, clientID := range existing.ClientIDList {
				if clientID == audience {
					return false, nil // Already configured
				}
			}

			// Add the audience
			_, addErr := iamCli.AddClientIDToOpenIDConnectProvider(ctx, &iam.AddClientIDToOpenIDConnectProviderInput{
				OpenIDConnectProviderArn: aws.String(providerARN),
				ClientID:                 aws.String(audience),
			})
			if addErr != nil {
				return false, fmt.Errorf("adding audience to OIDC provider: %w", addErr)
			}
			return false, nil
		}
		return false, fmt.Errorf("creating OIDC provider: %w", err)
	}
	return true, nil
}

// trustPolicyDocument represents an IAM trust policy.
type trustPolicyDocument struct {
	Version   string                 `json:"Version"`
	Statement []trustPolicyStatement `json:"Statement"`
}

type trustPolicyStatement struct {
	Effect    string                 `json:"Effect"`
	Action    string                 `json:"Action"`
	Principal map[string]string      `json:"Principal"`
	Condition map[string]interface{} `json:"Condition,omitempty"`
}

// checkTrustPolicyAudience checks if the URL-encoded trust policy contains the expected audience.
// Returns (hasCorrectAudience, foundAudiences).
func checkTrustPolicyAudience(encodedPolicy *string, expectedAudience string) (bool, []string) {
	if encodedPolicy == nil {
		return false, nil
	}

	// The policy is URL-encoded
	decodedPolicy, err := url.QueryUnescape(*encodedPolicy)
	if err != nil {
		return false, nil
	}

	var policy trustPolicyDocument
	if err := json.Unmarshal([]byte(decodedPolicy), &policy); err != nil {
		return false, nil
	}

	// Check each statement for the correct audience condition
	expectedConditionKey := fmt.Sprintf("%s%s:aud", oidcIssuerHost, oidcIssuerPath)
	var foundAudiences []string
	hasCorrect := false

	for _, stmt := range policy.Statement {
		if stmt.Action != "sts:AssumeRoleWithWebIdentity" {
			continue
		}
		if condition, ok := stmt.Condition["StringEquals"]; ok {
			if condMap, ok := condition.(map[string]interface{}); ok {
				if aud, ok := condMap[expectedConditionKey]; ok {
					if audStr, ok := aud.(string); ok {
						foundAudiences = append(foundAudiences, audStr)
						if audStr == expectedAudience {
							hasCorrect = true
						}
					}
				}
			}
		}
	}
	return hasCorrect, foundAudiences
}

// createOIDCRole creates an IAM role with a trust policy for the OIDC provider.
// Returns the role ARN and whether it was newly created.
func createOIDCRole(
	ctx context.Context, iamCli iamClient, cfg *AWSConfig, orgName, roleName, providerARN string,
) (string, bool, error) {
	audience := fmt.Sprintf("aws:%s", orgName)

	trustPolicy := trustPolicyDocument{
		Version: "2012-10-17",
		Statement: []trustPolicyStatement{
			{
				Effect: "Allow",
				Action: "sts:AssumeRoleWithWebIdentity",
				Principal: map[string]string{
					"Federated": providerARN,
				},
				Condition: map[string]interface{}{
					"StringEquals": map[string]string{
						fmt.Sprintf("%s%s:aud", oidcIssuerHost, oidcIssuerPath): audience,
					},
				},
			},
		},
	}

	trustPolicyJSON, err := json.Marshal(trustPolicy)
	if err != nil {
		return "", false, fmt.Errorf("marshaling trust policy: %w", err)
	}

	output, err := iamCli.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName:                 aws.String(roleName),
		AssumeRolePolicyDocument: aws.String(string(trustPolicyJSON)),
		Description:              aws.String("Pulumi Insights discovery role for read-only cloud resource scanning"),
	})
	if err != nil {
		var entityExists *iamtypes.EntityAlreadyExistsException
		if errors.As(err, &entityExists) {
			// Role already exists - check if the trust policy already has the correct audience
			roleARN := fmt.Sprintf("arn:%s:iam::%s:role/%s", cfg.Partition, cfg.AccountID, roleName)
			getOutput, getErr := iamCli.GetRole(ctx, &iam.GetRoleInput{
				RoleName: aws.String(roleName),
			})
			if getErr != nil {
				return "", false, fmt.Errorf("getting existing role %q: %w", roleName, getErr)
			}

			// Check if the trust policy contains the correct audience
			hasCorrect, foundAudiences := checkTrustPolicyAudience(getOutput.Role.AssumeRolePolicyDocument, audience)
			if hasCorrect {
				// Trust policy is already correct, proceed
				return roleARN, false, nil
			}

			// Trust policy doesn't match - return error for user to decide
			return "", false, &roleExistsError{
				roleName:         roleName,
				accountID:        cfg.AccountID,
				audience:         audience,
				currentAudiences: foundAudiences,
			}
		}
		return "", false, fmt.Errorf("creating IAM role %q: %w", roleName, err)
	}

	return aws.ToString(output.Role.Arn), true, nil
}

// newIAMClient creates a real IAM client from the given profile.
func newIAMClient(ctx context.Context, profile string) (iamClient, error) {
	var opts []func(*awsconfig.LoadOptions) error
	if profile != "" && profile != "default" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config for IAM client: %w", err)
	}

	return iam.NewFromConfig(cfg), nil
}
