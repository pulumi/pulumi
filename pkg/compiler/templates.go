// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"text/template"

	"github.com/Masterminds/sprig"

	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/encoding"
)

// RenderTemplates performs standard template substitution on the given buffer using the given properties object.
// TODO[marapongo/mu#7]: render many templates at once so they can share code.
// TODO[marapongo/mu#7]: support configuration sections, etc., that can also contain templates.
func RenderTemplates(doc *diag.Document, ctx *core.Context) (*diag.Document, error) {
	r, err := newRenderer(doc, ctx)
	if err != nil {
		return nil, err
	}

	// Now actually render the template, supplying the context object as the data argument.
	b := bytes.NewBuffer(nil)
	if err = r.T.Execute(b, ctx); err != nil {
		return nil, err
	}
	fmt.Printf("==\n%v\n", b.String())

	return &diag.Document{
		File: doc.File,
		Body: b.Bytes(),
	}, nil
}

type renderer struct {
	T   *template.Template
	doc *diag.Document
	ctx *core.Context
}

func newRenderer(doc *diag.Document, ctx *core.Context) (*renderer, error) {
	r := &renderer{doc: doc, ctx: ctx}

	t := template.New(doc.File)

	// We will issue errors if the template tries to use a key that doesn't exist.
	// TODO[marapongo/mu#7]: consider having an option to relax this.
	t.Option("missingkey=error")

	// Add a stock set of helper functions to the template.
	t = t.Funcs(r.standardTemplateFuncs())

	// Parse up the resulting template from the provided document.
	var err error
	t, err = t.Parse(string(doc.Body))
	if err != nil {
		return nil, err
	}

	r.T = t
	return r, nil
}

// standardTemplateFuncs returns a new FuncMap containing all of the functions available to templates.  It is a
// member function of renderer because it closes over its state and may use it recursively.
func (r *renderer) standardTemplateFuncs() template.FuncMap {
	// Use the Sprig library to seed our map with a lot of useful functions.
	// TODO[marapongo/mu#7]: audit these and add them one-by-one, so any changes are intentional.  There also may be
	//     some that we don't actually want to offer.
	funcs := sprig.TxtFuncMap()

	// Include textually includes the given document, also expanding templates.
	funcs["include"] = func(name string) (string, error) {
		// Attempt to load the target file so that we may expand templates within it.
		dir := filepath.Dir(r.doc.File)
		path := filepath.Join(dir, name)
		raw, err := ioutil.ReadFile(path)
		if err != nil {
			return "", err
		}

		// Now perform the template expansion.
		b := bytes.NewBuffer(nil)
		u, err := r.T.Parse(string(raw))
		if err != nil {
			return "", err
		}
		if err := u.Execute(b, r.ctx); err != nil {
			return "", err
		}
		return b.String(), nil
	}

	// Add functions to unmarshal structures into their JSON/YAML textual equivalents.
	funcs["json"] = func(v interface{}) (string, error) {
		res, err := encoding.JSON.Marshal(v)
		return string(res), err
	}
	funcs["yaml"] = func(v interface{}) (string, error) {
		res, err := encoding.YAML.Marshal(v)
		return string(res), err
	}

	return funcs
}
