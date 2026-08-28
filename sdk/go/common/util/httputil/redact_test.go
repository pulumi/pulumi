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

package httputil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "postgres dsn password",
			in:   "postgres://user:hunter2@db.example.com:5432/pulumi",
			want: "postgres://user:xxxxx@db.example.com:5432/pulumi",
		},
		{
			name: "git pat in userinfo",
			in:   "https://x-access-token:ghp_secret@github.com/pulumi/pulumi.git",
			want: "https://x-access-token:xxxxx@github.com/pulumi/pulumi.git",
		},
		{
			name: "presigned query values",
			in:   "https://bucket.s3.amazonaws.com/p.tgz?X-Amz-Signature=deadbeef&X-Amz-Credential=AKIA",
			want: "https://bucket.s3.amazonaws.com/p.tgz?X-Amz-Credential=redacted&X-Amz-Signature=redacted",
		},
		{
			name: "benign query preserved, sensitive redacted",
			in:   "azblob://container?sv=2021&sig=abc123",
			want: "azblob://container?sig=redacted&sv=2021",
		},
		{
			name: "no credentials unchanged",
			in:   "https://api.pulumi.com",
			want: "https://api.pulumi.com",
		},
		{
			name: "unparseable",
			in:   "http://foo.com/%",
			want: "[redacted url]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, RedactURL(tt.in))
		})
	}
}

func TestURLSecrets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"password extracted", "postgres://user:hunter2@db/pulumi", []string{"hunter2"}},
		{"git pat as password", "https://x-access-token:ghp_secret@github.com/x", []string{"ghp_secret"}},
		{"azblob sas signature", "azblob://container?sv=2021&sp=r&sig=abc123def", []string{"abc123def"}},
		{"benign query not extracted", "s3://bucket?region=us-west-2&profile=dev", nil},
		{"no userinfo", "https://api.pulumi.com", nil},
		{"username only, no password", "https://ghp_tok@github.com/x", nil},
		{"unparseable", "http://foo.com/%", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.ElementsMatch(t, tt.want, URLSecrets(tt.in))
		})
	}
}
