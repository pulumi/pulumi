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

	"github.com/pulumi/pulumi/sdk/v2/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v2/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/logging"
)

func newEventSink(events eventEmitter, statusSink bool) diag.Sink {
	return &eventSink{
		events:     events,
		statusSink: statusSink,
	}
}

// eventSink is a sink which writes all events to a channel
type eventSink struct {
	events     eventEmitter // the channel to emit events into.
	statusSink bool         // whether this is an event sink for status messages.
}

func (s *eventSink) LogfDepth(
	depth int,
	sev diag.Severity,
	d *diag.Diag,
	args ...interface{}) {

	switch sev {
	case diag.Debug:
		s.DebugfDepth(depth+1, d, args...)
	case diag.Info:
		s.InfofDepth(depth+1, d, args...)
	case diag.Infoerr:
		s.InfoerrfDepth(depth+1, d, args...)
	case diag.Warning:
		s.WarningfDepth(depth+1, d, args...)
	case diag.Error:
		s.ErrorfDepth(depth+1, d, args...)
	default:
		contract.Failf("Unrecognized severity: %v", sev)
	}
}

func (s *eventSink) DebugfDepth(depth int, d *diag.Diag, args ...interface{}) {
	// For debug messages, write both to the glogger and a stream, if there is one.
	if logging.V(3) {
		logging.InfofDepth(depth+1, d.Message, args...)
	}
	prefix, msg := s.Stringify(diag.Debug, d, args...)
	if logging.V(9) {
		logging.InfofDepth(depth+1, "eventSink::Debug(%v)", msg[:len(msg)-1])
	}
	s.events.diagDebugEvent(d, prefix, msg, s.statusSink)
}

func (s *eventSink) InfofDepth(depth int, d *diag.Diag, args ...interface{}) {
	prefix, msg := s.Stringify(diag.Info, d, args...)
	if logging.V(5) {
		logging.InfofDepth(depth+1, "eventSink::Info(%v)", msg[:len(msg)-1])
	}
	s.events.diagInfoEvent(d, prefix, msg, s.statusSink)
}

func (s *eventSink) InfoerrfDepth(depth int, d *diag.Diag, args ...interface{}) {
	prefix, msg := s.Stringify(diag.Info /* not Infoerr, just "info: "*/, d, args...)
	if logging.V(5) {
		logging.InfofDepth(depth+1, "eventSink::Infoerr(%v)", msg[:len(msg)-1])
	}
	s.events.diagInfoerrEvent(d, prefix, msg, s.statusSink)
}

func (s *eventSink) ErrorfDepth(depth int, d *diag.Diag, args ...interface{}) {
	prefix, msg := s.Stringify(diag.Error, d, args...)
	if logging.V(5) {
		logging.InfofDepth(depth+1, "eventSink::Error(%v)", msg[:len(msg)-1])
	}
	s.events.diagErrorEvent(d, prefix, msg, s.statusSink)
}

func (s *eventSink) WarningfDepth(depth int, d *diag.Diag, args ...interface{}) {
	prefix, msg := s.Stringify(diag.Warning, d, args...)
	if logging.V(5) {
		logging.InfofDepth(depth+1, "eventSink::Warning(%v)", msg[:len(msg)-1])
	}
	s.events.diagWarningEvent(d, prefix, msg, s.statusSink)
}

func (d *eventSink) Logf(sev diag.Severity, diag *diag.Diag, args ...interface{}) {
	d.LogfDepth(1, sev, diag, args...)
}

func (d *eventSink) Debugf(diag *diag.Diag, args ...interface{}) {
	d.DebugfDepth(1, diag, args...)
}

func (d *eventSink) Infof(diag *diag.Diag, args ...interface{}) {
	d.InfofDepth(1, diag, args...)
}

func (d *eventSink) Infoerrf(diag *diag.Diag, args ...interface{}) {
	d.InfoerrfDepth(1, diag, args...)
}

func (d *eventSink) Errorf(diag *diag.Diag, args ...interface{}) {
	d.ErrorfDepth(1, diag, args...)
}

func (d *eventSink) Warningf(diag *diag.Diag, args ...interface{}) {
	d.WarningfDepth(1, diag, args...)
}

func (s *eventSink) Stringify(sev diag.Severity, d *diag.Diag, args ...interface{}) (string, string) {
	var prefix bytes.Buffer
	if sev != diag.Info && sev != diag.Infoerr {
		// Unless it's an ordinary stdout message, prepend the message category's prefix (error/warning).
		switch sev {
		case diag.Debug:
			prefix.WriteString(colors.SpecDebug)
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
	}

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
