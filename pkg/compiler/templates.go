// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"bytes"
	"text/template"

	"github.com/Masterminds/sprig"

	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/encoding"
	"github.com/marapongo/mu/pkg/util"
)

// RenderTemplates performs standard template substitution on the given buffer using the given properties object.
// TODO[marapongo/mu#7]: render many templates at once so they can share code.
// TODO[marapongo/mu#7]: support configuration sections, etc., that can also contain templates.
func RenderTemplates(doc *diag.Document, ctx *core.Context) (*diag.Document, error) {
	t := template.New(doc.File)

	// We will issue errors if the template tries to use a key that doesn't exist.
	// TODO[marapongo/mu#7]: consider having an option to relax this.
	t.Option("missingkey=error")

	// Add a stock set of helper functions to the template.
	t = t.Funcs(standardTemplateFuncs())

	// Now actually render the template, supplying the context object as the data argument.
	var err error
	t, err = t.Parse(string(doc.Body))
	if err != nil {
		return nil, err
	}
	b := bytes.NewBuffer(nil)
	if err = t.Execute(b, ctx); err != nil {
		return nil, err
	}
	return &diag.Document{
		File: doc.File,
		Body: b.Bytes(),
	}, nil
}

func standardTemplateFuncs() template.FuncMap {
	// Use the Sprig library to seed our map with a lot of useful functions.
	// TODO[marapongo/mu#7]: audit these and add them one-by-one, so any changes are intentional.  There also may be
	//     some that we don't actually want to offer.
	funcs := sprig.TxtFuncMap()

	// Add functions to unmarshal structures into their JSON/YAML textual equivalents.
	funcs["json"] = func(v interface{}) string {
		res, err := encoding.JSON.Marshal(v)
		util.AssertMF(err == nil, "Unexpected JSON marshaling error: %v", err)
		return string(res)
	}
	funcs["yaml"] = func(v interface{}) string {
		res, err := encoding.YAML.Marshal(v)
		util.AssertMF(err == nil, "Unexpected YAML marshaling error: %v", err)
		return string(res)
	}

	return funcs
}
