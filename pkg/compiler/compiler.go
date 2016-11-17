// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/compiler/clouds"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/schedulers"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
	"github.com/marapongo/mu/pkg/workspace"
)

// Compiler provides an interface into the many phases of the Mu compilation process.
type Compiler interface {
	core.Phase

	// Context returns the current compiler context.
	Context() *core.Context

	// Build detects and compiles inputs from the given location, storing build artifacts in the given destination.
	Build(inp string, outp string)
	// BuildJSON uses the given JSON-based Mufile directly, and stores build artifacts in the given destination.
	BuildJSON(mufile []byte, outp string)
	// BuildYAML uses the given YAML-based Mufile directly, and stores build artifacts in the given destination.
	BuildYAML(mufile []byte, outp string)
}

// compiler is the canonical implementation of the Mu compiler.
type compiler struct {
	ctx  *core.Context
	opts Options
}

// NewCompiler creates a new instance of the Mu compiler, with the given initialization settings.
func NewCompiler(opts Options) Compiler {
	return &compiler{
		ctx:  &core.Context{},
		opts: opts,
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

	c.buildDocument(doc, outp)
}

func (c *compiler) BuildJSON(mufile []byte, outp string) {
	glog.Infof("Building in-memory JSON file (bytes=%v out='%v')", len(mufile), outp)
	c.buildDocument(&diag.Document{File: workspace.MufileBase + ".json", Body: mufile}, outp)
}

func (c *compiler) BuildYAML(mufile []byte, outp string) {
	glog.Infof("Building in-memory YAML file (bytes=%v out='%v')", len(mufile), outp)
	c.buildDocument(&diag.Document{File: workspace.MufileBase + ".yaml", Body: mufile}, outp)
}

func (c *compiler) buildDocument(doc *diag.Document, outp string) {
	glog.Infof("Building doc '%v' (bytes=%v out='%v')", doc.File, len(doc.Body), outp)
	if glog.V(2) {
		defer func() {
			glog.V(2).Infof("Building doc '%v' completed w/ %v warnings and %v errors",
				doc.File, c.Diag().Warnings(), c.Diag().Errors())
		}()
	}

	// Perform the front-end passes to generate a stack AST.
	stack, ok := c.parseStack(doc)
	if !ok {
		return
	}

	// Perform the semantic analysis passes to validate, transform, and/or update the AST.
	stack, ok = c.analyzeStack(doc, stack)
	if !ok {
		return
	}

	if !c.opts.SkipCodegen {
		// Figure out which cloud architecture we will be targeting during code-gen.
		target, arch, ok := c.discoverTargetArch(doc, stack)
		if !ok {
			return
		}
		if glog.V(2) {
			tname := "n/a"
			if target != nil {
				tname = target.Name
			}
			glog.V(2).Infof("Stack %v targets target=%v cloud=%v", stack.Name, tname, arch)
		}

		// TODO: lower the ASTs to the target backend's representation, emit it.
		// TODO: delta generation, deployment, etc.
	}
}

// loadAndParseStack takes a Mufile document, parses and validates it, and returns a stack AST.  If anything goes wrong
// during this process, the number of errors will be non-zero, and the bool will be false.
func (c *compiler) parseStack(doc *diag.Document) (*ast.Stack, bool) {
	// To build the Mu package, first parse the input file.
	p := NewParser(c)
	stack := p.Parse(doc)
	if p.Diag().Errors() > 0 {
		// If any errors happened during parsing, exit.
		return stack, false
	}

	// Do a pass over the parse tree to ensure that all is well.
	ptAnalyzer := NewPTAnalyzer(c)
	ptAnalyzer.Analyze(doc, stack)
	if p.Diag().Errors() > 0 {
		// If any errors happened during parse tree analysis, exit.
		return stack, false
	}

	return stack, true
}

// discoverTargetArch uses a variety of mechanisms to discover the target architecture, returning it.  If no
// architecture was discovered, an error is issued, and the bool return will be false.
func (c *compiler) discoverTargetArch(doc *diag.Document, stack *ast.Stack) (*ast.Target, Arch, bool) {
	// Target and architectures settings may come from one of three places, in order of search preference:
	//		1) command line arguments.
	//		2) settings specific to this stack.
	//		3) cluster-wide settings in a Mucluster file.
	// In other words, 1 overrides 2 which overrides 3.
	arch := c.opts.Arch

	// If a target was specified, look it up and load up its options.
	var target *ast.Target
	if c.opts.Target != "" {
		// First, check the stack to see if it has a targets section.
		if t, exists := stack.Targets[c.opts.Target]; exists {
			target = &t
		} else {
			// If that didn't work, see if there's a clusters file we can consult.
			// TODO: support Mucluster files.
			c.Diag().Errorf(errors.CloudTargetNotFound.WithDocument(doc), c.opts.Target)
			return target, arch, false
		}
	}

	// If no target was specified or discovered yet, see if there is a default one to use.
	if target == nil {
		for _, t := range stack.Targets {
			if t.Default {
				target = &t
				break
			}
		}
	}

	if target == nil {
		// If no target was found, and we don't have an architecture, error out.
		if arch.Cloud == clouds.NoArch {
			c.Diag().Errorf(errors.MissingTarget.WithDocument(doc))
			return target, arch, false
		}
	} else {
		// If a target was found, go ahead and extract and validate the target architecture.
		a, ok := c.getTargetArch(doc, target, arch)
		if !ok {
			return target, arch, false
		}
		arch = a
	}

	return target, arch, true
}

// getTargetArch gets and validates the architecture from an existing target.
func (c *compiler) getTargetArch(doc *diag.Document, target *ast.Target, existing Arch) (Arch, bool) {
	targetCloud := existing.Cloud
	targetScheduler := existing.Scheduler

	// If specified, look up the target's architecture settings.
	if target.Cloud != "" {
		tc, ok := clouds.ArchMap[target.Cloud]
		if !ok {
			c.Diag().Errorf(errors.UnrecognizedCloudArch.WithDocument(doc), target.Cloud)
			return existing, false
		}
		targetCloud = tc
	}
	if target.Scheduler != "" {
		ts, ok := schedulers.ArchMap[target.Scheduler]
		if !ok {
			c.Diag().Errorf(errors.UnrecognizedSchedulerArch.WithDocument(doc), target.Scheduler)
			return existing, false
		}
		targetScheduler = ts
	}

	// Ensure there aren't any conflicts, comparing compiler options to target settings.
	tarch := Arch{targetCloud, targetScheduler}
	if targetCloud != existing.Cloud && existing.Cloud != clouds.NoArch {
		c.Diag().Errorf(errors.ConflictingTargetArchSelection.WithDocument(doc), existing, target.Name, tarch)
		return tarch, false
	}
	if targetScheduler != existing.Scheduler && existing.Scheduler != schedulers.NoArch {
		c.Diag().Errorf(errors.ConflictingTargetArchSelection.WithDocument(doc), existing, target.Name, tarch)
		return tarch, false
	}

	return tarch, true
}

// analyzeStack performs semantic analysis on a stack -- validating, transforming, and/or updating it -- and then
// returns the result.  If a problem occurs, errors will have been emitted, and the bool return will be false.
func (c *compiler) analyzeStack(doc *diag.Document, stack *ast.Stack) (*ast.Stack, bool) {
	// TODO: load dependencies.

	binder := NewBinder(c)
	binder.Bind(doc, stack)
	if c.Diag().Errors() > 0 {
		// If any errors happened during binding, exit.
		return stack, false
	}

	// TODO: perform semantic analysis on the bound tree.

	return stack, true
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
