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
	sink := builddir("testdata", "parsetree", "bad__missing_stack_name")

	// Check that the compiler complained about a missing Stack name.
	d := errors.MissingStackName
	assert.Equal(t, sink.Errors(), 1, "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml", d.Message),
		sink.ErrorMsgs()[0])
}

func TestBadStackSemVer1(t *testing.T) {
	sink := builddir("testdata", "parsetree", "bad__stack_semver__1")

	// Check that the compiler complained about an illegal semantic version.
	d := errors.IllegalStackSemVer
	assert.Equal(t, sink.Errors(), 1, "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "badbadbad")),
		sink.ErrorMsgs()[0])
}

func TestBadStackSemVer2(t *testing.T) {
	sink := builddir("testdata", "parsetree", "bad__stack_semver__2")

	// Check that the compiler complained about an illegal semantic version.
	d := errors.IllegalStackSemVer
	assert.Equal(t, sink.Errors(), 1, "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, ">1.0.0")),
		sink.ErrorMsgs()[0])
}
