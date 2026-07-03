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

package diy

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranslateLegacyS3Params(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		give string
		// wantQuery is the expected decoded query; nil means the URL must be
		// returned unchanged.
		wantQuery url.Values
	}{
		{
			name: "no query",
			give: "s3://bucket",
		},
		{
			name: "v2 params untouched",
			give: "s3://bucket?endpoint=https://example.com&s3ForcePathStyle=true&awssdk=v2",
		},
		{
			name: "disableSSL translated",
			give: "s3://bucket?endpoint=https://minio:9000&disableSSL=true",
			wantQuery: url.Values{
				"endpoint":      {"https://minio:9000"},
				"disable_https": {"true"},
			},
		},
		{
			name: "disableSSL false translated",
			give: "s3://bucket?disableSSL=false",
			wantQuery: url.Values{
				"disable_https": {"false"},
			},
		},
		{
			name: "bare endpoint gets https scheme",
			give: "s3://bucket?endpoint=minio:9000",
			wantQuery: url.Values{
				"endpoint": {"https://minio:9000"},
			},
		},
		{
			name: "bare endpoint with disableSSL gets http scheme",
			give: "s3://bucket?endpoint=minio:9000&disableSSL=true&s3ForcePathStyle=true",
			wantQuery: url.Values{
				"endpoint":         {"http://minio:9000"},
				"disable_https":    {"true"},
				"s3ForcePathStyle": {"true"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := translateLegacyS3Params(tt.give)
			require.NoError(t, err)
			if tt.wantQuery == nil {
				assert.Equal(t, tt.give, got)
				return
			}
			u, err := url.Parse(got)
			require.NoError(t, err)
			assert.Equal(t, "s3", u.Scheme)
			assert.Equal(t, "bucket", u.Host)
			assert.Equal(t, tt.wantQuery, u.Query())
		})
	}

	t.Run("invalid disableSSL value", func(t *testing.T) {
		t.Parallel()

		_, err := translateLegacyS3Params("s3://bucket?disableSSL=banana")
		assert.ErrorContains(t, err, `invalid value for query parameter "disableSSL"`)
	})
}
