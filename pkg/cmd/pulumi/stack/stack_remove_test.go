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
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStackRemoveHasResourcesError_DefaultSuggestsPulumiDestroy(t *testing.T) {
	t.Parallel()

	err := stackRemoveHasResourcesError("org/project/stack", "nodejs")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "still has resources; removal rejected")
	assert.Contains(t, err.Error(), "Run `pulumi destroy` to delete the resources, then run `pulumi stack rm`")
	assert.NotContains(t, err.Error(), "terraform destroy")
}

func TestStackRemoveHasResourcesError_TerraformCLISuggestsTerraformDestroy(t *testing.T) {
	t.Parallel()

	err := stackRemoveHasResourcesError("org/project/stack", terraformCLIRuntime)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "still has resources; removal rejected")
	assert.Contains(t, err.Error(), "Run `terraform destroy` to delete the resources, then run `pulumi stack rm`")
	assert.NotContains(t, err.Error(), "pulumi destroy")
}

func TestStackRuntimeName_FromTags(t *testing.T) {
	t.Parallel()

	s := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				StringV:  "org/project/stack",
				NameV:    tokens.MustParseStackName("stack"),
				ProjectV: "project",
			}
		},
		TagsF: func() map[apitype.StackTagName]string {
			return map[apitype.StackTagName]string{
				apitype.ProjectRuntimeTag: terraformCLIRuntime,
			}
		},
	}

	assert.Equal(t, terraformCLIRuntime, stackRuntimeName(s))
}
