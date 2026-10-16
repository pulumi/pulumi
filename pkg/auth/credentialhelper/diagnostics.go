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

package credentialhelper

import (
	"fmt"
	"os"
	"strings"
)

// diagnostics assembles lines and forwards them while the helper is running.
type diagnostics struct {
	line    []byte
	session *Session
}

func newDiagnostics(session *Session) *diagnostics { return &diagnostics{session: session} }
func (d *diagnostics) Write(p []byte) (int, error) {
	for _, b := range p {
		if b == '\n' {
			d.flush()
			continue
		}
		d.line = append(d.line, b)
	}
	return len(p), nil
}

func (d *diagnostics) flush() {
	message := strings.TrimSuffix(string(d.line), "\r")
	report(d.session, message)
	d.line = d.line[:0]
}

func (d *diagnostics) close() {
	if len(d.line) > 0 {
		d.flush()
	}
}

func report(s *Session, message string) {
	if s != nil {
		s.outputMu.Lock()
		defer s.outputMu.Unlock()
		if s.output != nil {
			s.output(message)
			return
		}
	}
	fmt.Fprintln(os.Stderr, message)
}

// DisplayDiagnostics sends helper output through the display's existing system-message sink.
// The sink may block, but must return when its display shuts down.
func (s *Session) DisplayDiagnostics(send func(string)) func() {
	s.outputMu.Lock()
	s.output = send
	s.outputMu.Unlock()
	return func() {
		s.outputMu.Lock()
		defer s.outputMu.Unlock()
		s.output = nil
	}
}
