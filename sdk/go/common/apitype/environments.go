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

package apitype

import (
	"fmt"
	"strings"

	"github.com/pulumi/esc"
)

// EnvironmentDiagnosticSeverity is the severity of an EnvironmentDiagnostic. An empty severity is
// treated as an error for backward compatibility with services that predate the field.
type EnvironmentDiagnosticSeverity string

const (
	DiagError   EnvironmentDiagnosticSeverity = "error"
	DiagWarning EnvironmentDiagnosticSeverity = "warning"
)

type EnvironmentDiagnostic struct {
	Range    *esc.Range                    `json:"range,omitempty"`
	Summary  string                        `json:"summary,omitempty"`
	Detail   string                        `json:"detail,omitempty"`
	Severity EnvironmentDiagnosticSeverity `json:"severity,omitempty"`
}

// IsError reports whether the diagnostic is an error. An empty severity is treated as an error.
func (d EnvironmentDiagnostic) IsError() bool {
	return d.Severity == DiagError || d.Severity == ""
}

type EnvironmentDiagnostics []EnvironmentDiagnostic

// Error implements the Error interface.
func (err EnvironmentDiagnostics) Error() string {
	var diags strings.Builder
	for _, d := range err {
		fmt.Fprintf(&diags, "%v\n", d.Summary)
	}
	return diags.String()
}

// HasErrors reports whether any diagnostic is an error (as opposed to a warning or other non-error
// severity). A successful environment update can still return non-error diagnostics.
func (err EnvironmentDiagnostics) HasErrors() bool {
	for _, d := range err {
		if d.IsError() {
			return true
		}
	}
	return false
}

type EnvironmentDiagnosticsResponse struct {
	Diagnostics EnvironmentDiagnostics `json:"diagnostics,omitempty"`
}

// Error implements the Error interface.
func (err EnvironmentDiagnosticsResponse) Error() string {
	return err.Diagnostics.Error()
}

type OpenEnvironmentResponse struct {
	ID          string                  `json:"id"`
	Diagnostics []EnvironmentDiagnostic `json:"diagnostics,omitempty"`
}
