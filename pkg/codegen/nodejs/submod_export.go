package nodejs

import (
	"fmt"
	"io"
)

// Desired Behavior:
// Given a submodule "foo"…
// …these are the given syntax forms:
// 
// import * as fooModule from "./foo";
// export const foo: typeof fooModule = {} as typeof fooModule;
// 
// These are the methods we need to produce those forms:
// foo
// "./foo"
// fooModule
// typeof fooModule
// 
// The functions to test:
// name => foo
// moduleFileName() => "./foo"
// moduleTypeName => fooModule
// qualifiedTypeName => typeof fooModule
// 
// genModuleImport => import * as fooModule from "./foo";
// genConstLocal => export const foo: typeof fooModule = {} as typeof fooModule;
// TODO: Does this last statement impact Intellisense?

// submoduleExport represents a package exposed from inside another package.
// In NodeJS, this takes the form of a "submodule export", wherein the child
// package is exported from the parent package. The types `submoduleExport`
// and `submoduleExportList` serve as helpful wrappers to sanely generate
// and lazy-load these exports.
type submoduleExport string

// newSubmoduleExport is the constructor for a submoduleExport.
// The name provided is assumed to be legal.
func newSubmoduleExport(name string) submoduleExport {
	return submoduleExport(name)
}

// fileName generates the NodeJS import path
// to the submodule. All submodules are within the same
// directory as the calling code.
func (exp submoduleExport) fileName() string {
	return `"./` + string(exp) + `"`
}

// typeName provides the raw name of the module type.
// Since we're lazy-loading, we declare the type name
// as the module's actual name plus a suffix, so we can
// use module name elsewhere as a local variable declaration.
func (exp submoduleExport) typeName() string {
	return string(exp) + "Module"
}

// qualifiedTypeName returns the the module's type name qualified
// as it will be used as a type identifier.
func (exp submoduleExport) qualifiedTypeName() string {
	return "typeof " + exp.typeName()
}

// name returns the original module's name.
func (exp submoduleExport) name() string {
	return string(exp)
}

// genImport produces the import statement used to declare
// the module type name. It will ultimately be elided from the
// generated JavaScript code by TSC.
// e.g. for a module "foo":
//
// import * as fooModule from "./foo";
func (exp submoduleExport) genImport() string {
	return fmt.Sprintf(
		"import * as %s from %s;", 
		exp.typeName(), 
		exp.fileName(),
	)
}

// getExportLocal exports a local variable declaration ascribed
// with the qualified type name.
// e.g. for a module "foo":
//
// export const foo: typeof fooModule = {} as typeof fooModule;
func (exp submoduleExport) genExportLocal() string {
	return fmt.Sprintf(
		"export const %s: %s = {} as %s;", 
		exp.name(),
		exp.qualifiedTypeName(),
		exp.qualifiedTypeName(),
	)
}

// WriteSrc will generate source code for importing and declaring
// this submodule, and will write it to the buffer.
func (exp submoduleExport) WriteSrc(w io.Writer) {
	fmt.Fprintf(w, exp.genImport())
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, exp.genExportLocal())
	fmt.Fprintf(w, "\n")
}

type submoduleExportList []submoduleExport

func newSubmoduleExportList(vals ...string) submoduleExportList {
	var result = make(submoduleExportList, 0, len(vals))
	for _, val := range vals {
		result = append(result, newSubmoduleExport(val))
	}
	return result
}

/*
func (exp submoduleExportList) objectFields() string {
	var asPairs = make([]string, 0, len(exp))
	for _, field := range exp {
		asPairs = append(asPairs, field.nameTypePair())
	}
	return strings.Join(asPairs, ",\n  ")
}
*/

/*
func (exp submoduleExportList) exportConstDecl() string {
	return fmt.Sprintf("const exportNames = [%s];\n", exp.joinWithQuotes())
}

func (exp submoduleExportList) joinWithQuotes() string {
	var asStrings = make([]string, 0, len(exp))
	for _, item := range exp {
		asStrings = append(asStrings, item.wrapInQuotes())
	}
	return strings.Join(asStrings, ", ")
}

func (exp submoduleExportList) asTypeDecl() string {
	var template = "{\n  %v\n  }"
	return fmt.Sprintf(template, exp.objectFields)
}

// e.g.
//  const exports: {
//    foo: fooModuleType,
//    bar: barModuleType
//  } = {};
func (exp submoduleExportList) generateExportDecl() string {
	return fmt.Sprintf("const exports: %v = {}", exp.asTypeDecl())
}
*/
