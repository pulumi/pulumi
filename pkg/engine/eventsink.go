package engine

import (
	"bytes"
	"fmt"

	"github.com/pulumi/pulumi/pkg/diag"

	"github.com/golang/glog"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/util/contract"
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

func (s *eventSink) Logf(sev diag.Severity, d *diag.Diag, args ...interface{}) {
	switch sev {
	case diag.Debug:
		s.Debugf(d, args...)
	case diag.Info:
		s.Infof(d, args...)
	case diag.Warning:
		s.Warningf(d, args...)
	case diag.Error:
		s.Errorf(d, args...)
	default:
		contract.Failf("Unrecognized severity: %v", sev)
	}
}

func (s *eventSink) Debugf(d *diag.Diag, args ...interface{}) {
	// For debug messages, write both to the glogger and a stream, if there is one.
	glog.V(3).Infof(d.Message, args...)
	msg := s.Stringify(diag.Debug, d, args...)
	if glog.V(9) {
		glog.V(9).Infof("eventSink::Debug(%v)", msg[:len(msg)-1])
	}
	s.events.diagDebugEvent(d.URN, msg)
}

func (s *eventSink) Infof(d *diag.Diag, args ...interface{}) {
	msg := s.Stringify(diag.Info, d, args...)
	if glog.V(5) {
		glog.V(5).Infof("eventSink::Info(%v)", msg[:len(msg)-1])
	}
	s.events.diagInfoEvent(d.URN, msg)
}

func (s *eventSink) Errorf(d *diag.Diag, args ...interface{}) {
	msg := s.Stringify(diag.Error, d, args...)
	if glog.V(5) {
		glog.V(5).Infof("eventSink::Error(%v)", msg[:len(msg)-1])
	}
	s.events.diagErrorEvent(d.URN, msg)
}

func (s *eventSink) Warningf(d *diag.Diag, args ...interface{}) {
	msg := s.Stringify(diag.Warning, d, args...)
	if glog.V(5) {
		glog.V(5).Infof("eventSink::Warning(%v)", msg[:len(msg)-1])
	}
	s.events.diagWarningEvent(d.URN, msg)
}

func (s *eventSink) Stringify(sev diag.Severity, d *diag.Diag, args ...interface{}) string {
	var buffer bytes.Buffer

	// Now print the message category's prefix (error/warning).
	switch sev {
	case diag.Debug:
		buffer.WriteString(colors.SpecDebug)
	case diag.Info, diag.Infoerr:
		buffer.WriteString(colors.SpecInfo)
	case diag.Error:
		buffer.WriteString(colors.SpecError)
	case diag.Warning:
		buffer.WriteString(colors.SpecWarning)
	default:
		contract.Failf("Unrecognized diagnostic severity: %v", sev)
	}

	buffer.WriteString(string(sev))
	buffer.WriteString(": ")
	buffer.WriteString(colors.Reset)

	// Finally, actually print the message itself.
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

	return buffer.String()
}
