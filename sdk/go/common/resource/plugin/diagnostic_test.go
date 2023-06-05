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

package plugin

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
)

func TestRoundtripDiagnostics(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		diag hcl.Diagnostic
	}{
		{
			name: "simple",
			diag: hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Some issue",
				Detail:   "More info",
			},
		},
		{
			name: "with subject",
			diag: hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Some issue",
				Detail:   "More info",
				Subject: &hcl.Range{
					Filename: "foo",
					Start:    hcl.Pos{Line: 1, Column: 2, Byte: 3},
					End:      hcl.Pos{Line: 4, Column: 5, Byte: 6},
				},
			},
		},
		{
			name: "with context",
			diag: hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Some issue",
				Detail:   "More info",
				Context: &hcl.Range{
					Filename: "foo",
					Start:    hcl.Pos{Line: 1, Column: 2, Byte: 3},
					End:      hcl.Pos{Line: 4, Column: 5, Byte: 6},
				},
			},
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rpcDiag := HclDiagnosticToRPCDiagnostic(&tt.diag)
			roundtripDiag := RPCDiagnosticToHclDiagnostic(rpcDiag)
			assert.Equal(t, tt.diag, *roundtripDiag)
		})
	}
}
