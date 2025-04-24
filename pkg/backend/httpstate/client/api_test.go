// Copyright 2023-2024, Pulumi Corporation.
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

package client

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRetryPolicy_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		give retryPolicy
		want string
	}{
		{give: retryNone, want: "none"},
		{give: retryGetMethod, want: "get"},
		{give: retryAllMethods, want: "all"},
		{give: retryPolicy(42), want: "retryPolicy(42)"}, // unknown
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			got := tt.give.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRetryPolicy_shouldRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc   string
		policy retryPolicy
		method string // HTTP method
		want   bool
	}{
		{
			desc:   "none/GET",
			policy: retryNone,
			method: http.MethodGet,
			want:   false,
		},
		{
			desc:   "none/POST",
			policy: retryNone,
			method: http.MethodPost,
			want:   false,
		},
		{
			desc:   "get/GET",
			policy: retryGetMethod,
			method: http.MethodGet,
			want:   true,
		},
		{
			desc:   "get/POST",
			policy: retryGetMethod,
			method: http.MethodPost,
			want:   false,
		},
		{
			desc:   "all/GET",
			policy: retryAllMethods,
			method: http.MethodGet,
			want:   true,
		},
		{
			desc:   "all/POST",
			policy: retryAllMethods,
			method: http.MethodPost,
			want:   true,
		},

		// Sanity check: default is get
		{
			desc: "default/GET",
			// Don't set policy field;
			// zero value should be retryGetMethod.
			method: http.MethodGet,
			want:   true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			got := tt.policy.shouldRetry(&http.Request{
				Method: tt.method,
			})
			assert.Equal(t, tt.want, got)
		})
	}
}
