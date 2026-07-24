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

package adder

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Backend resolves at most once per execution, no matter how many times it is
// asked for.
func TestBackendMemoized(t *testing.T) {
	t.Parallel()

	logins := 0
	be := &backend.MockBackend{}
	s := Environment{
		WS: &pkgWorkspace.MockContext{},
		LM: &cmdBackend.MockLoginManager{
			LoginF: func(
				context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool, bool, colors.Colorization,
			) (backend.Backend, error) {
				logins++
				return be, nil
			},
		},
	}

	cmd := &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, args []string) error {
			b1, err := s.Backend(cmd)
			require.NoError(t, err)
			b2, err := s.Backend(cmd)
			require.NoError(t, err)
			assert.Same(t, b1, b2)
			return nil
		},
	}
	cmd.SetContext(WithBag(t.Context()))

	require.NoError(t, cmd.Execute())
	assert.Equal(t, 1, logins)
}

// StackFlag registration is idempotent and its usage configuration is order
// independent: a custom usage wins whether it is declared before or after a
// default declaration.
func TestStackFlagCommutes(t *testing.T) {
	t.Parallel()

	customBefore := &cobra.Command{Use: "before"}
	StackFlag(customBefore, "custom usage")
	StackFlag(customBefore, "")

	customAfter := &cobra.Command{Use: "after"}
	StackFlag(customAfter, "")
	StackFlag(customAfter, "custom usage")

	for _, cmd := range []*cobra.Command{customBefore, customAfter} {
		f := cmd.PersistentFlags().Lookup("stack")
		require.NotNil(t, f)
		assert.Equal(t, "custom usage", f.Usage)
	}
}
