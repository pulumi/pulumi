// Copyright 2024, Pulumi Corporation.
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

package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// outputFormat selects the rendering format for commands that emit structured data.
type outputFormat string

const (
	outputText outputFormat = "text"
	outputJSON outputFormat = "json"
)

// addOutputFlag registers a --output string flag on cmd. The string value should later
// be passed through parseOutputFormat to convert it into a typed outputFormat.
func addOutputFlag(cmd *cobra.Command, output *string) {
	cmd.Flags().StringVar(output, "output", "text", `output format: "text" (default) or "json"`)
}

// parseOutputFormat converts the raw --output flag value into an outputFormat. The empty
// string is treated the same as "text" so callers don't need a special case for an unset
// flag.
func parseOutputFormat(s string) (outputFormat, error) {
	switch s {
	case "", "text":
		return outputText, nil
	case "json":
		return outputJSON, nil
	default:
		return "", fmt.Errorf(`invalid --output %q: must be "text" or "json"`, s)
	}
}

// writeJSON emits v as indented JSON followed by a newline.
func writeJSON(stdout io.Writer, v any) error {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
