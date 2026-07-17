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

package cancel

import (
	"bytes"
	"context"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/adder"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testEnvironment resolves --stack against a mock backend and records the
// stack reference each CancelCurrentUpdate is called with.
func testEnvironment(canceled *[]string) adder.Environment {
	var be *backend.MockBackend
	be = &backend.MockBackend{
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				StringV: s,
				NameV:   tokens.MustParseStackName(s),
			}, nil
		},
		GetStackF: func(_ context.Context, ref backend.StackReference) (backend.Stack, error) {
			return &backend.MockStack{
				RefF:     func() backend.StackReference { return ref },
				BackendF: func() backend.Backend { return be },
			}, nil
		},
		CancelCurrentUpdateF: func(_ context.Context, ref backend.StackReference) error {
			*canceled = append(*canceled, ref.String())
			return nil
		},
	}
	return adder.Environment{
		WS: &pkgWorkspace.MockContext{},
		LM: &cmdBackend.MockLoginManager{
			LoginF: func(
				context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool, bool, colors.Colorization,
			) (backend.Backend, error) {
				return be, nil
			},
		},
	}
}

func TestCancelStackFlag(t *testing.T) {
	t.Parallel()

	var canceled []string
	cmd := NewCancelCmd(testEnvironment(&canceled))
	cmd.SetContext(adder.WithBag(t.Context()))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--stack", "dev", "--yes"})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, []string{"dev"}, canceled)
}

func TestCancelStackArgument(t *testing.T) {
	t.Parallel()

	var canceled []string
	cmd := NewCancelCmd(testEnvironment(&canceled))
	cmd.SetContext(adder.WithBag(t.Context()))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"prod", "--yes"})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, []string{"prod"}, canceled)
}

func TestCancelStackFlagAndArgumentConflict(t *testing.T) {
	t.Parallel()

	var canceled []string
	cmd := NewCancelCmd(testEnvironment(&canceled))
	cmd.SetContext(adder.WithBag(t.Context()))
	cmd.SetArgs([]string{"prod", "--stack", "dev", "--yes"})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.EqualError(t, err, "only one of --stack or argument stack name may be specified, not both")
	assert.Empty(t, canceled)
}
