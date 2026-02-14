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
	"io"
	"net/http"
	"os"
	"strings"

	"golang.org/x/term"
)

// FormatResponse reads the response body and formats it for output.
// If jsonOutput is true, prints raw JSON. Otherwise, formats based on Content-Type.
func FormatResponse(resp *http.Response, jsonOutput bool) error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	// Handle rate limiting.
	if resp.StatusCode == 429 {
		return fmt.Errorf("pulumi service: request rate-limit exceeded")
	}

	// Handle error responses.
	if resp.StatusCode >= 400 {
		var pretty bytes.Buffer
		if json.Indent(&pretty, body, "", "  ") == nil && pretty.Len() > 0 {
			fmt.Fprintf(os.Stderr, "Error %d:\n%s\n", resp.StatusCode, pretty.String())
		} else {
			fmt.Fprintf(os.Stderr, "Error %d: %s\n", resp.StatusCode, string(body))
		}
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	// Empty response (e.g., 204 No Content)
	if len(body) == 0 {
		return nil
	}

	if jsonOutput {
		_, err = os.Stdout.Write(body)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout)
		return nil
	}

	// Route based on response Content-Type.
	ct := resp.Header.Get("Content-Type")
	switch {
	case strings.Contains(ct, "application/json"):
		return formatJSON(body)
	case strings.Contains(ct, "application/x-yaml"),
		strings.Contains(ct, "text/plain"),
		strings.Contains(ct, "text/markdown"):
		return formatText(body)
	case strings.Contains(ct, "application/x-tar"),
		strings.Contains(ct, "application/octet-stream"):
		return formatBinary(body)
	default:
		// Unknown content type: try JSON pretty-print, fall back to raw text.
		return formatJSON(body)
	}
}

// formatJSON pretty-prints JSON output, falling back to raw output if not valid JSON.
func formatJSON(body []byte) error {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err != nil {
		// json.Indent can fail if the response contains unescaped control characters
		// (e.g. ANSI escape codes in tool output). Sanitize and retry.
		sanitized := sanitizeJSONControlChars(body)
		if err2 := json.Indent(&pretty, sanitized, "", "  "); err2 != nil {
			// Still invalid, print raw
			_, writeErr := os.Stdout.Write(body)
			if writeErr != nil {
				return writeErr
			}
			fmt.Fprintln(os.Stdout)
			return nil
		}
	}
	fmt.Fprintln(os.Stdout, pretty.String())
	return nil
}

// sanitizeJSONControlChars replaces raw control characters (0x00-0x1F except \t, \n, \r)
// with their JSON unicode escape sequences. This fixes invalid JSON from APIs that embed
// unescaped control characters (e.g. ANSI escape codes) in string values.
func sanitizeJSONControlChars(data []byte) []byte {
	var buf bytes.Buffer
	buf.Grow(len(data))
	for _, b := range data {
		if b < 0x20 && b != '\t' && b != '\n' && b != '\r' {
			fmt.Fprintf(&buf, "\\u%04x", b)
		} else {
			buf.WriteByte(b)
		}
	}
	return buf.Bytes()
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
