// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package diag

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"

	"github.com/golang/glog"

	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// Sink facilitates pluggable diagnostics messages.
type Sink interface {
	// Count fetches the total number of diagnostics issued (errors plus warnings).
	Count() int
	// Infos fetches the number of debug messages issued.
	Debugs() int
	// Infos fetches the number of stdout informational messages issued.
	Infos() int
	// Infos fetches the number of stderr informational messages issued.
	Infoerrs() int
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
	// Infof issues an informational message (to stdout).
	Infof(diag *Diag, args ...interface{})
	// Infoerrf issues an informational message (to stderr).
	Infoerrf(diag *Diag, args ...interface{})
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
	Infoerr Severity = "info#err"
	Warning Severity = "warning"
	Error   Severity = "error"
)

type Color string

const (
	Always Color = "always"
	Never  Color = "never"
	Raw    Color = "raw"
)

func GetColor(debug bool, color string) (Color, error) {
	switch color {
	case "auto":
		if debug {
			// we will use color so long as we're not spewing to debug (which is colorless).
			return Never, nil
		}

		return Always, nil
	case string(Always):
		return Always, nil
	case string(Never):
		return Never, nil
	case string(Raw):
		return Raw, nil
	}

	return Never, fmt.Errorf("unsupported color option: '%s'.  Supported values are: auto, always, never, raw", color)
}

func (c Color) Colorize(v string) string {
	switch c {
	case "raw":
		// Don't touch the string.  Output control sequences as is.
		return v
	case "always":
		// Convert the constrol sequences into appropriate console escapes for the platform we're on.
		return colors.ColorizeText(v)
	case "never":
		// Remove all the colors that any other layers added.
		return stripColors(v)
	default:
		panic("Unexpected color value: " + string(c))
	}
}

func stripColors(v string) string {
	r, _ := regexp.Compile(`<\{%(.*?)%\}>`)
	return r.ReplaceAllString(v, "")
}

// FormatOptions controls the output style and content.
type FormatOptions struct {
	Pwd   string // the working directory.
	Color Color  // how output should be colorized.
	Debug bool   // if true, debugging will be output to stdout.
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
		counts:  make(map[Severity]int),
		writers: writers,
	}
}

const DefaultSinkIDPrefix = "PU"

// defaultSink is the default sink which logs output to stderr/stdout.
type defaultSink struct {
	opts    FormatOptions          // a set of options that control output style and content.
	writers map[Severity]io.Writer // the writers to use for each kind of diagnostic severity.

	counts map[Severity]int // the number of messages that have been issued per severity.
	mutex  sync.RWMutex     // a mutex for guarding updates to the counts map
}

func (d *defaultSink) Count() int    { return d.Debugs() + d.Infos() + d.Errors() + d.Warnings() }
func (d *defaultSink) Debugs() int   { return d.getCount(Debug) }
func (d *defaultSink) Infos() int    { return d.getCount(Info) }
func (d *defaultSink) Infoerrs() int { return d.getCount(Infoerr) }
func (d *defaultSink) Errors() int   { return d.getCount(Error) }
func (d *defaultSink) Warnings() int { return d.getCount(Warning) }
func (d *defaultSink) Success() bool { return d.Errors() == 0 }

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

func (d *defaultSink) Debugf(diag *Diag, args ...interface{}) {
	// For debug messages, write both to the glogger and a stream, if there is one.
	glog.V(3).Infof(diag.Message, args...)
	msg := d.Stringify(Debug, diag, args...)
	if glog.V(9) {
		glog.V(9).Infof("defaultSink::Debug(%v)", msg[:len(msg)-1])
	}
	fmt.Fprint(d.writers[Debug], msg)
	d.incrementCount(Debug)
}

func (d *defaultSink) Infof(diag *Diag, args ...interface{}) {
	msg := d.Stringify(Info, diag, args...)
	if glog.V(5) {
		glog.V(5).Infof("defaultSink::Info(%v)", msg[:len(msg)-1])
	}
	fmt.Fprint(d.writers[Info], msg)
	d.incrementCount(Info)
}

func (d *defaultSink) Infoerrf(diag *Diag, args ...interface{}) {
	msg := d.Stringify(Info /* not Infoerr, just "info: "*/, diag, args...)
	if glog.V(5) {
		glog.V(5).Infof("defaultSink::Infoerr(%v)", msg[:len(msg)-1])
	}
	fmt.Fprint(d.writers[Infoerr], msg)
	d.incrementCount(Infoerr)
}

func (d *defaultSink) Errorf(diag *Diag, args ...interface{}) {
	msg := d.Stringify(Error, diag, args...)
	if glog.V(5) {
		glog.V(5).Infof("defaultSink::Error(%v)", msg[:len(msg)-1])
	}
	fmt.Fprint(d.writers[Error], msg)
	d.incrementCount(Error)
}

func (d *defaultSink) Warningf(diag *Diag, args ...interface{}) {
	msg := d.Stringify(Warning, diag, args...)
	if glog.V(5) {
		glog.V(5).Infof("defaultSink::Warning(%v)", msg[:len(msg)-1])
	}
	fmt.Fprint(d.writers[Warning], msg)
	d.incrementCount(Warning)
}

func (d *defaultSink) incrementCount(sev Severity) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.counts[sev]++
}

func (d *defaultSink) getCount(sev Severity) int {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.counts[sev]
}

func (d *defaultSink) getColor() Color {
	return d.opts.Color
}

func (d *defaultSink) Stringify(sev Severity, diag *Diag, args ...interface{}) string {
	var buffer bytes.Buffer

	// First print the location if there is one.
	if diag.Doc != nil || diag.Loc != nil {
		buffer.WriteString(d.StringifyLocation(sev, diag.Doc, diag.Loc))
		buffer.WriteString(": ")
	}

	// Now print the message category's prefix (error/warning).
	switch sev {
	case Debug:
		buffer.WriteString(colors.SpecDebug)
	case Info, Infoerr:
		buffer.WriteString(colors.SpecInfo)
	case Error:
		buffer.WriteString(colors.SpecError)
	case Warning:
		buffer.WriteString(colors.SpecWarning)
	default:
		contract.Failf("Unrecognized diagnostic severity: %v", sev)
	}

	buffer.WriteString(string(sev))

	if diag.ID > 0 {
		buffer.WriteString(" ")
		buffer.WriteString(DefaultSinkIDPrefix)
		buffer.WriteString(strconv.Itoa(int(diag.ID)))
	}

	buffer.WriteString(": ")
	buffer.WriteString(colors.Reset)

	// Finally, actually print the message itself.
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

	s := buffer.String()

	// If colorization was requested, compile and execute the directives now.
	return d.getColor().Colorize(s)
}

func (d *defaultSink) StringifyLocation(sev Severity, doc *Document, loc *Location) string {
	var buffer bytes.Buffer

	if doc != nil {
		buffer.WriteString(colors.SpecLocation)

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
		buffer.WriteString(colors.Reset)
		s = buffer.String()

		// If colorization was requested, compile and execute the directives now.
		s = d.getColor().Colorize(s)
	}

	return s
}
