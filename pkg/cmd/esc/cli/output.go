// Copyright 2026, Pulumi Corporation.

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
