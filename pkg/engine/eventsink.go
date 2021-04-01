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

	"github.com/golang/glog"

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

	var messageSeverity diag.Severity = sev
	var glogLevel glog.Level = 5
	var format string

	switch sev {
	case diag.Debug:
		glogLevel = 9
		format = "eventSink::Debug(%v)"

		// For debug messages, write both to the glogger and a
		// stream, if there is one.
		if logging.V(3) {
			logging.InfofDepth(depth+1, d.Message, args...)
		}

	case diag.Info:
		format = "eventSink::Info(%v)"
	case diag.Infoerr:
		messageSeverity = diag.Info /* not Infoerr, just "info: "*/
		format = "eventSink::Infoerr(%v)"
	case diag.Warning:
		format = "eventSink::Warning(%v)"
	case diag.Error:
		format = "eventSink::Error(%v)"
	default:
		contract.Failf("Unrecognized severity: %v", sev)
	}

	prefix, msg := s.Stringify(messageSeverity, d, args...)
	if logging.V(glogLevel) {
		logging.InfofDepth(depth+1, format, msg[:len(msg)-1])
	}
	diagEvent(&s.events, d, prefix, msg, sev, s.statusSink)
}

func (s *eventSink) Logf(sev diag.Severity, d *diag.Diag, args ...interface{}) {
	s.LogfDepth(1, sev, d, args...)
}

func (s *eventSink) Debugf(d *diag.Diag, args ...interface{}) {
	s.LogfDepth(1, diag.Debug, d, args...)
}

func (s *eventSink) Infof(d *diag.Diag, args ...interface{}) {
	s.LogfDepth(1, diag.Info, d, args...)
}

func (s *eventSink) Infoerrf(d *diag.Diag, args ...interface{}) {
	s.LogfDepth(1, diag.Infoerr, d, args...)
}

func (s *eventSink) Errorf(d *diag.Diag, args ...interface{}) {
	s.LogfDepth(1, diag.Error, d, args...)
}

func (s *eventSink) Warningf(d *diag.Diag, args ...interface{}) {
	s.LogfDepth(1, diag.Warning, d, args...)
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
