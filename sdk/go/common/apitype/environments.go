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

package apitype

import (
	"fmt"
	"strings"
)

// A EnvironmentRange defines a range within an environment definition.
type EnvironmentRange struct {
	// The name of the environment.
	Environment string `json:"environment,omitempty"`

	// The beginning of the range.
	Begin EnvironmentRangePos `json:"begin"`

	// The end of the range.
	End EnvironmentRangePos `json:"end"`
}

// Contains returns true if the range contains the given position.
func (r EnvironmentRange) Contains(pos EnvironmentRangePos) bool {
	if pos.Byte >= r.Begin.Byte && pos.Byte < r.End.Byte {
		return true
	}
	if pos.Line < r.Begin.Line || pos.Line > r.End.Line {
		return false
	}
	if r.Begin.Line == r.End.Line {
		return pos.Line == r.Begin.Line && pos.Column >= r.Begin.Column && pos.Column < r.End.Column
	}
	return true
}

// A EnvironmentRangePos defines a position within an environment definition.
type EnvironmentRangePos struct {
	// Line is the source code line where this position points. Lines are counted starting at 1 and incremented for each
	// newline character encountered.
	Line int `json:"line"`

	// Column is the source code column where this position points. Columns are counted in visual cells starting at 1,
	// and are incremented roughly per grapheme cluster encountered.
	Column int `json:"column"`

	// Byte is the byte offset into the file where the indicated position begins.
	Byte int `json:"byte"`
}

type EnvironmentDiagnostic struct {
	Range   EnvironmentRange `json:"range,omitempty"`
	Summary string           `json:"summary,omitempty"`
	Detail  string           `json:"detail,omitempty"`
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
