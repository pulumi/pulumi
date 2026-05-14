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

package newcmd

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendTemplateEnvironments_NoExistingEnvironment(t *testing.T) {
	t.Parallel()

	ps := &workspace.ProjectStack{}
	added := appendTemplateEnvironments(ps, []string{"infra/aws-prod-creds", "infra/shared-tags"})

	assert.Equal(t, []string{"infra/aws-prod-creds", "infra/shared-tags"}, added)
	require.NotNil(t, ps.Environment)
	assert.Equal(t, []string{"infra/aws-prod-creds", "infra/shared-tags"}, ps.Environment.Imports())
}

func TestAppendTemplateEnvironments_AppendsToExisting(t *testing.T) {
	t.Parallel()

	ps := &workspace.ProjectStack{
		Environment: workspace.NewEnvironment([]string{"team/base"}),
	}
	added := appendTemplateEnvironments(ps, []string{"infra/aws-prod-creds"})

	assert.Equal(t, []string{"infra/aws-prod-creds"}, added)
	assert.Equal(t, []string{"team/base", "infra/aws-prod-creds"}, ps.Environment.Imports())
}

func TestAppendTemplateEnvironments_DedupesAgainstExisting(t *testing.T) {
	t.Parallel()

	ps := &workspace.ProjectStack{
		Environment: workspace.NewEnvironment([]string{"infra/aws-prod-creds"}),
	}
	added := appendTemplateEnvironments(ps, []string{"infra/aws-prod-creds", "infra/shared-tags"})

	assert.Equal(t, []string{"infra/shared-tags"}, added)
	assert.Equal(t, []string{"infra/aws-prod-creds", "infra/shared-tags"}, ps.Environment.Imports())
}

func TestAppendTemplateEnvironments_EmptyInputIsNoOp(t *testing.T) {
	t.Parallel()

	ps := &workspace.ProjectStack{}
	added := appendTemplateEnvironments(ps, nil)

	assert.Empty(t, added)
	assert.Nil(t, ps.Environment)
}

func TestAppendTemplateEnvironments_AllDuplicatesIsNoOp(t *testing.T) {
	t.Parallel()

	ps := &workspace.ProjectStack{
		Environment: workspace.NewEnvironment([]string{"infra/aws-prod-creds"}),
	}
	added := appendTemplateEnvironments(ps, []string{"infra/aws-prod-creds"})

	assert.Empty(t, added)
	assert.Equal(t, []string{"infra/aws-prod-creds"}, ps.Environment.Imports())
}
