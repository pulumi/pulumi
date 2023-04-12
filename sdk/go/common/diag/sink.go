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
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// Sink facilitates pluggable diagnostics messages.
type Sink interface {
	// Logf issues a log message.
	Logf(sev Severity, diag *Diag, args ...interface{})
	// Debugf issues a debugging message.
	Debugf(diag *Diag, args ...interface{})
	// Infof issues an informational message (to stdout).
	Infof(diag *Diag, args ...interface{})
	// Infoerrf issues an informational message (to stderr).
	Infoerrf(diag *Diag, args ...interface{})
	// Errorf issues a new error diagnostic.
	Errorf(diag *Diag, args ...interface{})
	// Warningf issues a new warning diagnostic.
	Warningf(diag *Diag, args ...interface{})

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
	contract.Requiref(stdout != nil, "stdout", "must not be nil")
	contract.Requiref(stderr != nil, "stderr", "must not be nil")

	stdoutMu := &sync.Mutex{}
	stderrMu := &sync.Mutex{}
	func() {
		defer func() {
			// The == check below can panic if stdout and stderr are not comparable.
			// If that happens, ignore the panic and use separate mutexes.
			_ = recover()
		}()

		if stdout == stderr {
			// If stdout and stderr point to the same stream,
			// use the same mutex for them.
			stderrMu = stdoutMu
		}
	}()

	// Wrap the stdout and stderr writers in a mutex
	// to ensure that we don't interleave output.
	stdout = &syncWriter{Writer: stdout, mu: stdoutMu}
	stderr = &syncWriter{Writer: stderr, mu: stderrMu}

	// Discard debug output by default unless requested.
	debug := io.Discard
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
	contract.Assertf(writers[Debug] != nil, "Writer for %v must be set", Debug)
	contract.Assertf(writers[Info] != nil, "Writer for %v must be set", Info)
	contract.Assertf(writers[Infoerr] != nil, "Writer for %v must be set", Infoerr)
	contract.Assertf(writers[Error] != nil, "Writer for %v must be set", Error)
	contract.Assertf(writers[Warning] != nil, "Writer for %v must be set", Warning)
	contract.Assertf(opts.Color != "", "FormatOptions.Color must be set")
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

func (d *defaultSink) Logf(sev Severity, diag *Diag, args ...interface{}) {
	switch sev {
	case Debug:
		d.Debugf(diag, args...)
	case Info:
		d.Infof(diag, args...)
	case Infoerr:
		d.Infoerrf(diag, args...)
	case Warning:
		d.Warningf(diag, args...)
	case Error:
		d.Errorf(diag, args...)
	default:
		contract.Failf("Unrecognized severity: %v", sev)
	}
}

func (d *defaultSink) createMessage(sev Severity, diag *Diag, args ...interface{}) string {
	prefix, msg := d.Stringify(sev, diag, args...)
	return prefix + msg
}

func (d *defaultSink) Debugf(diag *Diag, args ...interface{}) {
	// For debug messages, write both to the glogger and a stream, if there is one.
	logging.V(3).Infof(diag.Message, args...)
	msg := d.createMessage(Debug, diag, args...)
	if logging.V(9) {
		logging.V(9).Infof("defaultSink::Debug(%v)", msg[:len(msg)-1])
	}
	d.print(Debug, msg)
}

func (d *defaultSink) Infof(diag *Diag, args ...interface{}) {
	msg := d.createMessage(Info, diag, args...)
	if logging.V(5) {
		logging.V(5).Infof("defaultSink::Info(%v)", msg[:len(msg)-1])
	}
	d.print(Info, msg)
}

func (d *defaultSink) Infoerrf(diag *Diag, args ...interface{}) {
	msg := d.createMessage(Info /* not Infoerr, just "info: "*/, diag, args...)
	if logging.V(5) {
		logging.V(5).Infof("defaultSink::Infoerr(%v)", msg[:len(msg)-1])
	}
	d.print(Infoerr, msg)
}

func (d *defaultSink) Errorf(diag *Diag, args ...interface{}) {
	msg := d.createMessage(Error, diag, args...)
	if logging.V(5) {
		logging.V(5).Infof("defaultSink::Error(%v)", msg[:len(msg)-1])
	}
	d.print(Error, msg)
}

func (d *defaultSink) Warningf(diag *Diag, args ...interface{}) {
	msg := d.createMessage(Warning, diag, args...)
	if logging.V(5) {
		logging.V(5).Infof("defaultSink::Warning(%v)", msg[:len(msg)-1])
	}
	d.print(Warning, msg)
}

func (d *defaultSink) print(sev Severity, msg string) {
	fmt.Fprint(d.writers[sev], msg)
}

func (d *defaultSink) Stringify(sev Severity, diag *Diag, args ...interface{}) (string, string) {
	var prefix bytes.Buffer
	if sev != Info && sev != Infoerr {
		// Unless it's an ordinary stdout message, prepend the message category's prefix (error/warning).
		switch sev {
		case Debug:
			prefix.WriteString(colors.SpecDebug)
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
	}

	// Finally, actually print the message itself.
	var buffer bytes.Buffer
	buffer.WriteString(colors.SpecNote)

	if diag.Raw {
		buffer.WriteString(diag.Message)
	} else {
		fmt.Fprintf(&buffer, diag.Message, args...)
	}

	buffer.WriteString(colors.Reset)
	buffer.WriteRune('\n')

	// Ensure that any sensitive data we know about is filtered out preemptively.
	filtered := logging.FilterString(buffer.String())

	// If colorization was requested, compile and execute the directives now.
	return d.opts.Color.Colorize(prefix.String()), d.opts.Color.Colorize(filtered)
}

// syncWriter wraps an io.Writer and ensures that all writes are synchronized
// with a mutex.
type syncWriter struct {
	io.Writer

	mu *sync.Mutex
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Writer.Write(p)
}
