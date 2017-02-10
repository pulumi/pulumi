// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/errors"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/util/contract"
	"github.com/marapongo/mu/pkg/util/testutil"
)

func testCompile(paths ...string) *testutil.TestDiagSink {
	// Create the test directory path.
	pwd, _ := os.Getwd()
	testdir := filepath.Join(append([]string{pwd}, paths...)...)

	// Create a test sink, so we can capture and inspect outputs.
	sink := testutil.NewTestDiagSink(testdir)

	// Create the compiler machinery, perform the compile, and return the sink.
	comp, err := New(testdir, &core.Options{Diag: sink})
	contract.Assertf(err == nil, "Expected a nil error from compiler constructor; got '%v'", err)
	comp.Compile()
	return sink
}

func TestBadMissingMufile(t *testing.T) {
	sink := testCompile("testdata", "bad__missing_mufile")

	// Check that the compiler complained about a missing Mufile.
	d := errors.ErrorMissingMufile
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v %v%v: %v\n",
			diag.Error, diag.DefaultSinkIDPrefix, d.ID, fmt.Sprintf(d.Message, sink.Pwd)),
		sink.ErrorMsgs()[0])
}

func TestBadMufileCasing(t *testing.T) {
	sink := testCompile("testdata", "bad__mufile_casing")

	// Check that the compiler warned about a bad Mufile casing (mu.yaml).
	d := errors.WarningIllegalMarkupFileCasing
	assert.Equal(t, 1, sink.Warnings(), "expected a single warning")
	assert.Equal(t,
		fmt.Sprintf("%v: %v %v%v: %v\n",
			"mu.yaml", diag.Warning, diag.DefaultSinkIDPrefix, d.ID, fmt.Sprintf(d.Message, "Mu")),
		sink.WarningMsgs()[0])
}

func TestBadMufileExt(t *testing.T) {
	sink := testCompile("testdata", "bad__mufile_ext")

	// Check that the compiler warned about a bad Mufile extension (none).
	d := errors.WarningIllegalMarkupFileExt
	assert.Equal(t, 1, sink.Warnings(), "expected a single warning")
	assert.Equal(t,
		fmt.Sprintf("%v: %v %v%v: %v\n",
			"Mu", diag.Warning, diag.DefaultSinkIDPrefix, d.ID,
			fmt.Sprintf(d.Message, "Mu", "")),
		sink.WarningMsgs()[0])
}

func TestBadMufileExt2(t *testing.T) {
	sink := testCompile("testdata", "bad__mufile_ext_2")

	// Check that the compiler warned about a bad Mufile extension (".txt").
	d := errors.WarningIllegalMarkupFileExt
	assert.Equal(t, 1, sink.Warnings(), "expected a single warning")
	assert.Equal(t,
		fmt.Sprintf("%v: %v %v%v: %v\n",
			"Mu.txt", diag.Warning, diag.DefaultSinkIDPrefix, d.ID,
			fmt.Sprintf(d.Message, "Mu", ".txt")),
		sink.WarningMsgs()[0])
}

func TestBadMissingPackageName(t *testing.T) {
	sink := testCompile("testdata", "bad__missing_package_name")

	// Check that the compiler complained about a missing package name.
	d := errors.ErrorIllegalMufileSyntax
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v %v%v: %v\n",
			"Mu.yaml", diag.Error, diag.DefaultSinkIDPrefix, d.ID,
			fmt.Sprintf(d.Message, "Missing required pack.Package field `name`")),
		sink.ErrorMsgs()[0])
}

func TestBadEmptyPackageName(t *testing.T) {
	sink := testCompile("testdata", "bad__empty_package_name")

	// Check that the compiler complained about a missing package name.
	d := errors.ErrorIllegalMufileSyntax
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v %v%v: %v\n",
			"Mu.yaml", diag.Error, diag.DefaultSinkIDPrefix, d.ID,
			fmt.Sprintf(d.Message, "Missing required pack.Package field `name`")),
		sink.ErrorMsgs()[0])
}

func TestBadEmptyPackageName2(t *testing.T) {
	sink := testCompile("testdata", "bad__empty_package_name_2")

	// Check that the compiler complained about a missing package name.
	d := errors.ErrorInvalidPackageName
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v %v%v: %v\n",
			"Mu.yaml", diag.Error, diag.DefaultSinkIDPrefix, d.ID, d.Message),
		sink.ErrorMsgs()[0])
}
