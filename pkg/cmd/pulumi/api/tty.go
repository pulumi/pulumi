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
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"golang.org/x/term"
)

// stdoutIsTTY reports whether stdout is a terminal.
func stdoutIsTTY() bool { return term.IsTerminal(int(os.Stdout.Fd())) }

// stderrIsTTY reports whether stderr is a terminal.
func stderrIsTTY() bool { return term.IsTerminal(int(os.Stderr.Fd())) }

// stdinIsTTY reports whether stdin is a terminal.
func stdinIsTTY() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

// resolveInteractivity collapses the --interactive / --no-interactive flag
// pair into a single bool. force wins over deny; otherwise the result follows
// whether both stdin and stdout are attached to a TTY.
func resolveInteractivity(force, deny bool) bool {
	switch {
	case force:
		return true
	case deny:
		return false
	default:
		return stdinIsTTY() && stdoutIsTTY()
	}
}

// OutputFormat describes how a command should render its top-level output.
type OutputFormat int

const (
	// OutputDefault is the human-friendly format each command chooses:
	// table for ls, inline schema text for describe, content-type
	// passthrough for raw API calls. Behavior does NOT change based on
	// whether stdout is a TTY — predictability beats cleverness.
	OutputDefault OutputFormat = iota
	// OutputJSON is the stable, agent-facing structured envelope.
	OutputJSON
	// OutputRaw passes body content through unchanged (raw API calls only).
	OutputRaw
	// OutputMarkdown emits a markdown document. Only `describe` implements this today.
	OutputMarkdown
)

// ResolveOutput decides the effective output mode given the --output flag.
// An empty value means OutputDefault (command picks its own human format).
// Accepting "table" explicitly lets `ls` keep the table even when stdout is
// piped — the auto-JSON fallback only runs when --output is unset.
func ResolveOutput(explicit string) (OutputFormat, error) {
	switch explicit {
	case "", "auto", "default", "table":
		return OutputDefault, nil
	case "json":
		return OutputJSON, nil
	case "raw":
		return OutputRaw, nil
	case "markdown", "md":
		return OutputMarkdown, nil
	default:
		return OutputDefault, NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
			"invalid --output value: "+explicit).
			WithSuggestions("--output=json", "--output=markdown", "--output=table", "--output=raw")
	}
}
