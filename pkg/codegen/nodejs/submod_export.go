package nodejs

import (
	"fmt"
	"io"
	"strings"
)

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

// quoted returns the name of the module surrounded in quotes.
func (exp submoduleExport) quoted() string {
	return `"` + string(exp) + `"`
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
		"const %s: %s = {} as %s;",
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

// submoduleExportList is a collection of submodule exports.
type submoduleExportList []submoduleExport

// newSubmoduleExportList is the constructor for a submoduleExportList.
func newSubmoduleExportList(vals ...string) submoduleExportList {
	var result = make(submoduleExportList, 0, len(vals))
	for _, val := range vals {
		result = append(result, newSubmoduleExport(val))
	}
	return result
}

// joinMods builds a JS array from the contained submodule names.
func (exp submoduleExportList) joinMods() string {
	var quoted []string
	for _, elem := range exp {
		quoted = append(quoted, elem.quoted())
	}
	var arrayItems = strings.Join(quoted, ", ")
	return "[" + arrayItems + "]"
}

// genLazyLoad generates the instructions for lazy-loading
// the modules in this list. target is the name of the variable
// which will will exported containing the submodules.
// It does not write a trailing newline.
func (exp submoduleExportList) genLazyLoad(target string) string {
	var srcTemplate = `
utilities.lazy_load_all(
	%s,
	%s,
);`
	return fmt.Sprintf(srcTemplate, target, exp.joinMods())
}

// WrtieSrc writes the generated source code to the buffer.
// Here, target is the name of the exported variable which
// will contain the lazy-loaded modules.
func (exp submoduleExportList) WriteSrc(w io.Writer, target string) {
	for _, submod := range exp {
		submod.WriteSrc(w)
	}
	fmt.Fprintf(w, "%s\n", exp.genLazyLoad(target))
}
