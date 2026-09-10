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
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
)

// A Diagnostic represents a warning or an error to be presented to the user.
type Diagnostic struct {
	hcl.Diagnostic

	Path string
}

// Error creates a new error-level diagnostic from the given subject, summary, and detail.
func Error(rng *hcl.Range, summary, path string) *Diagnostic {
	return &Diagnostic{
		Diagnostic: hcl.Diagnostic{Severity: hcl.DiagError, Subject: rng, Summary: summary},
		Path:       path,
	}
}

// NodeError creates a new error-level diagnostic from the given node, summary, and detail. If the node is non-nil,
// the diagnostic will be associated with the range of its associated syntax, if any.
func NodeError(node Node, summary string) *Diagnostic {
	var rng *hcl.Range
	var path string
	if node != nil {
		rng = node.Syntax().Range()
		path = node.Syntax().Path()
	}
	return Error(rng, summary, path)
}

// Diagnostics is a list of diagnostics.
type Diagnostics []*Diagnostic

// HasErrors returns true if the list of diagnostics contains any error-level diagnostics.
func (d Diagnostics) HasErrors() bool {
	for _, diag := range d {
		if diag.Severity == hcl.DiagError {
			return true
		}
	}
	return false
}

// Error implements the error interface so that Diagnostics values may interoperate with APIs that use errors.
func (d Diagnostics) Error() string {
	switch len(d) {
	case 0:
		return "no diagnostics"
	case 1:
		return d[0].Error()
	default:
		sort.Slice(d, func(i, j int) bool {
			return d[i].Severity < d[j].Severity
		})
		var sb strings.Builder
		for _, diag := range d {
			if diag.Severity == hcl.DiagError {
				sb.WriteString("\n-error: ")
			} else {
				sb.WriteString("\n-warning: ")
			}
			sb.WriteString(diag.Error())
		}
		return sb.String()
	}
}

// Extend appends the given list of diagnostics to the list.
func (d *Diagnostics) Extend(diags ...*Diagnostic) {
	if len(diags) != 0 {
		for _, diag := range diags {
			if diag != nil {
				*d = append(*d, diag)
			}
		}
	}
}
