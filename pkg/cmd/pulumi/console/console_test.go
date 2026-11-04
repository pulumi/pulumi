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

package console

import (
	"bytes"
	"context"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockLoginManager(be backend.Backend) *cmdBackend.MockLoginManager {
	return &cmdBackend.MockLoginManager{
		LoginF: func(
			context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool, bool, colors.Colorization,
		) (backend.Backend, error) {
			return be, nil
		},
	}
}

// Regression test for https://github.com/pulumi/pulumi/issues/25142. The stack URL is printed even when
// no browser is opened, which is the case in a non-interactive session.
func TestConsoleCmd_printsStackURL(t *testing.T) { //nolint:paralleltest
	prev := cmdutil.DisableInteractive
	cmdutil.DisableInteractive = true
	t.Cleanup(func() { cmdutil.DisableInteractive = prev })

	const url = "https://app.pulumi.com/org/proj/stk"
	be := &httpstate.MockHTTPBackend{
		MockBackend: backend.MockBackend{
			GetStackF: func(_ context.Context, ref backend.StackReference) (backend.Stack, error) {
				return &backend.MockStack{
					RefF: func() backend.StackReference { return ref },
				}, nil
			},
		},
		FStackConsoleURL: func(ref backend.StackReference) (string, error) {
			assert.Equal(t, "org/proj/stk", ref.String())
			return url, nil
		},
	}

	var buff bytes.Buffer
	cmd := NewConsoleCmd(&pkgWorkspace.MockContext{}, newMockLoginManager(be))
	cmd.SetArgs([]string{"--stack", "org/proj/stk"})
	cmd.SetOut(&buff)
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Equal(t, url+"\n", buff.String())
}

// Regression test for https://github.com/pulumi/pulumi/issues/25129. Backends other than Pulumi Cloud have
// no console, so the command fails rather than exiting successfully.
func TestConsoleCmd_nonCloudBackend(t *testing.T) {
	t.Parallel()

	cmd := NewConsoleCmd(&pkgWorkspace.MockContext{}, newMockLoginManager(&backend.MockBackend{}))
	err := cmd.Execute()
	assert.ErrorContains(t, err, "this command is not available for your backend")
	assert.ErrorContains(t, err, "https://www.pulumi.com/docs/iac/concepts/state-and-backends/#pulumi-cloud-backend")
}
