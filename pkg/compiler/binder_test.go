// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
)

func TestSymbolAlreadyExists(t *testing.T) {
	sink := builddir("testdata", "binder", "bad__symbol_already_exists")

	// Check that the compiler complained about a duplicate symbol.
	d := errors.SymbolAlreadyExists
	assert.Equal(t, 1, sink.Errors(), "expected a single error")
	assert.Equal(t,
		fmt.Sprintf("%v: %v%v: %v: %v\n",
			diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, "Mu.yaml",
			fmt.Sprintf(d.Message, "foo")),
		sink.ErrorMsgs()[0])
}
