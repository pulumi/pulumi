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

package telemetry

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPSender(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct {
		status int
		err    bool
	}{
		"accepted":    {status: http.StatusNoContent},
		"rejected":    {status: http.StatusBadRequest},
		"timeout":     {status: http.StatusRequestTimeout, err: true},
		"rate limit":  {status: http.StatusTooManyRequests, err: true},
		"unavailable": {status: http.StatusServiceUnavailable, err: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			service := newFakeService(t, c.status)
			send := httpSender(service.URL+"/", "pulumi-cli/1 (v3.275.0; linux)")

			err := send(t.Context(), []json.RawMessage{json.RawMessage(`{"facts":{"ci":true}}`)})

			if c.err {
				require.ErrorContains(t, err, "telemetry endpoint returned")
			} else {
				require.NoError(t, err)
			}
			received := service.received()
			require.Len(t, received, 1)
			require.Equal(t, "pulumi-cli/1 (v3.275.0; linux)", received[0].UserAgent)
			require.Len(t, received[0].Events, 1)
		})
	}
}
