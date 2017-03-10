// Copyright 2016 Pulumi, Inc. All rights reserved.

package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/coconut/pkg/compiler/core"
	"github.com/pulumi/coconut/pkg/compiler/errors"
	"github.com/pulumi/coconut/pkg/diag"
	"github.com/pulumi/coconut/pkg/util/contract"
	"github.com/pulumi/coconut/pkg/util/testutil"
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
	comp.Compile(nil)
	return sink
}

func TestBadMissingCocofile(t *testing.T) {
	sink := testCompile("testdata", "bad__missing_cocofile")

	// Check that the compiler complained about a missing Cocofile.
	d := errors.ErrorMissingCocofile
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v %v%v: %v\n",
			diag.Error, diag.DefaultSinkIDPrefix, d.ID, fmt.Sprintf(d.Message, sink.Pwd)),
		sink.ErrorMsgs()[0])
}

func TestBadCocofileCasing(t *testing.T) {
	sink := testCompile("testdata", "bad__cocofile_casing")

	// Check that the compiler warned about a bad Cocofile casing (coconut.yaml).
	d := errors.WarningIllegalMarkupFileCasing
	assert.Equal(t, 1, sink.Warnings(), "expected a single warning")
	assert.Equal(t,
		fmt.Sprintf("%v: %v %v%v: %v\n",
			"coconut.yaml", diag.Warning, diag.DefaultSinkIDPrefix, d.ID, fmt.Sprintf(d.Message, "Coconut")),
		sink.WarningMsgs()[0])
}

func TestBadCocofileExt(t *testing.T) {
	sink := testCompile("testdata", "bad__cocofile_ext")

	// Check that the compiler warned about a bad Cocofile extension (none).
	d := errors.WarningIllegalMarkupFileExt
	assert.Equal(t, 1, sink.Warnings(), "expected a single warning")
	assert.Equal(t,
		fmt.Sprintf("%v: %v %v%v: %v\n",
			"Coconut", diag.Warning, diag.DefaultSinkIDPrefix, d.ID,
			fmt.Sprintf(d.Message, "Coconut", "")),
		sink.WarningMsgs()[0])
}

func TestBadCocofileExt2(t *testing.T) {
	sink := testCompile("testdata", "bad__cocofile_ext_2")

	// Check that the compiler warned about a bad Cocofile extension (".txt").
	d := errors.WarningIllegalMarkupFileExt
	assert.Equal(t, 1, sink.Warnings(), "expected a single warning")
	assert.Equal(t,
		fmt.Sprintf("%v: %v %v%v: %v\n",
			"Coconut.txt", diag.Warning, diag.DefaultSinkIDPrefix, d.ID,
			fmt.Sprintf(d.Message, "Coconut", ".txt")),
		sink.WarningMsgs()[0])
}

func TestBadMissingPackageName(t *testing.T) {
	sink := testCompile("testdata", "bad__missing_package_name")

	// Check that the compiler complained about a missing package name.
	d := errors.ErrorIllegalCocofileSyntax
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v %v%v: %v\n",
			"Coconut.yaml", diag.Error, diag.DefaultSinkIDPrefix, d.ID,
			fmt.Sprintf(d.Message, "Missing required pack.Package field `name`")),
		sink.ErrorMsgs()[0])
}

func TestBadEmptyPackageName(t *testing.T) {
	sink := testCompile("testdata", "bad__empty_package_name")

	// Check that the compiler complained about a missing package name.
	d := errors.ErrorIllegalCocofileSyntax
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v %v%v: %v\n",
			"Coconut.yaml", diag.Error, diag.DefaultSinkIDPrefix, d.ID,
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
			"Coconut.yaml", diag.Error, diag.DefaultSinkIDPrefix, d.ID, d.Message),
		sink.ErrorMsgs()[0])
}
