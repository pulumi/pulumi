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

package neo

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func TestIsUpgradeRequired(t *testing.T) {
	t.Parallel()

	t.Run("DirectErrorResponse", func(t *testing.T) {
		t.Parallel()
		err := &apitype.ErrorResponse{Code: http.StatusUpgradeRequired, Message: "upgrade"}
		assert.True(t, isUpgradeRequired(err))
	})

	t.Run("WrappedErrorResponse", func(t *testing.T) {
		t.Parallel()
		// The neo client wraps with fmt.Errorf("creating Neo task: %w", err), so a
		// wrapped form must still be detected.
		inner := &apitype.ErrorResponse{Code: http.StatusUpgradeRequired, Message: "upgrade"}
		err := fmt.Errorf("creating Neo task: %w", inner)
		assert.True(t, isUpgradeRequired(err))
	})

	t.Run("OtherStatusReturnsFalse", func(t *testing.T) {
		t.Parallel()
		err := &apitype.ErrorResponse{Code: http.StatusBadRequest, Message: "bad"}
		assert.False(t, isUpgradeRequired(err))
	})

	t.Run("NonErrorResponseReturnsFalse", func(t *testing.T) {
		t.Parallel()
		assert.False(t, isUpgradeRequired(errors.New("network down")))
	})

	t.Run("NilReturnsFalse", func(t *testing.T) {
		t.Parallel()
		assert.False(t, isUpgradeRequired(nil))
	})
}

//nolint:paralleltest // mutates package-global upgradeWarnOnce
func TestWarnUpgradeRequired(t *testing.T) {
	t.Run("EmitsOneUIWarning", func(t *testing.T) {
		resetUpgradeWarnOnceForTest()

		ch := make(chan UIEvent, 4)
		err := &apitype.ErrorResponse{Code: http.StatusUpgradeRequired}

		require.True(t, warnUpgradeRequired(err, ch))
		// A second call must not enqueue another warning — the user gets a single
		// signal even if multiple endpoints return 426 in the same session.
		require.True(t, warnUpgradeRequired(err, ch))

		got := drainUIEvents(ch)
		require.Len(t, got, 1)
		warn, ok := got[0].(UIWarning)
		require.True(t, ok)
		assert.Contains(t, warn.Message, "Pulumi CLI is out of date")
	})

	t.Run("ReturnsFalseForNon426", func(t *testing.T) {
		resetUpgradeWarnOnceForTest()

		ch := make(chan UIEvent, 1)
		assert.False(t, warnUpgradeRequired(errors.New("network"), ch))
		assert.False(t, warnUpgradeRequired(&apitype.ErrorResponse{Code: 401}, ch))
		assert.Empty(t, drainUIEvents(ch))
	})
}
