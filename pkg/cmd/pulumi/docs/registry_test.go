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

package docs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveRegistryPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "package only", args: []string{"aws"}, want: "registry/packages/aws"},
		{name: "install", args: []string{"aws", "install"}, want: "registry/packages/aws/installation-configuration"},
		{
			name: "configuration",
			args: []string{"aws", "configuration"},
			want: "registry/packages/aws/installation-configuration",
		},
		{name: "api", args: []string{"aws", "api"}, want: "registry/packages/aws/api-docs"},
		{name: "api with module", args: []string{"aws", "api", "s3"}, want: "registry/packages/aws/api-docs/s3"},
		{name: "unknown subpage", args: []string{"aws", "changelog"}, want: "registry/packages/aws/changelog"},
		{name: "api-docs alias", args: []string{"aws", "api-docs"}, want: "registry/packages/aws/api-docs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, resolveRegistryPath(tt.args))
		})
	}
}
