// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
)

func TestBadMissingMufile(t *testing.T) {
	sink := builddir("testdata", "compiler", "bad__missing_mufile")

	// Check that the compiler complained about a missing Mufile.
	d := errors.MissingMufile
	assert.Equal(t, sink.Errors(), 1, "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, fmt.Sprintf(d.Message, sink.Pwd)),
		sink.ErrorMsgs()[0])
}

func TestBadMufileCasing(t *testing.T) {
	sink := builddir("testdata", "compiler", "bad__mufile_casing")

	// Check that the compiler warned about a bad Mufile casing (mu.yaml).
	d := errors.WarnIllegalMufileCasing
	assert.Equal(t, sink.Warnings(), 1, "expected a single warning")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkWarningPrefix, diag.DefaultSinkIDPrefix, d.ID, "mu.yaml", d.Message),
		sink.WarningMsgs()[0])
}

func TestBadMufileExt1(t *testing.T) {
	sink := builddir("testdata", "compiler", "bad__mufile_ext__1")

	// Check that the compiler warned about a bad Mufile extension (none).
	d := errors.WarnIllegalMufileExt
	assert.Equal(t, sink.Warnings(), 1, "expected a single warning")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkWarningPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu", fmt.Sprintf(d.Message, "")),
		sink.WarningMsgs()[0])
}

func TestBadMufileExt2(t *testing.T) {
	sink := builddir("testdata", "compiler", "bad__mufile_ext__2")

	// Check that the compiler warned about a bad Mufile extension (".txt").
	d := errors.WarnIllegalMufileExt
	assert.Equal(t, sink.Warnings(), 1, "expected a single warning")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkWarningPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.txt", fmt.Sprintf(d.Message, ".txt")),
		sink.WarningMsgs()[0])
}
