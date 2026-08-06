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
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

func TestConfirmDefaultsRendersBlock(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	opts := display.Options{Color: colors.Never, Stdout: &out}
	sel, offered := scriptedSelect(t, confirmYes)

	ok, err := confirmDefaults([]field{
		{"Project name", "my-project"},
		{"Description", "A minimal AWS TypeScript Pulumi program"},
		{"Stack name", "my-org/dev"},
	}, []field{{"aws:region", "us-east-1"}}, opts, sel)
	require.NoError(t, err)
	assert.True(t, ok)

	assert.Equal(t,
		"\n"+
			"Project name:  my-project\n"+
			"Description:   A minimal AWS TypeScript Pulumi program\n"+
			"Stack name:    my-org/dev\n"+
			"Config:\n"+
			"  aws:region:  us-east-1\n",
		out.String())
	require.Len(t, *offered, 1)
	assert.Equal(t, []string{confirmYes, confirmChange}, (*offered)[0])
}

func TestConfirmDefaultsOmitsEmptyConfig(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	opts := display.Options{Color: colors.Never, Stdout: &out}
	sel, _ := scriptedSelect(t, confirmChange)

	ok, err := confirmDefaults([]field{{"Project name", "p"}}, nil, opts, sel)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.NotContains(t, out.String(), "Config:")
}
