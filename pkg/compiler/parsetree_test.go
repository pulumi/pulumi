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
	sink := buildNoCodegen("testdata", "parsetree", "bad__missing_stack_name")

	// Check that the compiler complained about a missing Stack name.
	d := errors.MissingMetadataName
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "Stack")),
		sink.ErrorMsgs()[0])
}

func TestBadStackSemVer1(t *testing.T) {
	sink := buildNoCodegen("testdata", "parsetree", "bad__stack_semver__1")

	// Check that the compiler complained about an illegal semantic version.
	d := errors.IllegalMetadataSemVer
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "Stack", "badbadbad")),
		sink.ErrorMsgs()[0])
}

func TestBadStackSemVer2(t *testing.T) {
	sink := buildNoCodegen("testdata", "parsetree", "bad__stack_semver__2")

	// Check that the compiler complained about an illegal semantic version.
	d := errors.IllegalMetadataSemVer
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "Stack", ">1.0.0")),
		sink.ErrorMsgs()[0])
}

func TestBadDepSemVer1(t *testing.T) {
	sink := buildNoCodegen("testdata", "parsetree", "bad__dep_semver__1")

	// Check that the compiler complained about an illegal semantic version.
	d := errors.IllegalDependencySemVer
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "dep1", "badbadbad")),
		sink.ErrorMsgs()[0])
}

func TestBadDepSemVer2(t *testing.T) {
	sink := buildNoCodegen("testdata", "parsetree", "bad__dep_semver__2")

	// Check that the compiler complained about an illegal semantic version.
	d := errors.IllegalDependencySemVer
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "dep3", "badbadbad")),
		sink.ErrorMsgs()[0])
}

func TestBadDepSemVer3(t *testing.T) {
	sink := buildNoCodegen("testdata", "parsetree", "bad__dep_semver__3")

	// Check that the compiler complained about an illegal semantic version.
	d := errors.IllegalDependencySemVer
	assert.Equal(t, 4, sink.Errors(), "expected an error for each bad semver")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "dep1", "bad1")),
		sink.ErrorMsgs()[0])
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "dep2", "0.0")),
		sink.ErrorMsgs()[1])
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "dep3", "bad3")),
		sink.ErrorMsgs()[2])
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "dep4", "0.6.bad.ness.1")),
		sink.ErrorMsgs()[3])
}
