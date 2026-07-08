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

package do

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

type diagForwarder func(sev diag.Severity, d *diag.Diag, args ...any)

type forwardingSink struct {
	base diag.Sink

	mu      sync.Mutex
	forward diagForwarder
}

func (s *forwardingSink) set(forward diagForwarder) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.forward = forward
}

func (s *forwardingSink) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.forward = nil
}

func (s *forwardingSink) Logf(sev diag.Severity, d *diag.Diag, args ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.forward != nil {
		s.forward(sev, d, args...)
		return
	}
	s.base.Logf(sev, d, args...)
}

func (s *forwardingSink) Debugf(d *diag.Diag, args ...any)   { s.Logf(diag.Debug, d, args...) }
func (s *forwardingSink) Infof(d *diag.Diag, args ...any)    { s.Logf(diag.Info, d, args...) }
func (s *forwardingSink) Infoerrf(d *diag.Diag, args ...any) { s.Logf(diag.Infoerr, d, args...) }
func (s *forwardingSink) Errorf(d *diag.Diag, args ...any)   { s.Logf(diag.Error, d, args...) }
func (s *forwardingSink) Warningf(d *diag.Diag, args ...any) { s.Logf(diag.Warning, d, args...) }

func stringifyDiag(sev diag.Severity, d *diag.Diag, args ...any) (string, string) {
	var prefix bytes.Buffer
	switch sev {
	case diag.Debug:
		prefix.WriteString(colors.SpecDebug)
	case diag.Error:
		prefix.WriteString(colors.SpecError)
	case diag.Warning:
		prefix.WriteString(colors.SpecWarning)
	case diag.Info, diag.Infoerr:
	}
	if prefix.Len() > 0 {
		prefix.WriteString(string(sev))
		prefix.WriteString(": ")
		prefix.WriteString(colors.Reset)
	}

	var buffer bytes.Buffer
	buffer.WriteString(colors.SpecNote)
	if d.Raw {
		buffer.WriteString(d.Message)
	} else {
		fmt.Fprintf(&buffer, d.Message, args...)
	}
	buffer.WriteString(colors.Reset)
	buffer.WriteRune('\n')

	return prefix.String(), buffer.String()
}
