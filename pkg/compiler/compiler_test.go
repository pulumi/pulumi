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

// Now, begin all of our tests.

func builddir(paths ...string) *testDiagSink {
	pwd, _ := os.Getwd()
	td := filepath.Join(append([]string{pwd}, paths...)...)
	sink := newTestDiagSink(td)
	c := NewCompiler(Options{Diag: sink})
	c.Build(td, td)
	return sink
}

func TestMissingMufile(t *testing.T) {
	// New up a compiler in our test's test directory, where we expect it will complain about the lack of a Mufile.
	sink := builddir("testdata", "missing_mufile")

	// Check that a single error was issued and that it matches the expected text.
	d := errors.MissingMufile
	assert.Equal(t, sink.Errors(), 1, "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, fmt.Sprintf(d.Message, sink.Pwd)),
		sink.ErrorMsgs()[0])
}

func TestIllegalMufileCasing(t *testing.T) {
	// New up a compiler in our test's test directory, where we expect it will complain about incorrect casing.
	sink := builddir("testdata", "illegal_mufile_casing")

	// Check that a single error was issued and that it matches the expected text.
	d := errors.WarnIllegalMufileCasing
	assert.Equal(t, sink.Warnings(), 1, "expected a single warning")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkWarningPrefix, diag.DefaultSinkIDPrefix, d.ID, "mu.yaml", d.Message),
		sink.WarningMsgs()[0])
}

func TestIllegalMufileExt1(t *testing.T) {
	// New up a compiler in our test's test directory, where we expect it will complain about a bad extension.
	sink := builddir("testdata", "illegal_mufile_ext1")

	// Check that a single error was issued and that it matches the expected text.
	d := errors.WarnIllegalMufileExt
	assert.Equal(t, sink.Warnings(), 1, "expected a single warning")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkWarningPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu", fmt.Sprintf(d.Message, "")),
		sink.WarningMsgs()[0])
}

func TestIllegalMufileExt2(t *testing.T) {
	// New up a compiler in our test's test directory, where we expect it will complain about a bad extension.
	sink := builddir("testdata", "illegal_mufile_ext2")

	// Check that a single error was issued and that it matches the expected text.
	d := errors.WarnIllegalMufileExt
	assert.Equal(t, sink.Warnings(), 1, "expected a single warning")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkWarningPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.txt", fmt.Sprintf(d.Message, ".txt")),
		sink.WarningMsgs()[0])
}
