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

package cloud

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// outputFormat describes how a command should render its top-level output.
type outputFormat int

const (
	// outputDefault is "no --format passed" — the command picks its own
	// human-friendly format and may auto-switch when non-interactive
	// (e.g. `list` flips to JSON).
	outputDefault outputFormat = iota
	// outputTable is "user explicitly asked for a table" — never auto-switch.
	outputTable
	// outputJSON is the stable, agent-facing structured envelope.
	outputJSON
	// outputRaw passes body content through unchanged (raw API calls only).
	outputRaw
	// outputMarkdown emits a markdown document. Only `describe` implements this today.
	outputMarkdown
)

// resolveOutput decides the effective output mode given the --format flag.
func resolveOutput(explicit string) (outputFormat, error) {
	switch explicit {
	case "", "auto", "default":
		return outputDefault, nil
	case "table":
		return outputTable, nil
	case "json":
		return outputJSON, nil
	case "raw":
		return outputRaw, nil
	case "markdown", "md":
		return outputMarkdown, nil
	default:
		return outputDefault, NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
			"invalid --format value: "+explicit).
			WithSuggestions("--format=json", "--format=markdown", "--format=table", "--format=raw")
	}
}
