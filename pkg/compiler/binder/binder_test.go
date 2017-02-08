// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/errors"
	"github.com/marapongo/mu/pkg/compiler/metadata"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/util/contract"
	"github.com/marapongo/mu/pkg/util/testutil"
	"github.com/marapongo/mu/pkg/workspace"
)

func testBind(paths ...string) *testutil.TestDiagSink {
	// Create the test directory path.
	pwd, _ := os.Getwd()
	testdir := filepath.Join(append([]string{pwd}, paths...)...)

	// Create a test sink, so we can capture and inspect outputs.
	sink := testutil.NewTestDiagSink(testdir)

	// Create the compiler machinery (context, reader, workspace).
	ctx := core.NewContext(testdir, sink, &core.Options{Diag: sink})
	reader := metadata.NewReader(ctx)
	w, err := workspace.New(ctx)
	contract.Assertf(err == nil, "Expected nil workspace error; got '%v'", err)

	// Detect and read in the package.
	pkgpath, err := w.DetectPackage()
	contract.Assertf(err == nil, "Expected nil package detection error; got '%v'", err)
	pkgdoc, err := diag.ReadDocument(pkgpath)
	contract.Assertf(err == nil, "Expected nil package reader error; got '%v'", err)
	pkg := reader.ReadPackage(pkgdoc)

	// Now create a binder and bind away, returning the resulting sink.
	if pkg != nil {
		b := New(w, ctx, reader)
		b.BindPackage(pkg)
	}
	return sink
}

func TestBadDepSemVer(t *testing.T) {
	sink := testBind("testdata", "bad__dep_semver")

	// Check that the compiler complained about an illegal semantic version.
	d := errors.ErrorMalformedPackageURL
	assert.Equal(t, 3, sink.Errors(), "expected an error for each bad semver")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "dep1#badbadbad",
				"Illegal version spec: Could not get version from string: \"badbadbad\"")),
		sink.ErrorMsgs()[0])
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "hub.mu.com/dep2#badbadbad",
				"Illegal version spec: Could not get version from string: \"badbadbad\"")),
		sink.ErrorMsgs()[1])
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "https://hub.mu.com/dep3/a/b/c/d#badbadbad",
				"Illegal version spec: Could not get version from string: \"badbadbad\"")),
		sink.ErrorMsgs()[2])
}

func TestBadTypeNotFound(t *testing.T) {
	sink := testBind("testdata", "bad__type_not_found")

	// Check that the compiler complained about the type missisng.
	assert.Equal(t, 2, sink.Errors(), "expected a single error")
	d1 := errors.ErrorSymbolNotFound
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d1.ID,
			fmt.Sprintf(d1.Message, "missing/package:bad/module/Clazz", "package 'missing/package' not found")),
		sink.ErrorMsgs()[0])
	d2 := errors.ErrorTypeNotFound
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d2.ID,
			fmt.Sprintf(d2.Message, "missing/package:bad/module/Clazz", "type symbol not found")),
		sink.ErrorMsgs()[1])
}

func TestGoodPrimitiveTypes(t *testing.T) {
	sink := testBind("testdata", "good__primitive_types")

	// Check that no errors are found when using primitive types.
	assert.Equal(t, 0, sink.Errors(), "expected no errors when binding to primitive types")
}
