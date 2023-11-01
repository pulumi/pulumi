// Copyright 2023, Pulumi Corporation.
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

package syntax

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
)

func TestDiagsError(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		assert.Equal(t, "no diagnostics", Diagnostics{}.Error())
	})

	t.Run("single diagnostic", func(t *testing.T) {
		assert.Equal(t, "<nil>: diag summary; ", Diagnostics{Error(nil, "diag summary", "")}.Error())
	})

	t.Run("multiple diagnostics", func(t *testing.T) {
		diags := Diagnostics{
			&Diagnostic{Diagnostic: hcl.Diagnostic{Severity: hcl.DiagError, Summary: "error diag"}},
			&Diagnostic{Diagnostic: hcl.Diagnostic{Severity: hcl.DiagWarning, Summary: "warning diag"}},
		}
		assert.Equal(t, "\n-error: <nil>: error diag; \n-warning: <nil>: warning diag; ", diags.Error())
	})
}
