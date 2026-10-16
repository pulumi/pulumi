// Copyright 2026, Pulumi Corporation.
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

// gen-sdk-shims writes forwarding packages into the sdk module for every package that was moved to the shared
// module, so that existing imports of github.com/pulumi/pulumi/sdk/v3/... keep compiling.
//
// Usage (from the sdk directory): go run ./go/internal/gen-sdk-shims ../shared .
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/types"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

const (
	sharedModule = "github.com/pulumi/pulumi/shared/v3"
	fileBase     = "forward"
)

var platforms = []struct{ goos, goarch string }{
	{"linux", "amd64"},
	{"darwin", "arm64"},
	{"windows", "amd64"},
	{"freebsd", "amd64"},
	{"js", "wasm"},
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: gen-sdk-shims <shared dir> <sdk dir>")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// A forwarded declaration. Every package it refers to is qualified with an alias unique to the path, so that
// declarations can be combined into one file without clashes.
type decl struct {
	src     string
	imports map[string]string // path -> alias
}

type pkgInfo struct {
	name  string
	doc   string
	order []string
	decls map[string]decl
}

func run(sharedDir, sdkDir string) error {
	infos := map[string]*pkgInfo{}
	perPlatform := make([]map[string]map[string]bool, len(platforms))
	for i, p := range platforms {
		cfg := &packages.Config{
			Mode: packages.NeedName | packages.NeedTypes | packages.NeedSyntax | packages.NeedFiles,
			Dir:  sharedDir,
			Env:  append(os.Environ(), "GOOS="+p.goos, "GOARCH="+p.goarch, "CGO_ENABLED=0"),
		}
		pkgs, err := packages.Load(cfg, "./...")
		if err != nil {
			return err
		}
		perPlatform[i] = map[string]map[string]bool{}
		for _, pkg := range pkgs {
			if !shimmable(pkg) {
				continue
			}
			if len(pkg.Errors) > 0 {
				fmt.Fprintf(os.Stderr, "warning: skipping %s on %s: %v\n", pkg.PkgPath, p.goos, pkg.Errors[0])
				continue
			}
			info := infos[pkg.PkgPath]
			if info == nil {
				info = &pkgInfo{name: pkg.Name, decls: map[string]decl{}}
				infos[pkg.PkgPath] = info
			}
			if info.doc == "" {
				info.doc = packageDoc(pkg)
			}
			set := map[string]bool{}
			for _, d := range declsFor(pkg) {
				set[d.src] = true
				if _, ok := info.decls[d.src]; !ok {
					info.decls[d.src] = d
					info.order = append(info.order, d.src)
				}
			}
			perPlatform[i][pkg.PkgPath] = set
		}
	}

	paths := make([]string, 0, len(infos))
	for p := range infos {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, path := range paths {
		info := infos[path]
		dir := filepath.Join(sdkDir, strings.TrimPrefix(path, sharedModule+"/"))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		old, err := filepath.Glob(filepath.Join(dir, fileBase+"*.go"))
		if err != nil {
			return err
		}
		for _, f := range old {
			if err := os.Remove(f); err != nil {
				return err
			}
		}

		// Group declarations by the set of platforms they appear on. The unconstrained group always exists so that
		// the package has a file (and its doc comment) everywhere.
		all := make([]string, len(platforms))
		for i, p := range platforms {
			all[i] = p.goos
		}
		groups := map[string][]string{strings.Join(all, ","): nil}
		for _, src := range info.order {
			var on []string
			for i, p := range platforms {
				if perPlatform[i][path][src] {
					on = append(on, p.goos)
				}
			}
			k := strings.Join(on, ",")
			groups[k] = append(groups[k], src)
		}
		for k, srcs := range groups {
			on := strings.Split(k, ",")
			name, constraint, doc := fileBase+".go", "", info.doc
			if len(on) != len(platforms) {
				name, constraint, doc = fileBase+"_"+strings.Join(on, "_")+"_os.go", buildConstraint(on), ""
			}
			src, err := render(path, info, srcs, constraint, doc)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			if err := os.WriteFile(filepath.Join(dir, name), src, 0o600); err != nil {
				return err
			}
		}
	}
	return nil
}

func shimmable(pkg *packages.Package) bool {
	return pkg.Name != "main" && importable(pkg.PkgPath) && !strings.Contains(pkg.PkgPath, "/testdata/")
}

// importable reports whether a package can be imported from the sdk module.
func importable(path string) bool {
	if !strings.HasPrefix(path, sharedModule+"/") {
		return true
	}
	return !slices.Contains(strings.Split(path, "/"), "internal")
}

// buildConstraint prefers negative constraints so that unlisted platforms (e.g. openbsd) behave like their unix
// siblings.
func buildConstraint(on []string) string {
	var missing []string
	for _, p := range platforms {
		if !slices.Contains(on, p.goos) {
			missing = append(missing, "!"+p.goos)
		}
	}
	if slices.Contains(on, "linux") {
		return strings.Join(missing, " && ")
	}
	return strings.Join(on, " || ")
}

func packageDoc(pkg *packages.Package) string {
	for _, f := range pkg.Syntax {
		if f.Doc != nil {
			return f.Doc.Text()
		}
	}
	return ""
}

func comment(text string) string {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return ""
	}
	var b strings.Builder
	for l := range strings.SplitSeq(text, "\n") {
		if l == "" {
			b.WriteString("//\n")
		} else {
			b.WriteString("// " + l + "\n")
		}
	}
	return b.String()
}

// docs maps every top-level name to its doc comment.
func docs(pkg *packages.Package) map[string]string {
	res := map[string]string{}
	for _, f := range pkg.Syntax {
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Doc != nil {
					res[d.Name.Name] = d.Doc.Text()
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					doc := d.Doc
					if len(d.Specs) > 1 {
						doc = nil
					}
					var names []*ast.Ident
					switch s := spec.(type) {
					case *ast.TypeSpec:
						names = []*ast.Ident{s.Name}
						if s.Doc != nil {
							doc = s.Doc
						}
					case *ast.ValueSpec:
						names = s.Names
						if s.Doc != nil {
							doc = s.Doc
						}
					}
					if doc == nil {
						continue
					}
					for _, n := range names {
						res[n.Name] = doc.Text()
					}
				}
			}
		}
	}
	return res
}

func declsFor(pkg *packages.Package) []decl {
	scope := pkg.Types.Scope()
	docMap := docs(pkg)
	var out []decl
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}
		src, imports := forward(obj)
		if src == "" {
			continue
		}
		out = append(out, decl{src: comment(docMap[name]) + src, imports: imports})
	}
	return out
}

// alias returns an import alias that is unique to path.
func alias(self *types.Package, p *types.Package) string {
	if p == self {
		return "shared"
	}
	if !strings.Contains(strings.Split(p.Path(), "/")[0], ".") {
		// The standard library has no clashing names in exported signatures.
		return p.Name()
	}
	s := strings.NewReplacer("github.com/pulumi/pulumi/shared/v3/", "", "/", "_", ".", "_", "-", "_").
		Replace(p.Path())
	return "fwd_" + s
}

func forward(obj types.Object) (string, map[string]string) {
	self := obj.Pkg()
	imports := map[string]string{self.Path(): "shared"}
	q := func(p *types.Package) string {
		a := alias(self, p)
		imports[p.Path()] = a
		return a
	}
	name := obj.Name()
	simple := func(kind string) (string, map[string]string) {
		return fmt.Sprintf("%s %s = shared.%s\n", kind, name, name), map[string]string{self.Path(): "shared"}
	}
	switch obj := obj.(type) {
	case *types.Const:
		return simple("const")
	case *types.Var:
		// Copying a lock would stop it excluding anything, so forward a pointer: method calls still compile.
		if n, ok := obj.Type().(*types.Named); ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "sync" {
			return fmt.Sprintf("var %s = &shared.%s\n", name, name), map[string]string{self.Path(): "shared"}
		}
		return simple("var")
	case *types.TypeName:
		tparams := typeParamsOf(obj.Type())
		if tparams == nil || tparams.Len() == 0 {
			return simple("type")
		}
		decl, args := typeParamList(tparams, q)
		src := fmt.Sprintf("type %s%s = shared.%s%s\n", name, decl, name, args)
		if !referable(src, self, imports) {
			fmt.Fprintf(os.Stderr, "warning: cannot forward generic type %s.%s\n", self.Path(), name)
			return "", nil
		}
		return src, imports
	case *types.Func:
		sig := obj.Type().(*types.Signature)
		src := wrapper(obj, sig, q)
		if referable(src, self, imports) {
			return src, imports
		}
		if sig.TypeParams().Len() > 0 {
			fmt.Fprintf(os.Stderr, "warning: cannot forward generic func %s.%s\n", self.Path(), name)
			return "", nil
		}
		return simple("var")
	}
	return "", nil
}

func typeParamsOf(t types.Type) *types.TypeParamList {
	switch t := t.(type) {
	case *types.Named:
		return t.TypeParams()
	case *types.Alias:
		return t.TypeParams()
	}
	return nil
}

func typeParamList(tps *types.TypeParamList, q types.Qualifier) (string, string) {
	decl, args := make([]string, 0, tps.Len()), make([]string, 0, tps.Len())
	for tp := range tps.TypeParams() {
		decl = append(decl, tp.Obj().Name()+" "+types.TypeString(tp.Constraint(), q))
		args = append(args, tp.Obj().Name())
	}
	return "[" + strings.Join(decl, ", ") + "]", "[" + strings.Join(args, ", ") + "]"
}

// referable reports whether src only mentions exported names of the forwarded package and importable packages.
func referable(src string, self *types.Package, imports map[string]string) bool {
	for path := range imports {
		if !importable(path) {
			return false
		}
	}
	for _, n := range self.Scope().Names() {
		if !ast.IsExported(n) && strings.Contains(src, "shared."+n) {
			return false
		}
	}
	return true
}

func wrapper(obj *types.Func, sig *types.Signature, q types.Qualifier) string {
	tdecl, targs := "", ""
	if sig.TypeParams().Len() > 0 {
		tdecl, targs = typeParamList(sig.TypeParams(), q)
	}
	params := sig.Params()
	var ps, args []string
	used := map[string]bool{"shared": true}
	for i := range params.Len() {
		p := params.At(i)
		n := p.Name()
		if n == "" || n == "_" || used[n] || strings.HasPrefix(n, "fwd_") {
			n = fmt.Sprintf("p%d", i)
		}
		used[n] = true
		if sig.Variadic() && i == params.Len()-1 {
			ps = append(ps, n+" ..."+types.TypeString(p.Type().(*types.Slice).Elem(), q))
			args = append(args, n+"...")
		} else {
			ps = append(ps, n+" "+types.TypeString(p.Type(), q))
			args = append(args, n)
		}
	}
	rs := make([]string, 0, sig.Results().Len())
	for v := range sig.Results().Variables() {
		rs = append(rs, types.TypeString(v.Type(), q))
	}
	res := ""
	switch len(rs) {
	case 0:
	case 1:
		res = " " + rs[0]
	default:
		res = " (" + strings.Join(rs, ", ") + ")"
	}
	call := fmt.Sprintf("shared.%s%s(%s)", obj.Name(), targs, strings.Join(args, ", "))
	if len(rs) > 0 {
		call = "return " + call
	}
	return fmt.Sprintf("func %s%s(%s)%s {\n\t%s\n}\n", obj.Name(), tdecl, strings.Join(ps, ", "), res, call)
}

func render(path string, info *pkgInfo, srcs []string, constraint, doc string) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(`// Copyright 2026, Pulumi Corporation.
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

// Code generated by gen-sdk-shims. DO NOT EDIT.

`)
	if constraint != "" {
		fmt.Fprintf(&b, "//go:build %s\n\n", constraint)
	}
	b.WriteString(comment(doc))
	if constraint == "" {
		if doc != "" {
			b.WriteString("//\n")
		}
		fmt.Fprintf(&b, "// This package forwards to %s.\n", path)
	}
	fmt.Fprintf(&b, "package %s\n\n", info.name)

	imports := map[string]string{}
	for _, s := range srcs {
		maps.Copy(imports, info.decls[s].imports)
	}
	sorted := make([]string, 0, len(imports))
	for p := range imports {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)
	switch {
	case len(srcs) == 0:
		fmt.Fprintf(&b, "import _ %q\n", path)
	default:
		b.WriteString("import (\n")
		for _, p := range sorted {
			fmt.Fprintf(&b, "\t%s %q\n", imports[p], p)
		}
		b.WriteString(")\n\n")
	}
	for _, s := range srcs {
		b.WriteString(s)
		b.WriteString("\n")
	}
	out, err := format.Source(b.Bytes())
	if err != nil {
		return nil, fmt.Errorf("%w\n%s", err, b.String())
	}
	return out, nil
}
