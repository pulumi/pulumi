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

package schema

import (
	"errors"
	"strings"

	"github.com/hashicorp/hcl/v2"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// DiagnosticsError converts binding diagnostics into an error that renders every diagnostic, in the same
// format as `pulumi schema check`. hcl.Diagnostics is itself an error, but its Error method reports only the
// first diagnostic followed by "and N other diagnostic(s)", which hides the errors a schema author needs.
func DiagnosticsError(diags hcl.Diagnostics) error {
	var b strings.Builder
	err := hcl.NewDiagnosticTextWriter(&b, nil, 0, false).WriteDiagnostics(diags)
	contract.IgnoreError(err)
	return errors.New(strings.TrimRight(b.String(), "\n"))
}
