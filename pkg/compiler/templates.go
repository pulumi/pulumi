// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/compiler/backends/clouds"
	"github.com/marapongo/mu/pkg/compiler/backends/schedulers"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/encoding"
	"github.com/marapongo/mu/pkg/util"
)

// RenderTemplates performs standard template substitution on the given buffer using the given properties object.
// TODO[marapongo/mu#7]: render many templates at once so they can share code.
// TODO[marapongo/mu#7]: support configuration sections, etc., that can also contain templates.
func RenderTemplates(doc *diag.Document, ctx *Context) (*diag.Document, error) {
	glog.V(2).Infof("Rendering template %v", doc.File)

	r, err := newRenderer(doc, ctx)
	if err != nil {
		return nil, err
	}

	// Now actually render the template.
	b, err := r.Render()
	if err != nil {
		return nil, err
	}

	glog.V(5).Infof("Rendered template %v:\n%v", doc.File, string(b))
	return &diag.Document{
		File: doc.File,
		Body: b,
	}, nil
}

type renderer struct {
	T   *template.Template
	doc *diag.Document
	ctx *renderContext
}

func newRenderer(doc *diag.Document, ctx *Context) (*renderer, error) {
	// Create a new renderer; note that the template will be set last.
	r := &renderer{doc: doc, ctx: newRenderContext(ctx)}

	// Now create the template; this is a multi-step process.
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

// Render renders the root template and returns the result, or an error, whichever occurs.
func (r *renderer) Render() ([]byte, error) {
	b := bytes.NewBuffer(nil)
	if err := r.T.Execute(b, r.ctx); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// standardTemplateFuncs returns a new FuncMap containing all of the functions available to templates.  It is a
// member function of renderer because it closes over its state and may use it recursively.
func (r *renderer) standardTemplateFuncs() template.FuncMap {
	// Use the Sprig library to seed our map with a lot of useful functions.
	// TODO[marapongo/mu#7]: audit these and add them one-by-one, so any changes are intentional.  There also may be
	//     some that we don't actually want to offer.
	funcs := sprig.TxtFuncMap()

	// Panic abruptly quits the template processing by injecting an ordinary error into it.
	funcs["panic"] = func(msg string, args ...interface{}) (string, error) {
		return "", fmt.Errorf(msg, args...)
	}

	// Require checks that a condition is true, and errors out if it does not.  This is useful for validation tasks.
	funcs["require"] = func(cond bool, msg string, args ...interface{}) (string, error) {
		if cond {
			return "", nil
		} else {
			return "", fmt.Errorf(msg, args...)
		}
	}

	// Include textually includes the given document, also expanding templates.
	funcs["include"] = func(name string) (string, error) {
		glog.V(3).Infof("Recursive include of template file: %v", name)

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

		s := b.String()
		glog.V(5).Infof("Recursively included template file %v:\n%v", name, s)
		return s, nil
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

	// Functions for interacting with maps.
	funcs["has"] = func(m map[string]interface{}, k string) bool {
		_, has := m[k]
		return has
	}

	// Functions for interacting with the mutable set of template variables.
	funcs["get"] = func(key string) interface{} {
		return r.ctx.Vars[key]
	}
	funcs["set"] = func(key string, v interface{}) string {
		r.ctx.Vars[key] = v
		return ""
	}

	return funcs
}

// renderContext is a "template-friendly" version of the Context object.  Namely, certain structured types are projected
// as strings for easier usage within markup templates.
type renderContext struct {
	Arch       renderArch      // the cloud architecture to target.
	Cluster    ast.Cluster     // the cluster we will deploy to.
	Properties ast.PropertyBag // a set of properties associated with the current stack.
	Vars       ast.PropertyBag // mutable variables used throughout this template's evaluation.
}

// renderArch is just like a normal Arch, except it has been expanded into strings for easier usage.
type renderArch struct {
	Cloud     string
	Scheduler string
}

func newRenderContext(ctx *Context) *renderContext {
	util.Assert(ctx != nil)
	util.Assert(ctx.Cluster != nil)
	util.Assert(ctx.Properties != nil)
	return &renderContext{
		Arch: renderArch{
			Cloud:     clouds.Names[ctx.Arch.Cloud],
			Scheduler: schedulers.Names[ctx.Arch.Scheduler],
		},
		Cluster:    *ctx.Cluster,
		Properties: ctx.Properties,
		Vars:       make(ast.PropertyBag),
	}
}
