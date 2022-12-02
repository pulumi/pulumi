// Copyright 2016-2022, Pulumi Corporation.
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

package main

import (
	"context"
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/pulumi/pulumi/sdk/v3/proto/go/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// the main testing function, takes a kms url and tries to make a new secret manager out of it and encrypt and
// decrypt data
func testURL(ctx context.Context, t *testing.T, url string) {
	// Use the given url to initialize a plugin instance
	plugin := NewCloudSecretPlugin()
	initializeResponse, err := plugin.Initialize(ctx, &secrets.InitializeRequest{Args: []string{url}})
	require.NoError(t, err)
	assert.NotNil(t, initializeResponse)
	assert.NotEmpty(t, initializeResponse.State)

	// Try to encrypt a value
	encryptResponse, err := plugin.Encrypt(ctx, &secrets.EncryptRequest{Plaintexts: []string{"plaintext"}})
	require.NoError(t, err)
	assert.Len(t, encryptResponse.Ciphertexts, 1)
	ciphertext := encryptResponse.Ciphertexts[0]

	// Use the given state to then configure a _new_ instance of the plugin
	plugin = NewCloudSecretPlugin()
	configureResponse, err := plugin.Configure(ctx, &secrets.ConfigureRequest{State: initializeResponse.State})
	require.NoError(t, err)
	assert.NotNil(t, configureResponse)

	// And check we can decrypt the ciphertext
	decryptResponse, err := plugin.Decrypt(ctx, &secrets.DecryptRequest{Ciphertexts: []string{ciphertext}})
	require.NoError(t, err)
	assert.Len(t, decryptResponse.Plaintexts, 1)
	assert.Equal(t, "plaintext", decryptResponse.Plaintexts[0])
}

func randomName(t *testing.T) string {
	name := ""
	letters := "abcdefghijklmnopqrstuvwxyz"
	for i := 0; i < 32; i++ {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		require.NoError(t, err)

		char := letters[j.Int64()]
		name = name + string(char)
	}
	return name
}

func getAwsCaller(t *testing.T) (context.Context, aws.Config, *sts.GetCallerIdentityOutput) {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		t.Logf("Skipping, could not load aws config: %s", err)
		t.SkipNow()
	}

	stsClient := sts.NewFromConfig(cfg)
	caller, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		t.Logf("Skipping, couldn't use aws credentials to query identity: %s", err)
		t.SkipNow()
	}

	return ctx, cfg, caller
}

func createKey(ctx context.Context, t *testing.T, cfg aws.Config) *kms.CreateKeyOutput {
	kmsClient := kms.NewFromConfig(cfg)
	keyName := "test-key-" + randomName(t)
	key, err := kmsClient.CreateKey(ctx, &kms.CreateKeyInput{Description: &keyName})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := kmsClient.ScheduleKeyDeletion(ctx, &kms.ScheduleKeyDeletionInput{
			KeyId: key.KeyMetadata.KeyId,
		})
		assert.NoError(t, err)
	})

	return key
}

//nolint:paralleltest // mutates environment variables
func TestAWSCloudManager(t *testing.T) {
	t.Setenv("AWS_REGION", "us-west-2")
	ctx, cfg, _ := getAwsCaller(t)

	key := createKey(ctx, t, cfg)
	url := "awskms://" + *key.KeyMetadata.KeyId + "?awssdk=v2"

	testURL(ctx, t, url)
}

//nolint:paralleltest // mutates environment variables
func TestAWSCloudManager_SessionToken(t *testing.T) {
	t.Setenv("AWS_REGION", "us-west-2")
	ctx, cfg, _ := getAwsCaller(t)

	key := createKey(ctx, t, cfg)
	url := "awskms://" + *key.KeyMetadata.KeyId + "?awssdk=v2"

	creds, err := cfg.Credentials.Retrieve(ctx)
	require.NoError(t, err)

	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_ACCESS_KEY_ID", creds.AccessKeyID)
	t.Setenv("AWS_SECRET_ACCESS_KEY", creds.SecretAccessKey)
	t.Setenv("AWS_SESSION_TOKEN", creds.SessionToken)
	testURL(ctx, t, url)
}

/*
func TestCloudSecretProvider(t *testing.T) {
	t.Parallel()

	awsKmsKeyAlias := os.Getenv("PULUMI_TEST_KMS_KEY_ALIAS")
	if awsKmsKeyAlias == "" {
		t.Skipf("Skipping: PULUMI_TEST_KMS_KEY_ALIAS is not set")
	}

	azureKeyVault := os.Getenv("PULUMI_TEST_AZURE_KEY")
	if azureKeyVault == "" {
		t.Skipf("Skipping: PULUMI_TEST_AZURE_KEY is not set")
	}

	gcpKmsKey := os.Getenv("PULUMI_TEST_GCP_KEY")
	if azureKeyVault == "" {
		t.Skipf("Skipping: PULUMI_TEST_GCP_KEY is not set")
	}

	// Generic test options for all providers
	testOptions := integration.ProgramTestOptions{
		Dir:             "cloud_secrets_provider",
		Dependencies:    []string{"@pulumi/pulumi"},
		SecretsProvider: fmt.Sprintf("awskms://alias/%s", awsKmsKeyAlias),
		Secrets: map[string]string{
			"mysecret": "THISISASECRET",
		},
		ExtraRuntimeValidation: func(t *testing.T, stackInfo integration.RuntimeValidationStackInfo) {
			secretsProvider := stackInfo.Deployment.SecretsProviders
			assert.NotNil(t, secretsProvider)
			assert.Equal(t, secretsProvider.Type, "cloud")

			_, err := cloud.NewCloudSecretsManagerFromState(secretsProvider.State)
			assert.NoError(t, err)

			out, ok := stackInfo.Outputs["out"].(map[string]interface{})
			assert.True(t, ok)

			_, ok = out["ciphertext"]
			assert.True(t, ok)
		},
	}

	localTestOptions := testOptions.With(integration.ProgramTestOptions{
		CloudURL: integration.MakeTempBackend(t),
	})

	azureTestOptions := testOptions.With(integration.ProgramTestOptions{
		SecretsProvider: fmt.Sprintf("azurekeyvault://%s", azureKeyVault),
	})

	gcpTestOptions := testOptions.With(integration.ProgramTestOptions{
		SecretsProvider: fmt.Sprintf("gcpkms://projects/%s", gcpKmsKey),
	})

	// Run with default Pulumi service backend
	t.Run("service", func(t *testing.T) {
		integration.ProgramTest(t, &testOptions)
	})

	// Check Azure secrets provider
	t.Run("azure", func(t *testing.T) { integration.ProgramTest(t, &azureTestOptions) })

	// Check gcloud secrets provider
	t.Run("gcp", func(t *testing.T) { integration.ProgramTest(t, &gcpTestOptions) })

	// Also run with local backend
	t.Run("local", func(t *testing.T) { integration.ProgramTest(t, &localTestOptions) })

}*/
