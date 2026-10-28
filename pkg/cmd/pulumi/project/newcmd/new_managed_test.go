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
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

//nolint:paralleltest // changes directory for process, mocks login manager
func TestNewExistingManagedStackFailsBeforeCopy(t *testing.T) {
	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	b, created := guidedNewTestBackend(t)
	b.GetStackF = func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
		return &backend.MockStack{
			RefF:  func() backend.StackReference { return ref },
			TagsF: func() map[apitype.StackTagName]string { return nil },
			ConfigLocationF: func() backend.StackConfigLocation {
				return backend.StackConfigLocation{ManagedEnvironment: "proj/dev"}
			},
		}, nil
	}
	mockCurrentBackend(t, b)

	err := runNew(t.Context(), newArgs{
		yes:               true,
		prompt:            promptMock("proj", "dev"),
		secretsProvider:   "default",
		stack:             "my-org/proj/dev",
		templateNameOrURL: localTemplate(t),
		languageTemplate:  languageTemplateMock,
	})

	require.EqualError(t, err,
		"pulumi new --stack is not supported on stacks with an ESC-managed environment yet")
	assert.NoFileExists(t, filepath.Join(tempdir, "Pulumi.yaml"))
	assert.Empty(t, *created)
}
