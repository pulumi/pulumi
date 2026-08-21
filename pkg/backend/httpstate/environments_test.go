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

package httpstate

import (
	"errors"
	"net/http"
	"testing"

	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func TestTranslateConfigConflict(t *testing.T) {
	t.Parallel()

	conflict := &client.EnvironmentErrorResponse{Code: http.StatusConflict, Message: "modified"}
	translated := translateConfigConflict(conflict)
	require.ErrorIs(t, translated, backend.ErrConfigConflict)
	require.ErrorIs(t, translated, conflict, "the original esc error stays in the chain for context")

	// A 409 with an empty or non-JSON body surfaces as *apitype.ErrorResponse with the HTTP status,
	// not *EnvironmentErrorResponse, so it must still be recognized as a conflict.
	apiConflict := &apitype.ErrorResponse{Code: http.StatusConflict, Message: "conflict"}
	require.ErrorIs(t, translateConfigConflict(apiConflict), backend.ErrConfigConflict)

	other := &client.EnvironmentErrorResponse{Code: http.StatusBadRequest, Message: "bad request"}
	require.NotErrorIs(t, translateConfigConflict(other), backend.ErrConfigConflict)

	apiOther := &apitype.ErrorResponse{Code: http.StatusBadRequest, Message: "bad request"}
	require.NotErrorIs(t, translateConfigConflict(apiOther), backend.ErrConfigConflict)

	require.NoError(t, translateConfigConflict(nil))

	plain := errors.New("network down")
	require.Equal(t, plain, translateConfigConflict(plain))
}
