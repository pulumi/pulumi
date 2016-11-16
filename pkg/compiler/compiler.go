// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
	"github.com/marapongo/mu/pkg/workspace"
)

type Pass interface {
	// Diag fetches the diagnostics sink used by this compiler pass.
	Diag() diag.Sink
}

// Compiler provides an interface into the many phases of the Mu compilation process.
type Compiler interface {
	Pass

	// Context returns the current compiler context.
	Context() *Context

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
	mufile := c.detectMufile(inp)
	if mufile == "" {
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
		// If any errors happened during parsing, exit.
		return
	}

	// Do a pass over the parse tree to ensure that all is well.
	ptAnalyzer := NewPTAnalyzer(c)
	ptAnalyzer.Analyze(doc, stack)
	if p.Diag().Errors() > 0 {
		// If any errors happened during parse tree analysis, exit.
		return
	}

	// TODO: load dependencies.

	binder := NewBinder(c)
	binder.Bind(doc, stack)
	if p.Diag().Errors() > 0 {
		// If any errors happened during binding, exit.
		return
	}

	// TODO: perform semantic analysis on the bound tree.
	// TODO: select a target backend (including reading in a Muclusters file if needed).
	// TODO: lower the ASTs to the target backend's representation, emit it.
	// TODO: delta generation, deployment, etc.
}

// detectMufile locates the closest Mufile-looking file from the given path, searching "upwards" in the directory
// hierarchy.  If no Mufile is found, an empty path is returned.
func (c *compiler) detectMufile(from string) string {
	abs, err := filepath.Abs(from)
	if err != nil {
		glog.Fatalf("An IO error occurred while searching for a Mufile: %v", err)
		return ""
	}

	// It's possible the target is already the file we seek; if so, return right away.
	if c.isMufile(abs) {
		return abs
	}

	curr := abs
	for {
		stop := false

		// If the target is a directory, enumerate its files, checking each to see if it's a Mufile.
		files, err := ioutil.ReadDir(curr)
		if err != nil {
			glog.Fatalf("An IO error occurred while searching for a Mufile: %v", err)
			return ""
		}
		for _, file := range files {
			name := file.Name()
			path := filepath.Join(curr, name)
			if c.isMufile(path) {
				return path
			} else if name == workspace.Muspace {
				// If we hit a .muspace file, stop looking.
				stop = true
			}
		}

		// If we encountered a stop condition, break out of the loop.
		if stop {
			break
		}

		// If neither succeeded, keep looking in our parent directory.
		curr = filepath.Dir(curr)
		if os.IsPathSeparator(curr[len(curr)-1]) {
			break
		}
	}

	return ""
}

// isMufile returns true if the path references what appears to be a valid Mufile.
func (c *compiler) isMufile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// Directories can't be Mufiles.
	if info.IsDir() {
		return false
	}

	// Ensure the base name is expected.
	name := info.Name()
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	if base != workspace.MufileBase {
		if strings.EqualFold(base, workspace.MufileBase) {
			// If the strings aren't equal, but case-insensitively match, issue a warning.
			c.Diag().Warningf(errors.WarnIllegalMufileCasing.WithFile(name))
		}
		return false
	}

	// Check all supported extensions.
	for _, mufileExt := range workspace.MufileExts {
		if name == workspace.MufileBase+mufileExt {
			return true
		}
	}

	// If we got here, it means the base name matched, but not the extension.  Warn and return.
	c.Diag().Warningf(errors.WarnIllegalMufileExt.WithFile(name), ext)
	return false
}
