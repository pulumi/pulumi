// Copyright 2017 Pulumi, Inc. All rights reserved.

package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/lumi/pkg/compiler/core"
	"github.com/pulumi/lumi/pkg/compiler/errors"
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/util/testutil"
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

func TestBadMissingProject(t *testing.T) {
	sink := testCompile("testdata", "bad__missing_lumifile")

	// Check that the compiler complained about a missing Project.
	d := errors.ErrorMissingProject
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v %v%v: %v\n",
			diag.Error, diag.DefaultSinkIDPrefix, d.ID, fmt.Sprintf(d.Message, sink.Pwd)),
		sink.ErrorMsgs()[0])
}

func TestBadProjectCasing(t *testing.T) {
	sink := testCompile("testdata", "bad__lumifile_casing")

	// Check that the compiler warned about a bad Project casing (lumi.yaml).
	d := errors.WarningIllegalMarkupFileCasing
	assert.Equal(t, 1, sink.Warnings(), "expected a single warning")
	assert.Equal(t,
		fmt.Sprintf("%v: %v %v%v: %v\n",
			"lumi.yaml", diag.Warning, diag.DefaultSinkIDPrefix, d.ID, fmt.Sprintf(d.Message, "Lumi")),
		sink.WarningMsgs()[0])
}

func TestBadProjectExt(t *testing.T) {
	sink := testCompile("testdata", "bad__lumifile_ext")

	// Check that the compiler warned about a bad Project extension (none).
	d := errors.WarningIllegalMarkupFileExt
	assert.Equal(t, 1, sink.Warnings(), "expected a single warning")
	assert.Equal(t,
		fmt.Sprintf("%v: %v %v%v: %v\n",
			"Lumi", diag.Warning, diag.DefaultSinkIDPrefix, d.ID,
			fmt.Sprintf(d.Message, "Lumi", "")),
		sink.WarningMsgs()[0])
}

func TestBadProjectExt2(t *testing.T) {
	sink := testCompile("testdata", "bad__lumifile_ext_2")

	// Check that the compiler warned about a bad Project extension (".txt").
	d := errors.WarningIllegalMarkupFileExt
	assert.Equal(t, 1, sink.Warnings(), "expected a single warning")
	assert.Equal(t,
		fmt.Sprintf("%v: %v %v%v: %v\n",
			"Lumi.txt", diag.Warning, diag.DefaultSinkIDPrefix, d.ID,
			fmt.Sprintf(d.Message, "Lumi", ".txt")),
		sink.WarningMsgs()[0])
}

func TestBadMissingPackageName(t *testing.T) {
	sink := testCompile("testdata", "bad__missing_package_name")

	// Check that the compiler complained about a missing package name.
	d := errors.ErrorIllegalProjectSyntax
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v %v%v: %v\n",
			"Lumi.yaml", diag.Error, diag.DefaultSinkIDPrefix, d.ID,
			fmt.Sprintf(d.Message, "1 fields failed to decode:\n"+
				"\tname: Missing required field 'name' on 'pack.Package'")),
		sink.ErrorMsgs()[0])
}

func TestBadEmptyPackageName(t *testing.T) {
	sink := testCompile("testdata", "bad__empty_package_name")

	// Check that the compiler complained about a missing package name.
	d := errors.ErrorIllegalProjectSyntax
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v %v%v: %v\n",
			"Lumi.yaml", diag.Error, diag.DefaultSinkIDPrefix, d.ID,
			fmt.Sprintf(d.Message, "1 fields failed to decode:\n"+
				"\tname: Missing required field 'name' on 'pack.Package'")),
		sink.ErrorMsgs()[0])
}

func TestBadEmptyPackageName2(t *testing.T) {
	sink := testCompile("testdata", "bad__empty_package_name_2")

	// Check that the compiler complained about a missing package name.
	d := errors.ErrorInvalidPackageName
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v %v%v: %v\n",
			"Lumi.yaml", diag.Error, diag.DefaultSinkIDPrefix, d.ID, d.Message),
		sink.ErrorMsgs()[0])
}
