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
	"strings"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// diagnostics assembles lines and owns the terminal consumer for one helper invocation.
type diagnostics struct {
	line              []byte
	session           *Session
	defaultOutput     chan string
	defaultOutputDone chan struct{}
}

func newDiagnostics(session *Session) *diagnostics {
	d := &diagnostics{session: session, defaultOutput: make(chan string), defaultOutputDone: make(chan struct{})}
	go func() {
		defer close(d.defaultOutputDone)
		for message := range d.defaultOutput {
			session.defaultOutputMu.Lock()
			fmt.Fprintln(session.defaultOutput, message)
			session.defaultOutputMu.Unlock()
		}
	}()
	return d
}

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
	d.session.reportDiagnostic(message, d.defaultOutput)
	d.line = d.line[:0]
}

func (d *diagnostics) close() {
	if len(d.line) > 0 {
		d.flush()
	}
	close(d.defaultOutput)
	<-d.defaultOutputDone
}

func (s *Session) reportDiagnostic(message string, defaultOutput chan<- string) {
	s.displayOutputMu.Lock()
	defer s.displayOutputMu.Unlock()
	if s.displayOutput != nil {
		s.displayOutput <- message
	} else {
		defaultOutput <- message
	}
}

// DisplayDiagnostics routes helper output to messages until the returned function detaches it.
func (s *Session) DisplayDiagnostics(messages chan<- string) func() {
	contract.Requiref(messages != nil, "messages", "expected a diagnostic channel")
	s.displayOutputMu.Lock()
	s.displayOutput = messages
	s.displayOutputMu.Unlock()
	return sync.OnceFunc(func() {
		s.displayOutputMu.Lock()
		defer s.displayOutputMu.Unlock()
		s.displayOutput = nil
	})
}
