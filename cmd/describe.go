// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/marapongo/mu/pkg/encoding"
	"github.com/marapongo/mu/pkg/pack"
	"github.com/marapongo/mu/pkg/pack/ast"
	decode "github.com/marapongo/mu/pkg/pack/encoding"
	"github.com/marapongo/mu/pkg/pack/symbols"
	"github.com/marapongo/mu/pkg/util/contract"
)

func newDescribeCmd() *cobra.Command {
	var printExports bool
	var printIL bool
	var printSymbols bool
	var cmd = &cobra.Command{
		Use:   "describe [packages]",
		Short: "Describe a MuPackage",
		Long:  "Describe prints package, symbol, and IL information from one or more MuPackages.",
		Run: func(cmd *cobra.Command, args []string) {
			// Enumerate the list of packages, deserialize them, and print information.
			for _, arg := range args {
				var pkg *pack.Package
				if arg == "-" {
					// Read the package from stdin.
					b, err := ioutil.ReadAll(os.Stdin)
					if err != nil {
						fmt.Fprintf(os.Stderr, "error: could not read from stdin\n")
						fmt.Fprintf(os.Stderr, "       %v\n", err)
					}
					pkg = decodePackage(encoding.Marshalers[".json"], b, "stdin")
				} else {
					// Read the package from a file.
					pkg = readPackage(arg)
				}
				if pkg == nil {
					break
				}
				printPackage(pkg, printSymbols, printExports, printIL)
			}
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&printExports, "exports", "e", false,
		"Print just the exported symbols")
	cmd.PersistentFlags().BoolVarP(
		&printIL, "il", "i", false,
		"Pretty-print the MuIL")
	cmd.PersistentFlags().BoolVarP(
		&printSymbols, "symbols", "s", false,
		"Print a complete listing of all symbols, exported or otherwise")

	return cmd
}

func readPackage(path string) *pack.Package {
	// Lookup the marshaler for this format.
	ext := filepath.Ext(path)
	m, has := encoding.Marshalers[ext]
	if !has {
		fmt.Fprintf(os.Stderr, "error: no marshaler found for file format '%v'\n", ext)
		return nil
	}

	// Read the contents.
	b, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: a problem occurred when reading file '%v'\n", path)
		fmt.Fprintf(os.Stderr, "       %v\n", err)
		return nil
	}

	return decodePackage(m, b, path)
}

func decodePackage(m encoding.Marshaler, b []byte, path string) *pack.Package {
	// Unmarshal the contents into a fresh package.
	pkg, err := decode.Decode(m, b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: a problem occurred when unmarshaling file '%v'\n", path)
		fmt.Fprintf(os.Stderr, "       %v\n", err)
		return nil
	}
	return pkg
}

func printPackage(pkg *pack.Package, printSymbols bool, printExports bool, printIL bool) {
	// Pretty-print the package metadata:
	fmt.Printf("package \"%v\" {\n", pkg.Name)
	if pkg.Description != "" {
		fmt.Printf("\tdescription %v\n", pkg.Description)
	}
	if pkg.Author != "" {
		fmt.Printf("\tauthor %v\n", pkg.Author)
	}
	if pkg.Website != "" {
		fmt.Printf("\twebsite %v\n", pkg.Website)
	}
	if pkg.License != "" {
		fmt.Printf("\tlicense %v\n", pkg.License)
	}

	// Print the dependencies:
	fmt.Printf("\tdependencies [")
	if pkg.Dependencies != nil && len(*pkg.Dependencies) > 0 {
		fmt.Printf("\n")
		for _, dep := range *pkg.Dependencies {
			fmt.Printf("\t\t\"%v\"", dep)
		}
		fmt.Printf("\n\t")
	}
	fmt.Printf("]\n")

	// Print the modules (just names by default, or full symbols and/or IL if requested).
	printModules(pkg, printSymbols, printExports, printIL, "\t")

	fmt.Printf("}\n")
}

func printModules(pkg *pack.Package, printSymbols bool, printExports bool, printIL bool, indent string) {
	for _, name := range ast.StableModules(*pkg.Modules) {
		fmt.Printf("%vmodule \"%v\" {", indent, name)
		mod := (*pkg.Modules)[name]
		if (printSymbols || printExports) && mod.Members != nil {
			fmt.Printf("\n")
			for _, member := range ast.StableModuleMembers(*mod.Members) {
				printModuleMember(member, (*mod.Members)[member], printExports, indent+"\t")
			}
			fmt.Printf("%v", indent)
		}
		fmt.Printf("}\n")
	}
}

func printModuleMember(name symbols.Token, member ast.ModuleMember, exportOnly bool, indent string) {
	acc := member.GetAccess()
	if !exportOnly || (acc != nil && *acc == symbols.PublicAccessibility) {
		switch member.GetKind() {
		case ast.ExportKind:
			printExport(name, member.(*ast.Export), indent)
		case ast.ClassKind:
			printClass(name, member.(*ast.Class), exportOnly, indent)
		case ast.ModulePropertyKind:
			printModuleProperty(name, member.(*ast.ModuleProperty), indent)
		case ast.ModuleMethodKind:
			printModuleMethod(name, member.(*ast.ModuleMethod), indent)
		default:
			contract.FailMF("Unexpected ModuleMember kind: %v\n", member.GetKind())
		}
	}
}

func printExport(name symbols.Token, export *ast.Export, indent string) {
	var mods []string
	if export.Access != nil {
		mods = append(mods, string(*export.Access))
	}
	fmt.Printf("%vexport \"%v\"%v %v\n", indent, name, modString(mods), export.Token)
}

func printClass(name symbols.Token, class *ast.Class, exportOnly bool, indent string) {
	fmt.Printf("%vclass \"%v\"", indent, name)

	var mods []string
	if class.Access != nil {
		mods = append(mods, string(*class.Access))
	}
	if class.Sealed != nil && *class.Sealed {
		mods = append(mods, "sealed")
	}
	if class.Abstract != nil && *class.Abstract {
		mods = append(mods, "abstract")
	}
	if class.Record != nil && *class.Record {
		mods = append(mods, "record")
	}
	if class.Interface != nil && *class.Interface {
		mods = append(mods, "interface")
	}
	fmt.Printf(modString(mods))

	if class.Extends != nil {
		fmt.Printf("\n%v\t\textends %v", indent, string(*class.Extends))
	}
	if class.Implements != nil {
		for _, impl := range *class.Implements {
			fmt.Printf("\n%v\t\timplements %v", indent, string(impl))
		}
	}

	fmt.Printf(" {")
	if class.Members != nil {
		fmt.Printf("\n")
		for _, member := range ast.StableClassMembers(*class.Members) {
			printClassMember(member, (*class.Members)[member], exportOnly, indent+"\t")
		}
		fmt.Printf(indent)
	}
	fmt.Printf("}\n")
}

func printClassMember(name symbols.Token, member ast.ClassMember, exportOnly bool, indent string) {
	acc := member.GetAccess()
	if !exportOnly || (acc != nil && *acc == symbols.PublicClassAccessibility) {
		switch member.GetKind() {
		case ast.ClassPropertyKind:
			printClassProperty(name, member.(*ast.ClassProperty), indent)
		case ast.ClassMethodKind:
			printClassMethod(name, member.(*ast.ClassMethod), indent)
		default:
			contract.FailMF("Unexpected ClassMember kind: %v\n", member.GetKind())
		}
	}
}

func printClassProperty(name symbols.Token, prop *ast.ClassProperty, indent string) {
	var mods []string
	if prop.Access != nil {
		mods = append(mods, string(*prop.Access))
	}
	if prop.Static != nil && *prop.Static {
		mods = append(mods, "static")
	}
	if prop.Readonly != nil && *prop.Readonly {
		mods = append(mods, "readonly")
	}
	fmt.Printf("%vproperty \"%v\"%v", indent, name, modString(mods))
	if prop.Type != nil {
		fmt.Printf(": %v", *prop.Type)
	}
	fmt.Printf("\n")
}

func printClassMethod(name symbols.Token, meth *ast.ClassMethod, indent string) {
	var mods []string
	if meth.Access != nil {
		mods = append(mods, string(*meth.Access))
	}
	if meth.Static != nil && *meth.Static {
		mods = append(mods, "static")
	}
	if meth.Sealed != nil && *meth.Sealed {
		mods = append(mods, "sealed")
	}
	if meth.Abstract != nil && *meth.Abstract {
		mods = append(mods, "abstract")
	}
	fmt.Printf("%vmethod \"%v\"%v: %v\n", indent, name, modString(mods), funcSig(meth))
}

func printModuleMethod(name symbols.Token, meth *ast.ModuleMethod, indent string) {
	var mods []string
	if meth.Access != nil {
		mods = append(mods, string(*meth.Access))
	}
	fmt.Printf("%vmethod \"%v\"%v: %v\n", indent, name, modString(mods), funcSig(meth))
}

func printModuleProperty(name symbols.Token, prop *ast.ModuleProperty, indent string) {
	var mods []string
	if prop.Access != nil {
		mods = append(mods, string(*prop.Access))
	}
	if prop.Readonly != nil && *prop.Readonly {
		mods = append(mods, "readonly")
	}
	fmt.Printf("%vproperty \"%v\"%v", indent, name, modString(mods))
	if prop.Type != nil {
		fmt.Printf(": %v", *prop.Type)
	}
	fmt.Printf("\n")
}

func modString(mods []string) string {
	if len(mods) == 0 {
		return ""
	}
	s := " ["
	for i, mod := range mods {
		if i > 0 {
			s += ", "
		}
		s += mod
	}
	s += "]"
	return s
}

func funcSig(fun ast.Function) string {
	sig := "("

	// To create a signature, first concatenate the parameters.
	params := fun.GetParameters()
	if params != nil {
		for i, param := range *params {
			if i > 0 {
				sig += ", "
			}
			sig += string(param.Name.Ident)
			if param.Type != nil {
				sig += ": " + string(*param.Type)
			}
		}
	}
	sig += ")"

	// And then the return type, if present.
	ret := fun.GetReturnType()
	if ret != nil {
		sig += ": " + string(*ret)
	}

	return sig
}
