// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"fmt"

	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
	"github.com/marapongo/mu/pkg/workspace"
)

// Compiler provides an interface into the many phases of the Mu compilation process.
type Compiler interface {
	// Diags fetches the diagnostics sink used by this compiler instance.
	Diags() diag.Sink

	// Build detects and compiles inputs from the given location, storing build artifacts in the given destination.
	Build(inp string, outp string)
}

// compiler is the canonical implementation of the Mu compiler.
type compiler struct {
	opts Options
}

// NewCompiler creates a new instance of the Mu compiler, with the given initialization settings.
func NewCompiler(opts Options) Compiler {
	return &compiler{opts}
}

func (c *compiler) Diags() diag.Sink {
	return c.opts.Diags
}

func (c *compiler) Build(inp string, outp string) {
	glog.Infof("Building target directory '%v' to '%v'", inp, outp)

	// First find the root of the current package based on the location of its Mufile.
	mufile, err := workspace.DetectMufile(inp)
	if err != nil {
		c.Diags().Errorf(errors.MissingMufile, inp)
		return
	}

	// To build the Mu package, first parse the input file.
	p := NewParser(c)
	stack := p.Parse(mufile)

	// If any errors happened during parsing, we cannot proceed; exit now.
	if c.Diags().Errors() > 0 {
		return
	}

	fmt.Printf("PARSED: %v\n", stack)

}
