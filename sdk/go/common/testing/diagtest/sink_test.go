// Copyright 2016-2023, Pulumi Corporation.
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

package diagtest

import (
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogSink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string

		// Reference to the diag.Sink method to call.
		// This is an unbound method reference.
		// Use the syntax "($T).$method" to get an unbound method reference.
		fn   func(diag.Sink, *diag.Diag, ...any)
		want string
	}{
		{
			desc: "debug",
			fn:   (diag.Sink).Debugf,
			want: "[stdout] debug: msg",
		},
		{
			desc: "info",
			fn:   (diag.Sink).Infof,
			want: "[stdout] msg",
		},
		{
			desc: "infoerr",
			fn:   (diag.Sink).Infoerrf,
			want: "[stderr] msg",
		},
		{
			desc: "warning",
			fn:   (diag.Sink).Warningf,
			want: "[stderr] warning: msg",
		},
		{
			desc: "error",
			fn:   (diag.Sink).Errorf,
			want: "[stderr] error: msg",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			fakeT := FakeT{TB: t}
			sink := LogSink(&fakeT)

			tt.fn(sink, diag.Message("", "msg"))
			fakeT.RunCleanup()

			require.Len(t, fakeT.Msgs, 1)
			assert.Equal(t, tt.want, fakeT.Msgs[0])
		})
	}
}
