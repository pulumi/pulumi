// Copyright 2016, Pulumi Corporation.
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

package logs

import (
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestParseSince(t *testing.T) {
	t.Parallel()

	a, _ := parseSince("", time.Now())
	assert.Nil(t, a)

	now := time.Now().UTC()
	b, _ := parseSince("1m30s", now)
	assert.True(t, b.UnixNano() < now.UnixNano())
	fmt.Printf("Res: %v\n", b)

	c, _ := parseSince("2006-01-02T15:04:05", time.Now().UTC())
	assert.Equal(t, "2006-01-02T15:04:05Z", c.UTC().Format(time.RFC3339))

	d, _ := parseSince("2006-01-02", time.Now().UTC())
	assert.Equal(t, "2006-01-02T00:00:00Z", d.UTC().Format(time.RFC3339))

	pst, err := time.LoadLocation("America/Los_Angeles")
	require.NoError(t, err)

	e, _ := parseSince("2006-01-02T15:04:05-08:00", time.Now().In(pst))
	assert.Equal(t, "2006-01-02T15:04:05-08:00", e.In(pst).Format(time.RFC3339))

	f, _ := parseSince("2006-01-02-08:00", time.Now().In(pst))
	assert.Equal(t, "2006-01-02T00:00:00-08:00", f.In(pst).Format(time.RFC3339))
}

func TestLogsRemovalError(t *testing.T) {
	errReadProject := errors.New("read project")
	run := func() error {
		cmd := NewLogsCmd(&pkgWorkspace.MockContext{
			ReadProjectF: func(string) (*workspace.Project, string, error) {
				return nil, "", errReadProject
			},
		})
		cmd.SetArgs(nil)
		cmd.SetErr(io.Discard)
		cmd.SetOut(io.Discard)
		return cmd.Execute()
	}

	t.Run("default", func(t *testing.T) {
		t.Setenv(env.DisableExperimentalLogsRemovalError.Var().Name(), "false")
		assert.EqualError(t, run(), "command \"logs\" is deprecated and disabled by default.\n\n"+
			"This command will be removed in a future version of pulumi.\n\n"+
			"The removal is being tracked in https://github.com/pulumi/pulumi/issues/24718. If you believe that \n"+
			"`pulumi logs` should not be removed, please comment on the issue explaining why.\n\n"+
			"To run the command until it is removed, set PULUMI_DISABLE_EXPERIMENTAL_LOGS_REMOVAL_ERROR=1.")
	})

	t.Run("disabled", func(t *testing.T) {
		t.Setenv(env.DisableExperimentalLogsRemovalError.Var().Name(), "true")
		assert.ErrorIs(t, run(), errReadProject)
	})
}
