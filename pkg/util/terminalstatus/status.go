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

// Package terminalstatus reports CLI activity using the OSC 7501 Program Status Protocol.
// See https://www.superlogical.com/rex/docs/build/program-status.
package terminalstatus

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// State is a state value in the OSC 7501 Program Status Protocol.
type State string

const (
	// StateIdle indicates the program is waiting for its next instruction.
	StateIdle State = "idle"
	// StateWorking indicates the program is running.
	StateWorking State = "working"
	// StateDone indicates the program finished and its result is ready to look at.
	StateDone State = "done"
	// StateBlocked indicates the program needs user input to continue.
	StateBlocked State = "blocked"
	// StateError indicates the program failed and stopped.
	StateError State = "error"
	// StateClear removes the addressed record and its descendants.
	StateClear State = "clear"
)

// Working reports an active operation with the given message and returns a function that clears its status.
// If out is nil, it defaults to stderr.
func Working(out io.Writer, message string) func() {
	if out == nil {
		out = os.Stderr
	}
	return start(out, "pulumi/operation", StateWorking, message)
}

// NeedsInput reports a prompt and returns a function that clears its status.
// If out is nil, it defaults to stderr.
func NeedsInput(out io.Writer) func() {
	if out == nil {
		out = os.Stderr
	}
	return start(out, "pulumi/prompt", StateBlocked, "")
}

func start(out io.Writer, id string, state State, message string) func() {
	f, ok := out.(*os.File)
	if !ok || !cmdutil.Interactive() || !term.IsTerminal(int(f.Fd())) {
		return func() {}
	}
	// Separate records let a prompt temporarily take priority over an ongoing operation.
	// Do not include prompt text, which may contain secrets or terminal control characters.
	var messageField string
	if encoded := encodeMessage(message); encoded != "" {
		messageField = ":msg=" + encoded
	}
	fmt.Fprintf(out, "\x1b]7501;state=%s:id=%s:app=pulumi%s\x1b\\", state, id, messageField)
	return func() {
		fmt.Fprintf(out, "\x1b]7501;state=%s:id=%s:app=pulumi\x1b\\", StateClear, id)
	}
}

func encodeMessage(message string) string {
	var text strings.Builder
	for _, r := range message {
		if unicode.IsControl(r) {
			continue
		}
		if text.Len()+utf8.RuneLen(r) > 2048 {
			break
		}
		text.WriteRune(r)
	}
	return base64.StdEncoding.EncodeToString([]byte(text.String()))
}
