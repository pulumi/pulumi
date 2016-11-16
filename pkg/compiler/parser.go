// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/api"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
)

type Parser interface {
	// Diag fetches the diagnostics sink used by this parser.
	Diag() diag.Sink

	// Parse detects and parses input from the given path.  If an error occurs, the return value will be nil.  It is
	// expected that errors are conveyed using the diag.Sink interface.
	Parse(inp string) *api.Stack
}

func NewParser(c Compiler) Parser {
	return &parser{c}
}

type parser struct {
	c Compiler
}

func (p *parser) Diag() diag.Sink {
	return p.c.Diag()
}

func (p *parser) Parse(mufile string) *api.Stack {
	glog.Infof("Parsing Mufile '%v'", mufile)
	if glog.V(2) {
		defer func() {
			glog.V(2).Infof("Parsing Mufile '%v' completed w/ %v warnings and %v errors",
				mufile, p.Diag().Warnings(), p.Diag().Errors())
		}()
	}

	// We support both JSON and YAML as a file format.  Detect the file extension and deserialize the contents.
	ext := filepath.Ext(mufile)
	switch ext {
	case ".json":
		return p.parseFromJSON(mufile)
	case ".yaml":
		return p.parseFromYAML(mufile)
	default:
		p.Diag().Errorf(errors.IllegalMufileExt.WithFile(mufile), ext)
		return nil
	}
}

func (p *parser) parseFromJSON(mufile string) *api.Stack {
	body, err := ioutil.ReadFile(mufile)
	if err != nil {
		p.Diag().Errorf(errors.CouldNotReadMufile.WithFile(mufile), err)
		return nil
	}

	var stack api.Stack
	if err := json.Unmarshal(body, &stack); err != nil {
		p.Diag().Errorf(errors.IllegalMufileSyntax.WithFile(mufile), err)
		// TODO: it would be great if we issued an error per issue found in the file with line/col numbers.
		return nil
	}
	return &stack
}

func (p *parser) parseFromYAML(mufile string) *api.Stack {
	body, err := ioutil.ReadFile(mufile)
	if err != nil {
		p.Diag().Errorf(errors.CouldNotReadMufile.WithFile(mufile), err)
		return nil
	}

	var stack api.Stack
	if err := yaml.Unmarshal(body, &stack); err != nil {
		p.Diag().Errorf(errors.IllegalMufileSyntax.WithFile(mufile), err)
		// TODO: it would be great if we issued an error per issue found in the file with line/col numbers.
		return nil
	}
	return &stack
}
