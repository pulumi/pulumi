// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"os"

	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
	"github.com/marapongo/mu/pkg/util"
	"github.com/marapongo/mu/pkg/workspace"
)

// Compiler provides an interface into the many phases of the Mu compilation process.
type Compiler interface {
	core.Phase

	// Context returns the current compiler context.
	Context() *core.Context

	// Build detects and compiles inputs from the given location, storing build artifacts in the given destination.
	Build(inp string, outp string)
	// BuildFile uses the given Mufile directly, and stores build artifacts in the given destination.
	BuildFile(mufile []byte, ext string, outp string)
}

// compiler is the canonical implementation of the Mu compiler.
type compiler struct {
	ctx  *core.Context
	opts Options
	deps map[ast.Ref]*diag.Document // a cache of mapping names to loaded dependencies.
}

// NewCompiler creates a new instance of the Mu compiler, with the given initialization settings.
func NewCompiler(opts Options) Compiler {
	return &compiler{
		ctx:  &core.Context{},
		opts: opts,
		deps: make(map[ast.Ref]*diag.Document),
	}
}

func (c *compiler) Context() *core.Context {
	return c.ctx
}

func (c *compiler) Diag() diag.Sink {
	return c.opts.Diag
}

func (c *compiler) Build(inp string, outp string) {
	glog.Infof("Building target '%v' (out='%v')", inp, outp)

	// First find the root of the current package based on the location of its Mufile.
	w, err := workspace.New(inp, c.Diag())
	if err != nil {
		c.Diag().Errorf(errors.ErrorIO.AtFile(inp), err)
		return
	}
	mufile, err := w.DetectMufile()
	if err != nil {
		c.Diag().Errorf(errors.ErrorIO.AtFile(inp), err)
		return
	}
	if mufile == "" {
		c.Diag().Errorf(errors.ErrorMissingMufile, inp)
		return
	}

	// Read in the contents of the document and make it available to subsequent stages.
	doc, err := diag.ReadDocument(mufile)
	if err != nil {
		c.Diag().Errorf(errors.ErrorCouldNotReadMufile.AtFile(mufile), err)
		return
	}

	c.buildDocument(w, doc, outp)
}

func (c *compiler) BuildFile(mufile []byte, ext string, outp string) {
	glog.Infof("Building in-memory %v file (bytes=%v out='%v')", ext, len(mufile), outp)

	// Default to the current working directory for the workspace.
	dir, err := os.Getwd()
	if err != nil {
		c.Diag().Errorf(errors.ErrorIO, err)
		return
	}
	w, err := workspace.New(dir, c.Diag())
	if err != nil {
		c.Diag().Errorf(errors.ErrorIO, err)
		return
	}

	doc := &diag.Document{File: workspace.Mufile + ext, Body: mufile}
	c.buildDocument(w, doc, outp)
}

func (c *compiler) buildDocument(w workspace.W, doc *diag.Document, outp string) {
	glog.Infof("Building doc '%v' (bytes=%v out='%v')", doc.File, len(doc.Body), outp)
	if glog.V(2) {
		defer func() {
			glog.V(2).Infof("Building doc '%v' completed w/ %v warnings and %v errors",
				doc.File, c.Diag().Warnings(), c.Diag().Errors())
		}()
	}

	// Perform the front-end phases of the compiler.
	stack := c.buildDocumentFE(w, doc)
	if !c.Diag().Success() {
		return
	}
	util.Assert(stack != nil)

	// Next, perform the semantic analysis phases of the compiler.
	c.buildDocumentSema(w, stack)
	if !c.Diag().Success() {
		return
	}

	// Finally, perform the back-end phases of the compiler.
	c.buildDocumentBE(w, stack)
}
