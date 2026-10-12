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

package operations

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestAutonamingStackContext_NonHTTPStateStack(t *testing.T) {
	t.Parallel()

	// A plain backend.Stack (not the cloud httpstate.Stack) cannot supply an
	// org name — autonamingStackContext must fall back to the placeholder
	// "organization" so autonaming config still loads.
	proj := &workspace.Project{Name: tokens.PackageName("my-proj")}
	stk := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{NameV: tokens.MustParseStackName("dev")}
		},
	}

	got := autonamingStackContext(proj, stk)
	assert.Equal(t, "organization", got.Organization)
	assert.Equal(t, "my-proj", got.Project)
	assert.Equal(t, "dev", got.Stack)
}

func TestAutonamingStackContext_HTTPStateStackUsesOrgName(t *testing.T) {
	t.Parallel()

	// httpstate.Stack callers expose the real organization via OrgName(); that
	// value must propagate into the autonaming context.
	proj := &workspace.Project{Name: tokens.PackageName("my-proj")}
	stk := &fakeHTTPStack{
		MockStack: &backend.MockStack{
			RefF: func() backend.StackReference {
				return &backend.MockStackReference{NameV: tokens.MustParseStackName("prod")}
			},
		},
		orgName: "my-org",
	}

	got := autonamingStackContext(proj, stk)
	assert.Equal(t, "my-org", got.Organization)
	assert.Equal(t, "my-proj", got.Project)
	assert.Equal(t, "prod", got.Stack)
}

// fakeHTTPStack satisfies httpstate.Stack so tests can exercise the
// type-assertion branch in autonamingStackContext. Methods other than the
// two cloud-only ones delegate to the embedded MockStack.
type fakeHTTPStack struct {
	*backend.MockStack
	orgName string
}

func (f *fakeHTTPStack) OrgName() string {
	return f.orgName
}

func (f *fakeHTTPStack) CurrentOperation() *apitype.OperationStatus {
	return nil
}

func (f *fakeHTTPStack) StackIdentifier() client.StackIdentifier {
	return client.StackIdentifier{}
}

// Compile-time assertion: fakeHTTPStack must satisfy httpstate.Stack so the
// type assertion in autonamingStackContext succeeds.
var _ httpstate.Stack = (*fakeHTTPStack)(nil)
