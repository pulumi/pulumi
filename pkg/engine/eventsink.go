// Copyright 2016-2018, Pulumi Corporation.
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

package engine

import (
	"bytes"
	"fmt"

	"github.com/pulumi/pulumi/pkg/diag"

	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
)

func newEventSink(events eventEmitter) diag.Sink {
	return &eventSink{
		events: events,
	}
}

// eventSink is a sink which writes all events to a channel
type eventSink struct {
	events eventEmitter // the channel to emit events into.
}

func (s *eventSink) Logf(sev diag.Severity, d *diag.Diag, isStatus bool, args ...interface{}) {
	switch sev {
	case diag.Debug:
		s.Debugf(d, isStatus, args...)
	case diag.Info:
		s.Infof(d, isStatus, args...)
	case diag.Infoerr:
		s.Infoerrf(d, isStatus, args...)
	case diag.Warning:
		s.Warningf(d, isStatus, args...)
	case diag.Error:
		s.Errorf(d, isStatus, args...)
	default:
		contract.Failf("Unrecognized severity: %v", sev)
	}
}

func (s *eventSink) Debugf(d *diag.Diag, isStatus bool, args ...interface{}) {
	// For debug messages, write both to the glogger and a stream, if there is one.
	logging.V(3).Infof(d.Message, args...)
	prefix, msg := s.Stringify(diag.Debug, d, args...)
	if logging.V(9) {
		logging.V(9).Infof("eventSink::Debug(%v)", msg[:len(msg)-1])
	}
	s.events.diagDebugEvent(d, prefix, msg, isStatus)
}

func (s *eventSink) Infof(d *diag.Diag, isStatus bool, args ...interface{}) {
	prefix, msg := s.Stringify(diag.Info, d, args...)
	if logging.V(5) {
		logging.V(5).Infof("eventSink::Info(%v)", msg[:len(msg)-1])
	}
	s.events.diagInfoEvent(d, prefix, msg, isStatus)
}

func (s *eventSink) Infoerrf(d *diag.Diag, isStatus bool, args ...interface{}) {
	prefix, msg := s.Stringify(diag.Info /* not Infoerr, just "info: "*/, d, args...)
	if logging.V(5) {
		logging.V(5).Infof("eventSink::Infoerr(%v)", msg[:len(msg)-1])
	}
	s.events.diagInfoerrEvent(d, prefix, msg, isStatus)
}

func (s *eventSink) Errorf(d *diag.Diag, isStatus bool, args ...interface{}) {
	prefix, msg := s.Stringify(diag.Error, d, args...)
	if logging.V(5) {
		logging.V(5).Infof("eventSink::Error(%v)", msg[:len(msg)-1])
	}
	s.events.diagErrorEvent(d, prefix, msg, isStatus)
}

func (s *eventSink) Warningf(d *diag.Diag, isStatus bool, args ...interface{}) {
	prefix, msg := s.Stringify(diag.Warning, d, args...)
	if logging.V(5) {
		logging.V(5).Infof("eventSink::Warning(%v)", msg[:len(msg)-1])
	}
	s.events.diagWarningEvent(d, prefix, msg, isStatus)
}

func (s *eventSink) Stringify(sev diag.Severity, d *diag.Diag, args ...interface{}) (string, string) {
	var prefix bytes.Buffer

	// Now print the message category's prefix (error/warning).
	switch sev {
	case diag.Debug:
		prefix.WriteString(colors.SpecDebug)
	case diag.Info, diag.Infoerr:
		prefix.WriteString(colors.SpecInfo)
	case diag.Error:
		prefix.WriteString(colors.SpecError)
	case diag.Warning:
		prefix.WriteString(colors.SpecWarning)
	default:
		contract.Failf("Unrecognized diagnostic severity: %v", sev)
	}

	prefix.WriteString(string(sev))
	prefix.WriteString(": ")
	prefix.WriteString(colors.Reset)

	// Finally, actually print the message itself.
	var buffer bytes.Buffer
	buffer.WriteString(colors.SpecNote)

	if d.Raw {
		buffer.WriteString(d.Message)
	} else {
		buffer.WriteString(fmt.Sprintf(d.Message, args...))
	}

	buffer.WriteString(colors.Reset)
	buffer.WriteRune('\n')

	// TODO[pulumi/pulumi#15]: support Clang-style expressive diagnostics.  This would entail, for example, using
	//     the buffer within the target document, to demonstrate the offending line/column range of code.

	return prefix.String(), buffer.String()
}
