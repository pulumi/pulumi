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

package oss

import (
	"net/url"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob"
)

func TestSchemeIsRegistered(t *testing.T) {
	t.Parallel()

	assert.True(t, blob.DefaultURLMux().ValidBucketScheme(OSSScheme))
}

func TestOSSEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		region string
		want   string
	}{
		{"cn-hangzhou", "https://s3.oss-cn-hangzhou.aliyuncs.com"},
		{"us-west-1", "https://s3.oss-us-west-1.aliyuncs.com"},
		// An oss- prefixed region is normalized so we don't double the prefix.
		{"oss-cn-hangzhou", "https://s3.oss-cn-hangzhou.aliyuncs.com"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, ossEndpoint(tt.region), "region %q", tt.region)
	}
}

// Not parallel: uses t.Setenv to supply static credentials, which cannot be
// combined with parallel subtests.
//
//nolint:paralleltest
func TestConfigFromURL(t *testing.T) {
	// Provide static credentials so LoadDefaultConfig doesn't probe the
	// environment/metadata service.
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_ID", "test-id")
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "test-secret")
	t.Setenv("ALIBABA_CLOUD_SECURITY_TOKEN", "test-token")

	t.Run("region derives endpoint", func(t *testing.T) {
		u, err := url.Parse("oss://my-bucket?region=cn-hangzhou")
		require.NoError(t, err)

		cfg, err := configFromURL(t.Context(), u)
		require.NoError(t, err)

		require.NotNil(t, cfg.BaseEndpoint)
		assert.Equal(t, "https://s3.oss-cn-hangzhou.aliyuncs.com", *cfg.BaseEndpoint)
		assert.Equal(t, "cn-hangzhou", cfg.Region)
		assert.Equal(t, aws.RequestChecksumCalculationWhenRequired, cfg.RequestChecksumCalculation)

		creds, err := cfg.Credentials.Retrieve(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "test-id", creds.AccessKeyID)
		assert.Equal(t, "test-secret", creds.SecretAccessKey)
		assert.Equal(t, "test-token", creds.SessionToken)
	})

	t.Run("explicit endpoint overrides region-derived one", func(t *testing.T) {
		u, err := url.Parse(
			"oss://my-bucket?region=cn-hangzhou&endpoint=https://s3.oss-cn-hangzhou-internal.aliyuncs.com")
		require.NoError(t, err)

		cfg, err := configFromURL(t.Context(), u)
		require.NoError(t, err)

		require.NotNil(t, cfg.BaseEndpoint)
		assert.Equal(t, "https://s3.oss-cn-hangzhou-internal.aliyuncs.com", *cfg.BaseEndpoint)
	})

	t.Run("missing region and endpoint errors", func(t *testing.T) {
		u, err := url.Parse("oss://my-bucket")
		require.NoError(t, err)

		_, err = configFromURL(t.Context(), u)
		assert.ErrorContains(t, err, "region")
	})

	t.Run("missing bucket errors", func(t *testing.T) {
		u, err := url.Parse("oss://?region=cn-hangzhou")
		require.NoError(t, err)

		_, err = configFromURL(t.Context(), u)
		assert.ErrorContains(t, err, "bucket")
	})
}
