// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
	"github.com/marapongo/mu/pkg/workspace"
)

// Compiler provides an interface into the many phases of the Mu compilation process.
type Compiler interface {
	// Context returns the current compiler context.
	Context() *Context

	// Diag fetches the diagnostics sink used by this compiler instance.
	Diag() diag.Sink

	// Build detects and compiles inputs from the given location, storing build artifacts in the given destination.
	Build(inp string, outp string)
}

// compiler is the canonical implementation of the Mu compiler.
type compiler struct {
	ctx  *Context
	opts Options
}

// NewCompiler creates a new instance of the Mu compiler, with the given initialization settings.
func NewCompiler(opts Options) Compiler {
	return &compiler{
		ctx:  &Context{},
		opts: opts,
	}
}

func (c *compiler) Context() *Context {
	return c.ctx
}

func (c *compiler) Diag() diag.Sink {
	return c.opts.Diag
}

func (c *compiler) Build(inp string, outp string) {
	glog.Infof("Building target '%v' (out='%v')", inp, outp)
	if glog.V(2) {
		defer func() {
			glog.V(2).Infof("Building target '%v' completed w/ %v warnings and %v errors",
				inp, c.Diag().Warnings(), c.Diag().Errors())
		}()
	}

	// First find the root of the current package based on the location of its Mufile.
	mufile, err := workspace.DetectMufile(inp)
	if err != nil {
		c.Diag().Errorf(errors.MissingMufile, inp)
		return
	}

	// Read in the contents of the document and make it available to subsequent stages.
	doc, err := diag.ReadDocument(mufile)
	if err != nil {
		c.Diag().Errorf(errors.CouldNotReadMufile.WithFile(mufile), err)
		return
	}

	// To build the Mu package, first parse the input file.
	p := NewParser(c)
	stack := p.Parse(doc)
	if p.Diag().Errors() > 0 {
		// If any errors happened during parsing, we cannot proceed; exit now.
		return
	}

	// Do a pass over the parse tree to ensure that all is well.
	ptAnalyzer := NewPTAnalyzer(c)
	ptAnalyzer.Analyze(doc, stack)
	if p.Diag().Errors() > 0 {
		// If any errors happened during parse tree analysis, we cannot proceed; exit now.
		return
	}

	// TODO: here are some steps still remaining during compilation:
	// 		- read in dependencies (mu_modules or equivalent necessary).
	// 		- binding.
	// 		- decide if we need a "lower" form that includes bound nodes (likely yes).
	//		- semantic analysis (e.g., check that cloud targets aren't incompatible; argument checking; etc).
	// 		- read in cluster targets information if present.
	// 		- lower the ASTs to the provider's representation, and emit it.
}
