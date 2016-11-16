// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"os"
	"path/filepath"

	"github.com/marapongo/mu/pkg/diag"
)

// testDiagSink suppresses message output, but captures them, so that they can be compared to expected results.
type testDiagSink struct {
	Pwd      string
	sink     diag.Sink
	errors   []string
	warnings []string
}

func newTestDiagSink(pwd string) *testDiagSink {
	return &testDiagSink{Pwd: pwd, sink: diag.DefaultSink(pwd)}
}

func (d *testDiagSink) Count() int {
	return d.Errors() + d.Warnings()
}

func (d *testDiagSink) Errors() int {
	return len(d.errors)
}

func (d *testDiagSink) ErrorMsgs() []string {
	return d.errors
}

func (d *testDiagSink) Warnings() int {
	return len(d.warnings)
}

func (d *testDiagSink) WarningMsgs() []string {
	return d.warnings
}

func (d *testDiagSink) Errorf(dia *diag.Diag, args ...interface{}) {
	d.errors = append(d.errors, d.Stringify(dia, diag.DefaultSinkErrorPrefix, args...))
}

func (d *testDiagSink) Warningf(dia *diag.Diag, args ...interface{}) {
	d.warnings = append(d.warnings, d.Stringify(dia, diag.DefaultSinkWarningPrefix, args...))
}

func (d *testDiagSink) Stringify(dia *diag.Diag, prefix string, args ...interface{}) string {
	return d.sink.Stringify(dia, prefix, args...)
}

func builddir(paths ...string) *testDiagSink {
	pwd, _ := os.Getwd()
	td := filepath.Join(append([]string{pwd}, paths...)...)
	sink := newTestDiagSink(td)
	c := NewCompiler(Options{Diag: sink})
	c.Build(td, td)
	return sink
}
