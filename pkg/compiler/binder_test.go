// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
)

func TestBadMissingStackName(t *testing.T) {
	sink := buildNoCodegen("testdata", "binder", "bad__missing_stack_name")

	// Check that the compiler complained about a missing Stack name.
	d := errors.ErrorMissingStackName
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml", d.Message),
		sink.ErrorMsgs()[0])
}

func TestBadStackSemVer1(t *testing.T) {
	sink := buildNoCodegen("testdata", "binder", "bad__stack_semver__1")

	// Check that the compiler complained about an illegal semantic version.
	d := errors.ErrorIllegalStackVersion
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "badbadbad", "No Major.Minor.Patch elements found")),
		sink.ErrorMsgs()[0])
}

func TestBadStackSemVer2(t *testing.T) {
	sink := buildNoCodegen("testdata", "binder", "bad__stack_semver__2")

	// Check that the compiler complained about an illegal semantic version.
	d := errors.ErrorIllegalStackVersion
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, ">1.0.0",
				"Invalid character(s) found in major number \">1\"")),
		sink.ErrorMsgs()[0])
}

func TestBadDepSemVer1(t *testing.T) {
	sink := buildNoCodegen("testdata", "binder", "bad__dep_semver__1")

	// Check that the compiler complained about an illegal semantic version.
	d := errors.ErrorMalformedStackReference
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "dep1@badbadbad",
				"Illegal version spec: Could not get version from string: \"badbadbad\"")),
		sink.ErrorMsgs()[0])
}

func TestBadDepSemVer2(t *testing.T) {
	sink := buildNoCodegen("testdata", "binder", "bad__dep_semver__2")

	// Check that the compiler complained about an illegal semantic version.
	d := errors.ErrorMalformedStackReference
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "dep3@badbadbad",
				"Illegal version spec: Could not get version from string: \"badbadbad\"")),
		sink.ErrorMsgs()[0])
}

func TestBadDepSemVer3(t *testing.T) {
	sink := buildNoCodegen("testdata", "binder", "bad__dep_semver__3")

	// Check that the compiler complained about an illegal semantic version.
	d := errors.ErrorMalformedStackReference
	assert.Equal(t, 4, sink.Errors(), "expected an error for each bad semver")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "dep1@bad1",
				"Illegal version spec: Could not parse Range \"bad1\": "+
					"Could not parse comparator \"bad\" in \"bad1\"")),
		sink.ErrorMsgs()[0])
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "dep2@0.0",
				"Illegal version spec: Could not parse Range \"0.0\": "+
					"Could not parse version \"0.0\" in \"0.0\": No Major.Minor.Patch elements found")),
		sink.ErrorMsgs()[1])
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "dep3@bad3",
				"Illegal version spec: Could not parse Range \"bad3\": "+
					"Could not parse comparator \"bad\" in \"bad3\"")),
		sink.ErrorMsgs()[2])
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "dep4@0.6.bad.ness.1",
				"Illegal version spec: Could not parse Range \"0.6.bad.ness.1\": "+
					"Could not parse version \"0.6.bad.ness.1\" in \"0.6.bad.ness.1\": "+
					"Invalid character(s) found in patch number \"bad.ness.1\"")),
		sink.ErrorMsgs()[3])
}
func TestSymbolAlreadyExists(t *testing.T) {
	sink := buildNoCodegen("testdata", "binder", "bad__symbol_already_exists")

	// Check that the compiler complained about a duplicate symbol.
	d := errors.ErrorSymbolAlreadyExists
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "foo")),
		sink.ErrorMsgs()[0])
}

func TestTypeNotFound1(t *testing.T) {
	sink := buildNoCodegen("testdata", "binder", "bad__type_not_found__1")

	// Check that the compiler complained about the type missisng.
	d := errors.ErrorTypeNotFound
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "something/non/existent")),
		sink.ErrorMsgs()[0])
}

func TestTypeNotFound2(t *testing.T) {
	sink := buildNoCodegen("testdata", "binder", "bad__type_not_found__2")

	// Check that the compiler complained about the type missisng.
	d := errors.ErrorTypeNotFound
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "something/non/existent")),
		sink.ErrorMsgs()[0])
}

func TestGoodPredefTypes(t *testing.T) {
	sink := buildNoCodegen("testdata", "binder", "good__predef_types")

	// Check that no errors are found when using predefined stack types.
	assert.Equal(t, 0, sink.Errors(), "expected no errors when binding to predef types")
}
