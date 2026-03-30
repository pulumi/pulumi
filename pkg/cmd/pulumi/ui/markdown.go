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

package ui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// RenderMarkdown renders markdown content appropriately for the current terminal.
// In interactive mode, it renders with glamour and pipes through a pager.
// In non-interactive mode, it outputs raw markdown for agent consumption.
func RenderMarkdown(content string) error {
	return FprintMarkdown(os.Stdout, content)
}

// RenderMarkdownInline renders markdown with glamour but prints directly to stdout
// without opening a pager. Use this after interactive selections where opening
// a pager would be jarring.
func RenderMarkdownInline(content string) error {
	if !cmdutil.Interactive() {
		_, err := fmt.Fprint(os.Stdout, content)
		return err
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(0),
	)
	if err != nil {
		_, err := fmt.Fprint(os.Stdout, content)
		return err
	}

	rendered, err := renderer.Render(content)
	if err != nil {
		_, err := fmt.Fprint(os.Stdout, content)
		return err
	}

	_, err = fmt.Fprint(os.Stdout, rendered)
	return err
}

// FprintMarkdown renders markdown to the given writer.
func FprintMarkdown(w io.Writer, content string) error {
	if !cmdutil.Interactive() {
		_, err := fmt.Fprint(w, content)
		return err
	}

	// Render with glamour for terminal display.
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(0),
	)
	if err != nil {
		// Fallback to raw markdown.
		_, err := fmt.Fprint(w, content)
		return err
	}

	rendered, err := renderer.Render(content)
	if err != nil {
		_, err := fmt.Fprint(w, content)
		return err
	}

	// If stdout is the writer, pipe through a pager.
	if w == os.Stdout {
		return pagerOutput(rendered)
	}

	_, err = fmt.Fprint(w, rendered)
	return err
}

// pagerOutput pipes content through a pager (like less) for paginated viewing.
func pagerOutput(content string) error {
	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less"
	}

	// Build the command. Use -R for ANSI color support.
	var cmd *exec.Cmd
	if pager == "less" {
		cmd = exec.Command(pager, "-R")
	} else {
		parts := strings.Fields(pager)
		cmd = exec.Command(parts[0], parts[1:]...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		// Fallback: just print directly.
		_, err := fmt.Print(content)
		return err
	}

	if err := cmd.Start(); err != nil {
		// Fallback: just print directly.
		_, err := fmt.Print(content)
		return err
	}

	_, _ = io.WriteString(stdin, content)
	stdin.Close()

	return cmd.Wait()
}
