package engine

import (
	"bytes"
	"fmt"
	"strconv"
	"sync"

	"github.com/pulumi/pulumi/pkg/diag"

	"github.com/golang/glog"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

func newEventSink(events chan<- Event, opts diag.FormatOptions) diag.Sink {
	contract.Require(events != nil, "events")

	return &eventSink{
		events: events,
		opts:   opts,
		counts: make(map[diag.Severity]int),
	}
}

const eventSinkIDPrefix = "PU"

// eventSink is a sink which writes all events to a channel
type eventSink struct {
	events chan<- Event          // the channel to emit events into.
	opts   diag.FormatOptions    // a set of options that control output style and content.
	counts map[diag.Severity]int // the number of messages that have been issued per severity.
	mutex  sync.RWMutex          // a mutex for guarding updates to the counts map
}

func (s *eventSink) Count() int    { return s.Debugs() + s.Infos() + s.Errors() + s.Warnings() }
func (s *eventSink) Debugs() int   { return s.getCount(diag.Debug) }
func (s *eventSink) Infos() int    { return s.getCount(diag.Info) }
func (s *eventSink) Infoerrs() int { return s.getCount(diag.Infoerr) }
func (s *eventSink) Errors() int   { return s.getCount(diag.Error) }
func (s *eventSink) Warnings() int { return s.getCount(diag.Warning) }
func (s *eventSink) Success() bool { return s.Errors() == 0 }

func (s *eventSink) Logf(sev diag.Severity, d *diag.Diag, args ...interface{}) {
	switch sev {
	case diag.Debug:
		s.Debugf(d, args...)
	case diag.Info:
		s.Infof(d, args...)
	case diag.Infoerr:
		s.Infoerrf(d, args...)
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
	s.events <- diagDebugEvent(msg, s.opts.Color)
	s.incrementCount(diag.Debug)
}

func (s *eventSink) Infof(d *diag.Diag, args ...interface{}) {
	msg := s.Stringify(diag.Info, d, args...)
	if glog.V(5) {
		glog.V(5).Infof("eventSink::Info(%v)", msg[:len(msg)-1])
	}
	s.events <- diagInfoEvent(msg, s.opts.Color)
	s.incrementCount(diag.Info)
}

func (s *eventSink) Infoerrf(d *diag.Diag, args ...interface{}) {
	msg := s.Stringify(diag.Info /* not Infoerr, just "info: "*/, d, args...)
	if glog.V(5) {
		glog.V(5).Infof("eventSink::Infoerr(%v)", msg[:len(msg)-1])
	}
	s.events <- diagInfoerrEvent(msg, s.opts.Color)
	s.incrementCount(diag.Infoerr)
}

func (s *eventSink) Errorf(d *diag.Diag, args ...interface{}) {
	msg := s.Stringify(diag.Error, d, args...)
	if glog.V(5) {
		glog.V(5).Infof("eventSink::Error(%v)", msg[:len(msg)-1])
	}
	s.events <- diagErrorEvent(msg, s.opts.Color)
	s.incrementCount(diag.Error)
}

func (s *eventSink) Warningf(d *diag.Diag, args ...interface{}) {
	msg := s.Stringify(diag.Warning, d, args...)
	if glog.V(5) {
		glog.V(5).Infof("eventSink::Warning(%v)", msg[:len(msg)-1])
	}
	s.events <- diagWarningEvent(msg, s.opts.Color)
	s.incrementCount(diag.Warning)
}

func (s *eventSink) incrementCount(sev diag.Severity) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.counts[sev]++
}

func (s *eventSink) getCount(sev diag.Severity) int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.counts[sev]
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

	if d.ID > 0 {
		buffer.WriteString(" ")
		buffer.WriteString(eventSinkIDPrefix)
		buffer.WriteString(strconv.Itoa(int(d.ID)))
	}

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

	return s.opts.Color.Colorize(buffer.String())
}
