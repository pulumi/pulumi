// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package diag

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/golang/glog"

	"github.com/pulumi/pulumi-fabric/pkg/diag/colors"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

// Sink facilitates pluggable diagnostics messages.
type Sink interface {
	// Count fetches the total number of diagnostics issued (errors plus warnings).
	Count() int
	// Infos fetches the number of debug messages issued.
	Debugs() int
	// Infos fetches the number of informational messages issued.
	Infos() int
	// Errors fetches the number of errors issued.
	Errors() int
	// Warnings fetches the number of warnings issued.
	Warnings() int
	// Success returns true if this sink is currently error-free.
	Success() bool

	// Logf issues a log message.
	Logf(sev Severity, diag *Diag, args ...interface{})
	// Debugf issues a debugging message.
	Debugf(diag *Diag, args ...interface{})
	// Infof issues an informational message.
	Infof(diag *Diag, args ...interface{})
	// Errorf issues a new error diagnostic.
	Errorf(diag *Diag, args ...interface{})
	// Warningf issues a new warning diagnostic.
	Warningf(diag *Diag, args ...interface{})

	// Stringify stringifies a diagnostic in the usual way (e.g., "error: MU123: Lumi.yaml:7:39: error goes here\n").
	Stringify(sev Severity, diag *Diag, args ...interface{}) string
	// StringifyLocation stringifies a source document location.
	StringifyLocation(sev Severity, doc *Document, loc *Location) string
}

// Severity dictates the kind of diagnostic.
type Severity string

const (
	Debug   Severity = "debug"
	Info    Severity = "info"
	Warning Severity = "warning"
	Error   Severity = "error"
)

// FormatOptions controls the output style and content.
type FormatOptions struct {
	Pwd    string // the working directory.
	Colors bool   // if true, output will be colorized.
	Debug  bool   // if true, debugging will be output to stdout.
}

// DefaultSink returns a default sink that simply logs output to stderr/stdout.
func DefaultSink(opts FormatOptions) Sink {
	var debug io.Writer
	if opts.Debug {
		debug = os.Stdout
	} else {
		debug = ioutil.Discard
	}
	return newDefaultSink(opts, map[Severity]io.Writer{
		Debug:   debug,
		Info:    os.Stdout,
		Error:   os.Stderr,
		Warning: os.Stderr,
	})
}

func newDefaultSink(opts FormatOptions, writers map[Severity]io.Writer) *defaultSink {
	contract.Assert(writers[Debug] != nil)
	contract.Assert(writers[Info] != nil)
	contract.Assert(writers[Error] != nil)
	contract.Assert(writers[Warning] != nil)
	return &defaultSink{
		opts:    opts,
		counts:  make(map[Severity]int),
		writers: writers,
	}
}

const DefaultSinkIDPrefix = "LUMI"

// defaultSink is the default sink which logs output to stderr/stdout.
type defaultSink struct {
	opts    FormatOptions          // a set of options that control output style and content.
	counts  map[Severity]int       // the number of messages that have been issued per severity.
	writers map[Severity]io.Writer // the writers to use for each kind of diagnostic severity.
}

func (d *defaultSink) Count() int    { return d.Debugs() + d.Infos() + d.Errors() + d.Warnings() }
func (d *defaultSink) Debugs() int   { return d.counts[Debug] }
func (d *defaultSink) Infos() int    { return d.counts[Info] }
func (d *defaultSink) Errors() int   { return d.counts[Error] }
func (d *defaultSink) Warnings() int { return d.counts[Warning] }
func (d *defaultSink) Success() bool { return d.Errors() == 0 }

func (d *defaultSink) Logf(sev Severity, diag *Diag, args ...interface{}) {
	switch sev {
	case Debug:
		d.Debugf(diag, args...)
	case Info:
		d.Infof(diag, args...)
	case Warning:
		d.Warningf(diag, args...)
	case Error:
		d.Errorf(diag, args...)
	default:
		contract.Failf("Unrecognized severity: %v", sev)
	}
}

func (d *defaultSink) Debugf(diag *Diag, args ...interface{}) {
	// For debug messages, write both to the glogger and a stream, if there is one.
	glog.V(3).Infof(diag.Message, args...)
	msg := d.Stringify(Debug, diag, args...)
	if glog.V(9) {
		glog.V(9).Infof("defaultSink::Debug(%v)", msg[:len(msg)-1])
	}
	fmt.Fprintf(d.writers[Debug], msg)
	d.counts[Debug]++
}

func (d *defaultSink) Infof(diag *Diag, args ...interface{}) {
	msg := d.Stringify(Info, diag, args...)
	if glog.V(5) {
		glog.V(5).Infof("defaultSink::Info(%v)", msg[:len(msg)-1])
	}
	fmt.Fprintf(d.writers[Info], msg)
	d.counts[Info]++
}

func (d *defaultSink) Errorf(diag *Diag, args ...interface{}) {
	msg := d.Stringify(Error, diag, args...)
	if glog.V(5) {
		glog.V(5).Infof("defaultSink::Error(%v)", msg[:len(msg)-1])
	}
	fmt.Fprintf(d.writers[Error], msg)
	d.counts[Error]++
}

func (d *defaultSink) Warningf(diag *Diag, args ...interface{}) {
	msg := d.Stringify(Warning, diag, args...)
	if glog.V(5) {
		glog.V(5).Infof("defaultSink::Warning(%v)", msg[:len(msg)-1])
	}
	fmt.Fprintf(d.writers[Warning], msg)
	d.counts[Warning]++
}

func (d *defaultSink) useColor(sev Severity) bool {
	// we will use color so long as we're not spewing to debug (which is colorless).
	return d.opts.Colors
}

func (d *defaultSink) Stringify(sev Severity, diag *Diag, args ...interface{}) string {
	var buffer bytes.Buffer

	// First print the location if there is one.
	if diag.Doc != nil || diag.Loc != nil {
		buffer.WriteString(d.StringifyLocation(sev, diag.Doc, diag.Loc))
		buffer.WriteString(": ")
	}

	// Now print the message category's prefix (error/warning).
	if d.useColor(sev) {
		switch sev {
		case Debug:
			buffer.WriteString(colors.SpecDebug)
		case Info:
			buffer.WriteString(colors.SpecInfo)
		case Error:
			buffer.WriteString(colors.SpecError)
		case Warning:
			buffer.WriteString(colors.SpecWarning)
		default:
			contract.Failf("Unrecognized diagnostic severity: %v", sev)
		}
	}

	buffer.WriteString(string(sev))

	if diag.ID > 0 {
		buffer.WriteString(" ")
		buffer.WriteString(DefaultSinkIDPrefix)
		buffer.WriteString(strconv.Itoa(int(diag.ID)))
	}

	buffer.WriteString(": ")

	if d.useColor(sev) {
		buffer.WriteString(colors.Reset)
	}

	// Finally, actually print the message itself.
	if d.useColor(sev) {
		buffer.WriteString(colors.SpecNote)
	}

	buffer.WriteString(fmt.Sprintf(diag.Message, args...))

	if d.useColor(sev) {
		buffer.WriteString(colors.Reset)
	}

	buffer.WriteRune('\n')

	// TODO[pulumi/pulumi-fabric#15]: support Clang-style expressive diagnostics.  This would entail, for example, using
	//     the buffer within the target document, to demonstrate the offending line/column range of code.

	s := buffer.String()

	// If colorization was requested, compile and execute the directives now.
	if d.useColor(sev) {
		s = colors.ColorizeText(s)
	}

	return s
}

func (d *defaultSink) StringifyLocation(sev Severity, doc *Document, loc *Location) string {
	var buffer bytes.Buffer

	if doc != nil {
		if d.useColor(sev) {
			buffer.WriteString(colors.SpecLocation)
		}

		file := doc.File
		if d.opts.Pwd != "" {
			// If a PWD is available, try to create a relative path.
			rel, err := filepath.Rel(d.opts.Pwd, file)
			if err == nil {
				file = rel
			}
		}
		buffer.WriteString(file)
	}

	if loc != nil && !loc.IsEmpty() {
		buffer.WriteRune('(')
		buffer.WriteString(strconv.Itoa(loc.Start.Line))
		buffer.WriteRune(',')
		buffer.WriteString(strconv.Itoa(loc.Start.Column))
		buffer.WriteRune(')')
	}

	var s string
	if doc != nil || loc != nil {
		if d.useColor(sev) {
			buffer.WriteString(colors.Reset)
		}

		s = buffer.String()

		// If colorization was requested, compile and execute the directives now.
		if d.useColor(sev) {
			s = colors.ColorizeText(s)
		}
	}

	return s
}
