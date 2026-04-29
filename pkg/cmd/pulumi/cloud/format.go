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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// formatJSON pretty-prints JSON output to w, falling back to raw bytes when
// the server returned something that isn't valid JSON.
func formatJSON(w io.Writer, body []byte) error {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err != nil {
		if _, werr := w.Write(body); werr != nil {
			return werr
		}
		fmt.Fprintln(w)
		return nil
	}
	fmt.Fprintln(w, pretty.String())
	return nil
}

// formatText writes text content directly to w with a trailing newline if
// missing.
func formatText(w io.Writer, body []byte) error {
	if _, err := w.Write(body); err != nil {
		return err
	}
	if len(body) > 0 && body[len(body)-1] != '\n' {
		fmt.Fprintln(w)
	}
	return nil
}

// formatBinary writes binary content to w when not interactive, or prints a
// hint to stderr when w is the terminal so we don't blast bytes at the user.
func formatBinary(w io.Writer, body []byte) error {
	if w == os.Stdout && cmdutil.Interactive() {
		fmt.Fprintf(os.Stderr, "Binary response (%d bytes). Redirect stdout to save to a file.\n", len(body))
		return nil
	}
	_, err := w.Write(body)
	return err
}
