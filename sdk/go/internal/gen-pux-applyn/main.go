// Copyright 2016-2023, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// gen-pux-applyn generates Apply functions for the pulumix package.
// For each N in 1..8, it generates the following functions
// (type parameters elided for brevity):
//
//	func ApplyN(T1, ..., TN, func(I1, ..., IN) O) Output[O]
//	func ApplyNErr(T1, ..., TN, func(I1, ..., IN) (O, error)) Output[O]
//	func ApplyNContext(context.Context, T1, ..., TN, func(I1, ..., IN) O) Output[O]
//	func ApplyNContextErr(context.Context, T1, ..., TN, func(I1, ..., IN) (O, error)) Output[O]
//
//	// Where each Ti satisfies Input[Ii].
//
// The output of this program should be placed in the pulumix package.
// It assumes availability of unexported types and functions from that package.
package main

import (
	"bytes"
	"embed"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io"
	"log"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func main() {
	log.SetFlags(0) // don't print timestamps

	var p params
	flag.StringVar(&p.Dir, "d", "", "output directory (default: current directory)")
	flag.IntVar(&p.N, "n", 0, "generate Apply functions for 1..n parameters")
	flag.Parse()

	if err := run(&p); err != nil {
		log.Fatal(err)
	}
}

type params struct {
	Dir string // output directory
	N   int    // max parameter count for an Apply function
}

func run(p *params) error {
	if p.N == 0 {
		p.N = 8
	}

	if p.Dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		p.Dir = cwd
	}

	g := generator{Dir: p.Dir}
	g.Generate(1, p.N)
	return g.Err()
}

var (
	//go:embed tmpl/*
	_tmplFS embed.FS

	_tmpl = template.Must(
		template.New("").
			Funcs(template.FuncMap{
				"seq": seq,
			}).
			ParseFS(_tmplFS, "tmpl/*"),
	)
)

// seq generates a slice in the range [start, stop] (inclusive).
// It is necessary because text/template does not support three-clause for loops.
func seq(start, stop int) []int {
	result := make([]int, stop-start+1)
	for i := range result {
		result[i] = start + i
	}
	return result
}

type generator struct {
	Dir string // output directory

	// Errors encountered during generation.
	errors []error
}

// Err returns an error if any errors were encountered during generation.
func (g *generator) Err() error {
	if len(g.errors) == 0 {
		return nil
	}
	return errors.Join(g.errors...)
}

// Generate generates apply functions for parameters in the range
// [start, stop] (inclusive).
func (g *generator) Generate(start, stop int) {
	data := struct {
		Start int
		Stop  int
		Year  int
	}{
		Start: start,
		Stop:  stop,
		Year:  time.Now().Year(),
	}

	var src, test bytes.Buffer
	g.executeTemplate(&src, "applyn.go.tmpl", data)
	g.executeTemplate(&test, "applyn_test.go.tmpl", data)
	g.writeGoFile("applyn.go", src.Bytes())
	g.writeGoFile("applyn_test.go", test.Bytes())
}

func (g *generator) executeTemplate(w io.Writer, name string, data any) {
	err := _tmpl.ExecuteTemplate(w, name, data)
	if err != nil {
		g.errf("execute template %s: %w", name, err)
	}
}

func (g *generator) writeGoFile(relpath string, bs []byte) {
	src, err := format.Source(bs)
	if err != nil {
		log.Printf("Invalid generated code:\n%s", bs)
		g.errf("format %s: %w", relpath, err)
		return
	}

	f, err := os.Create(filepath.Join(g.Dir, relpath))
	if err != nil {
		g.errf("create %s: %w", relpath, err)
		return
	}
	defer contract.IgnoreClose(f)

	_, err = f.Write(src)
	if err != nil {
		g.errf("write %s: %w", relpath, err)
	}
}

func (g *generator) errf(format string, args ...any) {
	g.errors = append(g.errors, fmt.Errorf(format, args...))
}
