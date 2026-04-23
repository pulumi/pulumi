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

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/term"
)

// formatJSON pretty-prints JSON output, falling back to raw bytes when the
// server returned something that isn't valid JSON.
func formatJSON(body []byte) error {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err != nil {
		if _, werr := os.Stdout.Write(body); werr != nil {
			return werr
		}
		fmt.Fprintln(os.Stdout)
		return nil
	}
	fmt.Fprintln(os.Stdout, pretty.String())
	return nil
}

// formatText writes text content directly to stdout.
func formatText(body []byte) error {
	_, err := os.Stdout.Write(body)
	if err != nil {
		return err
	}
	// Ensure trailing newline for clean terminal output.
	if len(body) > 0 && body[len(body)-1] != '\n' {
		fmt.Fprintln(os.Stdout)
	}
	return nil
}

// formatBinary writes binary content to stdout for piping, or shows a message
// if stdout is a terminal.
func formatBinary(body []byte) error {
	if term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Fprintf(os.Stderr, "Binary response (%d bytes). Redirect stdout to save to a file.\n", len(body))
		return nil
	}
	_, err := os.Stdout.Write(body)
	return err
}
