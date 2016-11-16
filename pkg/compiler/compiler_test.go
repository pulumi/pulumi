// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
)

// testDiagSink suppresses message output, but captures them, so that they can be compared to expected results.
type testDiagSink struct {
	sink     diag.Sink
	errors   []string
	warnings []string
}

func newTestDiagSink(pwd string) *testDiagSink {
	return &testDiagSink{sink: diag.DefaultSink(pwd)}
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
	d.errors = append(d.errors, d.Stringify(dia, "error", args...))
}

func (d *testDiagSink) Warningf(dia *diag.Diag, args ...interface{}) {
	d.warnings = append(d.errors, d.Stringify(dia, "warning", args...))
}

func (d *testDiagSink) Stringify(dia *diag.Diag, prefix string, args ...interface{}) string {
	return d.sink.Stringify(dia, prefix, args...)
}

// Now, begin all of our tests.

func TestMissingMufile(t *testing.T) {
	// New up a compiler in our test's test directory, where we expect it will complain about the lack of a Mufile.
	pwd, _ := os.Getwd()
	td := filepath.Join(pwd, "testdata", "missing_mufile")
	sink := newTestDiagSink(td)
	c := NewCompiler(Options{Diag: sink})
	c.Build(td, td)

	// Check that a single error was issued and that it matches the expected text.
	d := errors.MissingMufile
	assert.Equal(t, sink.Errors(), 1, "expected a single error")
	assert.Equal(t, sink.ErrorMsgs()[0],
		fmt.Sprintf("error: MU%v: %v\n", d.ID, fmt.Sprintf(d.Message, td)))
}
