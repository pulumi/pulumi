// Copyright 2016-2024, Pulumi Corporation.
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
	"context"
	"iter"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/pkg/backend"
	"github.com/pulumi/pulumi/sdk/v3/pkg/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/pkg/util/testutil"
	pkgWorkspace "github.com/pulumi/pulumi/sdk/v3/pkg/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // changes directory
func TestErrorsOnNonHTTPBackend(t *testing.T) {
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)
	testutil.MockBackendInstance(t, &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, org string, name string) (bool, error) {
			return name == projectName, nil
		},
		NameF:	func() string { return "mock" },
		GetReadOnlyCloudRegistryF: func() registry.Registry {
			return &backend.MockCloudRegistry{
				ListTemplatesF: func(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error] {
					return func(yield func(apitype.TemplateMetadata, error) bool) {}
				},
			}
		},
	})

	testNewArgs := newArgs{
		aiPrompt:		"prompt",
		aiLanguage:		"typescript",
		interactive:		true,
		secretsProvider:	"default",
		promptForAIProjectURL:	promptForAIProjectURL,
	}

	assert.ErrorContains(t,
		runNew(
			context.Background(), testNewArgs,
		),
		"please log in to Pulumi Cloud to use Pulumi AI")
}

//nolint:paralleltest // changes directory for process, mocks backendInstance
func TestGeneratingProjectWithAIPromptSucceeds(t *testing.T) {
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	listTemplates := func(ctx context.Context, name *string) iter.Seq2[apitype.TemplateMetadata, error] {
		assert.Nil(t, name)
		return func(yield func(apitype.TemplateMetadata, error) bool) {
			if !yield(apitype.TemplateMetadata{
				Name:		"name1",
				Publisher:	"publisher1",
				Source:		"source1",
			}, nil) {
				return
			}
		}
	}

	testutil.MockBackendInstance(t, &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, org string, name string) (bool, error) {
			return true, nil
		},
		NameF:	func() string { return "mock" },
		GetReadOnlyCloudRegistryF: func() registry.Registry {
			return &backend.MockCloudRegistry{
				ListTemplatesF: listTemplates,
			}
		},
	})

	// Generate-only command is not creating any stacks, so don't bother with with the name uniqueness check.
	args := newArgs{
		generateOnly:	true,
		interactive:	true,
		prompt:		promptMock(projectName, ""),
		promptForAIProjectURL: func(ctx context.Context, ws pkgWorkspace.Context,
			args newArgs, opts display.Options,
		) (string, error) {
			// Return a plain template name so that we don't have to rely or a hard-coded AI path.
			// This has the same effect and is good enough for the mock-based testing.
			return "typescript", nil
		},
		secretsProvider:	"default",
		templateNameOrURL:	"",	// <-- must be empty to trigger the AI flow
	}

	err := runNew(context.Background(), args)
	require.NoError(t, err)

	proj := loadProject(t, tempdir)
	require.Equal(t, projectName, proj.Name.String())
}
