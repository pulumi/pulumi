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

package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

func TestConfirmDeletion(t *testing.T) {
	t.Parallel()

	t.Run("yes skips the prompt even when interactive", func(t *testing.T) {
		t.Parallel()
		var w bytes.Buffer
		err := ConfirmDeletion(true /*yes*/, true /*interactive*/, "prompt", "my-thing", &w,
			display.Options{Stdin: failingReader{t}})
		require.NoError(t, err)
		assert.Empty(t, w.String())
	})

	t.Run("non-interactive without yes requires yes", func(t *testing.T) {
		t.Parallel()
		var w bytes.Buffer
		err := ConfirmDeletion(false /*yes*/, false /*interactive*/, "prompt", "my-thing", &w,
			display.Options{Stdin: failingReader{t}})
		assert.ErrorIs(t, err, backenderr.ErrNonInteractiveRequiresYes)
		assert.Empty(t, w.String())
	})

	t.Run("interactive proceeds when the user retypes the value", func(t *testing.T) {
		t.Parallel()
		var w, out bytes.Buffer
		err := ConfirmDeletion(false /*yes*/, true /*interactive*/, "prompt", "my-thing", &w,
			display.Options{Color: colors.Never, Stdin: strings.NewReader("my-thing\n"), Stdout: &out})
		require.NoError(t, err)
		assert.Empty(t, w.String())
	})

	t.Run("interactive bails when the value does not match", func(t *testing.T) {
		t.Parallel()
		var w, out bytes.Buffer
		err := ConfirmDeletion(false /*yes*/, true /*interactive*/, "prompt", "my-thing", &w,
			display.Options{Color: colors.Never, Stdin: strings.NewReader("nope\n"), Stdout: &out})
		require.Error(t, err)
		assert.True(t, result.IsBail(err), "declining must return a bail error")
		assert.Contains(t, w.String(), "confirmation declined")
	})
}

// failingReader fails the test if anything tries to read from it
type failingReader struct{ t *testing.T }

func (r failingReader) Read([]byte) (int, error) {
	r.t.Helper()
	r.t.Fatal("stdin should not be read")
	return 0, nil
}
