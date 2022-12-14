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

package cloud

import (
	"context"
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/stretchr/testify/assert"
)

func assertNoError(t *testing.T, err error) {
	if !assert.NoError(t, err) {
		t.FailNow()
	}
}

// the main testing function, takes a kms url and tries to make a new secret manager out of it and encrypt and
// decrypt data
func testURL(ctx context.Context, t *testing.T, url string) {
	dataKey, err := GenerateNewDataKey(url)
	assertNoError(t, err)

	manager, err := NewCloudSecretsManager(url, dataKey)
	assertNoError(t, err)

	enc, err := manager.Encrypter()
	assertNoError(t, err)

	dec, err := manager.Decrypter()
	assertNoError(t, err)

	ciphertext, err := enc.EncryptValue(ctx, "plaintext")
	assertNoError(t, err)

	plaintext, err := dec.DecryptValue(ctx, ciphertext)
	assertNoError(t, err)
	assert.Equal(t, "plaintext", plaintext)
}

func randomName(t *testing.T) string {
	name := ""
	letters := "abcdefghijklmnopqrstuvwxyz"
	for i := 0; i < 32; i++ {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		assertNoError(t, err)

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
	assertNoError(t, err)
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
	assertNoError(t, err)

	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_ACCESS_KEY_ID", creds.AccessKeyID)
	t.Setenv("AWS_SECRET_ACCESS_KEY", creds.SecretAccessKey)
	t.Setenv("AWS_SESSION_TOKEN", creds.SessionToken)
	testURL(ctx, t, url)
}
