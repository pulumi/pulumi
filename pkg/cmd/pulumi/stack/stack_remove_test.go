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

package stack

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStackRemoveHasResourcesError_DefaultSuggestsPulumiDestroy(t *testing.T) {
	t.Parallel()

	err := stackRemoveHasResourcesError("org/project/stack", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "still has resources; removal rejected")
	assert.Contains(t, err.Error(), "Run `pulumi destroy` to delete the resources, then run `pulumi stack rm`")
	assert.NotContains(t, err.Error(), "terraform destroy")
}

func TestStackRemoveHasResourcesError_TerraformCLISuggestsTerraformDestroy(t *testing.T) {
	t.Parallel()

	err := stackRemoveHasResourcesError("org/project/stack", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "still has resources; removal rejected")
	assert.Contains(t, err.Error(), "Run `terraform destroy` to delete the resources, then run `pulumi stack rm`")
	assert.NotContains(t, err.Error(), "pulumi destroy")
}

func TestIsTerraformCLIStack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tags map[apitype.StackTagName]string
		want bool
	}{
		{
			name: "nil tags",
			tags: nil,
			want: false,
		},
		{
			name: "empty tags",
			tags: map[apitype.StackTagName]string{},
			want: false,
		},
		{
			name: "pulumi language runtime",
			tags: map[apitype.StackTagName]string{
				apitype.ProjectRuntimeTag: "nodejs",
			},
			want: false,
		},
		{
			name: "pulumi:runtime terraform-cli",
			tags: map[apitype.StackTagName]string{
				apitype.ProjectRuntimeTag: terraformCLIRuntime,
			},
			want: true,
		},
		{
			name: "terraform:execution-mode local",
			tags: map[apitype.StackTagName]string{
				terraformExecutionModeTag: "local",
			},
			want: true,
		},
		{
			name: "terraform:execution-mode remote",
			tags: map[apitype.StackTagName]string{
				terraformExecutionModeTag: "remote",
			},
			want: true,
		},
		{
			name: "both runtime and execution-mode",
			tags: map[apitype.StackTagName]string{
				apitype.ProjectRuntimeTag: terraformCLIRuntime,
				terraformExecutionModeTag: "remote",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &backend.MockStack{
				TagsF: func() map[apitype.StackTagName]string {
					return tt.tags
				},
			}
			assert.Equal(t, tt.want, isTerraformCLIStack(s))
		})
	}
}
