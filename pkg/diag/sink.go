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

package diag

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
)

// Sink facilitates pluggable diagnostics messages.
type Sink interface {
	// Logf issues a log message.
	Logf(sev Severity, diag *Diag, isStatus bool, args ...interface{})
	// Debugf issues a debugging message.
	Debugf(diag *Diag, isStatus bool, args ...interface{})
	// Infof issues an informational message (to stdout).
	Infof(diag *Diag, isStatus bool, args ...interface{})
	// Infoerrf issues an informational message (to stderr).
	Infoerrf(diag *Diag, isStatus bool, args ...interface{})
	// Errorf issues a new error diagnostic.
	Errorf(diag *Diag, isStatus bool, args ...interface{})
	// Warningf issues a new warning diagnostic.
	Warningf(diag *Diag, isStatus bool, args ...interface{})

	// Stringify stringifies a diagnostic into a prefix and message that is appropriate for printing.
	Stringify(sev Severity, diag *Diag, args ...interface{}) (string, string)
}

// Severity dictates the kind of diagnostic.
type Severity string

const (
	Debug   Severity = "debug"
	Info    Severity = "info"
	Infoerr Severity = "info#err"
	Warning Severity = "warning"
	Error   Severity = "error"
)

// FormatOptions controls the output style and content.
type FormatOptions struct {
	Pwd   string              // the working directory.
	Color colors.Colorization // how output should be colorized.
	Debug bool                // if true, debugging will be output to stdout.
}

// DefaultSink returns a default sink that simply logs output to stderr/stdout.
func DefaultSink(stdout io.Writer, stderr io.Writer, opts FormatOptions) Sink {
	contract.Require(stdout != nil, "stdout")
	contract.Require(stderr != nil, "stderr")
	// Discard debug output by default unless requested.
	debug := ioutil.Discard
	if opts.Debug {
		debug = stdout
	}
	return newDefaultSink(opts, map[Severity]io.Writer{
		Debug:   debug,
		Info:    stdout,
		Infoerr: stderr,
		Error:   stderr,
		Warning: stderr,
	})
}

func newDefaultSink(opts FormatOptions, writers map[Severity]io.Writer) *defaultSink {
	contract.Assert(writers[Debug] != nil)
	contract.Assert(writers[Info] != nil)
	contract.Assert(writers[Infoerr] != nil)
	contract.Assert(writers[Error] != nil)
	contract.Assert(writers[Warning] != nil)
	return &defaultSink{
		opts:    opts,
		writers: writers,
	}
}

const DefaultSinkIDPrefix = "PU"

// defaultSink is the default sink which logs output to stderr/stdout.
type defaultSink struct {
	opts    FormatOptions          // a set of options that control output style and content.
	writers map[Severity]io.Writer // the writers to use for each kind of diagnostic severity.
}

func (d *defaultSink) Logf(sev Severity, diag *Diag, isStatus bool, args ...interface{}) {
	switch sev {
	case Debug:
		d.Debugf(diag, isStatus, args...)
	case Info:
		d.Infof(diag, isStatus, args...)
	case Infoerr:
		d.Infoerrf(diag, isStatus, args...)
	case Warning:
		d.Warningf(diag, isStatus, args...)
	case Error:
		d.Errorf(diag, isStatus, args...)
	default:
		contract.Failf("Unrecognized severity: %v", sev)
	}
}

func (d *defaultSink) createMessage(sev Severity, diag *Diag, args ...interface{}) string {
	prefix, msg := d.Stringify(sev, diag, args...)
	return prefix + msg
}

func (d *defaultSink) Debugf(diag *Diag, isStatus bool, args ...interface{}) {
	// For debug messages, write both to the glogger and a stream, if there is one.
	logging.V(3).Infof(diag.Message, args...)
	msg := d.createMessage(Debug, diag, args...)
	if logging.V(9) {
		logging.V(9).Infof("defaultSink::Debug(%v)", msg[:len(msg)-1])
	}
	fmt.Fprint(d.writers[Debug], msg)
}

func (d *defaultSink) Infof(diag *Diag, isStatus bool, args ...interface{}) {
	msg := d.createMessage(Info, diag, args...)
	if logging.V(5) {
		logging.V(5).Infof("defaultSink::Info(%v)", msg[:len(msg)-1])
	}
	fmt.Fprint(d.writers[Info], msg)
}

func (d *defaultSink) Infoerrf(diag *Diag, isStatus bool, args ...interface{}) {
	msg := d.createMessage(Info /* not Infoerr, just "info: "*/, diag, args...)
	if logging.V(5) {
		logging.V(5).Infof("defaultSink::Infoerr(%v)", msg[:len(msg)-1])
	}
	fmt.Fprint(d.writers[Infoerr], msg)
}

func (d *defaultSink) Errorf(diag *Diag, isStatus bool, args ...interface{}) {
	msg := d.createMessage(Error, diag, args...)
	if logging.V(5) {
		logging.V(5).Infof("defaultSink::Error(%v)", msg[:len(msg)-1])
	}
	fmt.Fprint(d.writers[Error], msg)
}

func (d *defaultSink) Warningf(diag *Diag, isStatus bool, args ...interface{}) {
	msg := d.createMessage(Warning, diag, args...)
	if logging.V(5) {
		logging.V(5).Infof("defaultSink::Warning(%v)", msg[:len(msg)-1])
	}
	fmt.Fprint(d.writers[Warning], msg)
}

func (d *defaultSink) Stringify(sev Severity, diag *Diag, args ...interface{}) (string, string) {
	var prefix bytes.Buffer

	// Now print the message category's prefix (error/warning).
	switch sev {
	case Debug:
		prefix.WriteString(colors.SpecDebug)
	case Info, Infoerr:
		prefix.WriteString(colors.SpecInfo)
	case Error:
		prefix.WriteString(colors.SpecError)
	case Warning:
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

	if diag.Raw {
		buffer.WriteString(diag.Message)
	} else {
		buffer.WriteString(fmt.Sprintf(diag.Message, args...))
	}

	buffer.WriteString(colors.Reset)
	buffer.WriteRune('\n')

	// TODO[pulumi/pulumi#15]: support Clang-style expressive diagnostics.  This would entail, for example, using
	//     the buffer within the target document, to demonstrate the offending line/column range of code.

	// Ensure that any sensitive data we know about is filtered out preemptively.
	filtered := logging.FilterString(buffer.String())

	// If colorization was requested, compile and execute the directives now.
	return d.opts.Color.Colorize(prefix.String()), d.opts.Color.Colorize(filtered)
}
