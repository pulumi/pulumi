// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/marapongo/mu/pkg/compiler/backends"
	"github.com/marapongo/mu/pkg/compiler/backends/clouds"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
)

func TestBadMissingMufile(t *testing.T) {
	sink := buildNoCodegen("testdata", "compiler", "bad__missing_mufile")

	// Check that the compiler complained about a missing Mufile.
	d := errors.ErrorMissingMufile
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, fmt.Sprintf(d.Message, sink.Pwd)),
		sink.ErrorMsgs()[0])
}

func TestBadMufileCasing(t *testing.T) {
	sink := buildNoCodegen("testdata", "compiler", "bad__mufile_casing")

	// Check that the compiler warned about a bad Mufile casing (mu.yaml).
	d := errors.WarningIllegalMarkupFileCasing
	assert.Equal(t, 1, sink.Warnings(), "expected a single warning")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkWarningPrefix, diag.DefaultSinkIDPrefix, d.ID, "mu.yaml", fmt.Sprintf(d.Message, "Mu")),
		sink.WarningMsgs()[0])
}

func TestBadMufileExt1(t *testing.T) {
	sink := buildNoCodegen("testdata", "compiler", "bad__mufile_ext__1")

	// Check that the compiler warned about a bad Mufile extension (none).
	d := errors.WarningIllegalMarkupFileExt
	assert.Equal(t, 1, sink.Warnings(), "expected a single warning")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkWarningPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu",
			fmt.Sprintf(d.Message, "Mu", "")),
		sink.WarningMsgs()[0])
}

func TestBadMufileExt2(t *testing.T) {
	sink := buildNoCodegen("testdata", "compiler", "bad__mufile_ext__2")

	// Check that the compiler warned about a bad Mufile extension (".txt").
	d := errors.WarningIllegalMarkupFileExt
	assert.Equal(t, 1, sink.Warnings(), "expected a single warning")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkWarningPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.txt",
			fmt.Sprintf(d.Message, "Mu", ".txt")),
		sink.WarningMsgs()[0])
}

func TestMissingTarget(t *testing.T) {
	mufile := []byte("name: notarget\n" +
		"abstract: true\n")

	// Check that the compiler issued an error due to missing cloud targets.
	sink := buildFile(&Options{}, mufile, ".yaml")
	d := errors.ErrorMissingTarget
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, d.Message), sink.ErrorMsgs()[0])

	// Now check that this same project compiles fine if we manually specify an architecture.
	sink = buildFile(&Options{
		Arch: backends.Arch{
			Cloud: clouds.AWS,
		},
	}, mufile, ".yaml")
	assert.Equal(t, 0, sink.Errors(), "expected no compilation errors")
}

func TestUnrecognizedCloud(t *testing.T) {
	sink := buildNoCodegen("testdata", "compiler", "bad__unrecognized_cloud")

	// Check that the compiler issued an error about an unrecognized cloud.
	d := errors.ErrorUnrecognizedCloudArch
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, fmt.Sprintf(d.Message, "badcloud")),
		sink.ErrorMsgs()[0])
}
