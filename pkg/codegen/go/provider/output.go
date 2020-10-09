package provider

import (
	"encoding/json"
	"fmt"
	"go/types"
	"io"
	"os/exec"
	"reflect"
	"sort"
	"text/template"
)

type typeInfo struct {
	Kind    reflect.Kind `json:"kind"`
	PkgPath string       `json:"pkgPath"`
	Name    string       `json:"name"`
	Elem    *typeInfo    `json:"elem"`
	Key     *typeInfo    `json:"key"`
	Len     int          `json:"len"`
}

func (t *typeInfo) resolve(imports map[string]*types.Package) (types.Type, error) {
	if t.PkgPath != "" && t.Name != "" {
		pkg, ok := imports[t.PkgPath]
		if !ok {
			return nil, fmt.Errorf("could not find package %v", t.PkgPath)
		}
		obj := pkg.Scope().Lookup(t.Name)
		if obj == nil {
			return nil, fmt.Errorf("coud not find type %v in package %v", t.Name, t.PkgPath)
		}
		return obj.Type(), nil
	}

	switch t.Kind {
	case reflect.Invalid:
		return types.Typ[types.Invalid], nil
	case reflect.Bool:
		return types.Typ[types.Bool], nil
	case reflect.Int:
		return types.Typ[types.Int], nil
	case reflect.Int8:
		return types.Typ[types.Int8], nil
	case reflect.Int16:
		return types.Typ[types.Int16], nil
	case reflect.Int32:
		return types.Typ[types.Int32], nil
	case reflect.Int64:
		return types.Typ[types.Int64], nil
	case reflect.Uint:
		return types.Typ[types.Uint], nil
	case reflect.Uint8:
		return types.Typ[types.Uint8], nil
	case reflect.Uint16:
		return types.Typ[types.Uint16], nil
	case reflect.Uint32:
		return types.Typ[types.Uint32], nil
	case reflect.Uint64:
		return types.Typ[types.Uint64], nil
	case reflect.Uintptr:
		return types.Typ[types.Uintptr], nil
	case reflect.Float32:
		return types.Typ[types.Float32], nil
	case reflect.Float64:
		return types.Typ[types.Float64], nil
	case reflect.Complex64:
		return types.Typ[types.Complex64], nil
	case reflect.Complex128:
		return types.Typ[types.Complex128], nil
	case reflect.String:
		return types.Typ[types.String], nil
	case reflect.UnsafePointer:
		return types.Typ[types.UnsafePointer], nil

	case reflect.Array:
		elem, err := t.Elem.resolve(imports)
		if err != nil {
			return nil, err
		}
		return types.NewArray(elem, int64(t.Len)), nil
	case reflect.Interface:
		if t.Len != 0 {
			return nil, fmt.Errorf("unsupported unnamed type of kind %v", t.Kind)
		}
		return types.NewInterface(nil, nil), nil
	case reflect.Map:
		key, err := t.Key.resolve(imports)
		if err != nil {
			return nil, err
		}
		elem, err := t.Elem.resolve(imports)
		if err != nil {
			return nil, err
		}
		return types.NewMap(key, elem), nil
	case reflect.Ptr:
		elem, err := t.Elem.resolve(imports)
		if err != nil {
			return nil, err
		}
		return types.NewPointer(elem), nil
	case reflect.Slice:
		elem, err := t.Elem.resolve(imports)
		if err != nil {
			return nil, err
		}
		return types.NewSlice(elem), nil
	default:
		return nil, fmt.Errorf("unsupported unnamed type of kind %v", t.Kind)
	}
}

func collectImports(result map[string]*types.Package, p *types.Package) {
	if _, ok := result[p.Path()]; ok {
		return
	}

	result[p.Path()] = p
	for _, p := range p.Imports() {
		collectImports(result, p)
	}
}

func (p *pulumiPackage) resolveOutputTypes() error {
	const templateSource = `package main

import (
	"encoding/json"
	"log"
	"os"
	"reflect"

{{range $path, $name := .Packages}}
	{{$name}} "{{$path}}"
{{end}}
)

type typeInfo map[string]interface{}

func getTypeInfo(t reflect.Type) typeInfo {
	info := typeInfo{
		"kind": t.Kind(),
		"pkgPath": t.PkgPath(),
		"name": t.Name(),
	}

	switch t.Kind() {
	case reflect.Array:
		info["elem"], info["len"] = getTypeInfo(t.Elem()), t.Len()
	case reflect.Slice, reflect.Ptr:
		info["elem"] = getTypeInfo(t.Elem())
	case reflect.Map:
		info["key"], info["elem"] = getTypeInfo(t.Key()), getTypeInfo(t.Elem())
	case reflect.Interface:
		info["len"] = t.NumMethod()
	}

	return info
}

func main() {
	elementTypes := map[string]typeInfo{
		{{range $id, $type := .Types}}
		"{{$id}}": getTypeInfo(({{$type}}{}).ElementType()),
		{{end}}
	}

	if err := json.NewEncoder(os.Stdout).Encode(elementTypes); err != nil {
		log.Fatalf("failed to encode element type info: %v", err)
	}
}
`

	if len(p.outputTypes) == 0 {
		return nil
	}

	packages := map[string]string{}
	typs := map[string]string{}

	// Build the import set.
	for _, typ := range p.outputTypes {
		obj := typ.(*types.Named).Obj()
		pkg := obj.Pkg().Path()

		importName, ok := packages[pkg]
		if !ok {
			n := fmt.Sprintf("pkg%v", len(packages))
			importName, packages[pkg] = n, n
		}
		typs[obj.Id()] = fmt.Sprintf("%s.%s", importName, obj.Name())
	}

	// Spit out a source file that imports the required packages, then computes their type specs and serializes them
	// to stdout.
	tmpl, err := template.New("output_types.go").Parse(templateSource)
	if err != nil {
		return fmt.Errorf("failed to write output type: %w", err)
	}

	tempFilePath, err := writeTempFile("output_types_*.go", func(w io.Writer) error {
		return tmpl.Execute(w, map[string]interface{}{
			"Packages": packages,
			"Types":    typs,
		})
	})

	output, err := exec.Command("go", "run", tempFilePath).Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("failed to run output program: %w (%v)", err, string(exitError.Stderr))
		}
		return fmt.Errorf("failed to run output program: %w", err)
	}

	var result map[string]*typeInfo
	if err = json.Unmarshal(output, &result); err != nil {
		return fmt.Errorf("failed to decode output types: %w", err)
	}

	// Build the import map.
	imports := map[string]*types.Package{}
	for _, m := range p.modules {
		collectImports(imports, m.goPackage.Types)
	}

	// Resolve the element type info.
	ids := make([]string, 0, len(result))
	for id := range result {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		resolved, err := result[id].resolve(imports)
		if err != nil {
			return fmt.Errorf("failed to resolve type %v: %w", id, err)
		}
		p.outputTypes[id] = resolved

		p.providerModule.markType(resolved)
	}

	return nil
}
