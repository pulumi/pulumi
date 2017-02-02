// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/pack"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/cmdutil"
	"github.com/marapongo/mu/pkg/util/contract"
)

func newDescribeCmd() *cobra.Command {
	var printAll bool
	var printExports bool
	var printIL bool
	var printSymbols bool
	var cmd = &cobra.Command{
		Use:   "describe [packages...]",
		Short: "Describe a MuPackage",
		Long:  "Describe prints package, symbol, and IL information from one or more MuPackages.",
		Run: func(cmd *cobra.Command, args []string) {
			// If printAll is true, flip all the flags.
			if printAll {
				printExports = true
				printIL = true
				printSymbols = true
			}

			// Enumerate the list of packages, deserialize them, and print information.
			for _, arg := range args {
				pkg := cmdutil.ReadPackageFromArg(arg)
				if pkg == nil {
					break
				}
				printPackage(pkg, printSymbols, printExports, printIL)
			}
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&printSymbols, "all", "a", false,
		"Print everything: the package, symbols, and MuIL")
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

func printComment(pc *string, indent string) {
	// Prints a comment header using the given indentation, wrapping at 100 lines.
	if pc != nil {
		prefix := "// "
		maxlen := 100 - len(indent)

		// For every tab, chew up 3 more chars (so each one is 4 chars wide).
		for _, i := range indent {
			if i == '\t' {
				maxlen -= 3
			}
		}
		maxlen -= len(prefix)
		if maxlen < 40 {
			maxlen = 40
		}

		c := make([]rune, 0)
		for _, r := range *pc {
			c = append(c, r)
		}

		for len(c) > 0 {
			fmt.Print(indent + prefix)
			// Now, try to split the comment as close to maxlen-3 chars as possible (taking into account indent+"// "),
			// but don't split words -- only split at whitespace characters if we can help it.
			six := maxlen
			for {
				if len(c) <= six {
					six = len(c)
					break
				} else if unicode.IsSpace(c[six]) {
					// It's a space, set six to the first non-space character beforehand, and eix to the first
					// non-space character afterwards.
					for six > 0 && unicode.IsSpace(c[six-1]) {
						six--
					}
					break
				} else if six == 0 {
					// We hit the start of the string and didn't find any spaces.  Start over and try to find the
					// first space *beyond* the start point (instead of *before*) and use that.
					six = maxlen + 1
					for six < len(c) && !unicode.IsSpace(c[six]) {
						six++
					}
					break
				}

				// We need to keep searching, back up one and try again.
				six--
			}

			// Print what we've got thus far, plus a newline.
			fmt.Printf("%v\n", string(c[:six]))

			// Now find the first non-space character beyond the split point and use that for the remainder.
			eix := six
			for eix < len(c) && unicode.IsSpace(c[eix]) {
				eix++
			}
			c = c[eix:]
		}
	}
}

// printPackage pretty-prints the package metadata.
func printPackage(pkg *pack.Package, printSymbols bool, printExports bool, printIL bool) {
	printComment(pkg.Description, "")
	fmt.Printf("package \"%v\" {\n", pkg.Name)

	if pkg.Author != nil {
		fmt.Printf("%vauthor \"%v\"\n", tab, *pkg.Author)
	}
	if pkg.Website != nil {
		fmt.Printf("%vwebsite \"%v\"\n", tab, *pkg.Website)
	}
	if pkg.License != nil {
		fmt.Printf("%vlicense \"%v\"\n", tab, *pkg.License)
	}

	// Print the dependencies:
	fmt.Printf("%vdependencies [", tab)
	if pkg.Dependencies != nil && len(*pkg.Dependencies) > 0 {
		fmt.Printf("\n")
		for _, dep := range *pkg.Dependencies {
			fmt.Printf("%v\"%v\"\n", tab+tab, dep)
		}
		fmt.Printf("%v", tab)
	}
	fmt.Printf("]\n")

	// Print the modules (just names by default, or full symbols and/or IL if requested).
	printModules(pkg, printSymbols, printExports, printIL, tab)

	fmt.Printf("}\n")
}

func printModules(pkg *pack.Package, printSymbols bool, printExports bool, printIL bool, indent string) {
	for _, name := range ast.StableModules(*pkg.Modules) {
		mod := (*pkg.Modules)[name]

		// Print the name.
		fmt.Printf("%vmodule \"%v\" {", indent, name)

		// Now, if requested, print the tokens.
		if printSymbols || printExports {
			if mod.Imports != nil || mod.Members != nil {
				fmt.Printf("\n")

				if mod.Imports != nil {
					// Print the imports.
					fmt.Printf("%vimports [", indent+tab)
					if mod.Imports != nil && len(*mod.Imports) > 0 {
						fmt.Printf("\n")
						for _, imp := range *mod.Imports {
							fmt.Printf("%v\"%v\"\n", indent+tab+tab, imp.Tok)
						}
						fmt.Printf("%v", indent+tab)
					}
					fmt.Printf("]\n")
				}

				if mod.Members != nil {
					// Print the members.
					for _, member := range ast.StableModuleMembers(*mod.Members) {
						printModuleMember(member, (*mod.Members)[member], printExports, indent+tab)
					}
					fmt.Printf("%v", indent)
				}
			}
		} else {
			// Print a "..." so that it's clear we're omitting information, versus the module being empty.
			fmt.Printf("...")
		}
		fmt.Printf("}\n")
	}
}

func printModuleMember(name tokens.ModuleMemberName, member ast.ModuleMember, exportOnly bool, indent string) {
	printComment(member.GetDescription(), indent)

	acc := member.GetAccess()
	if !exportOnly || (acc != nil && *acc == tokens.PublicAccessibility) {
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
			contract.Failf("Unexpected ModuleMember kind: %v\n", member.GetKind())
		}
	}
}

func printExport(name tokens.ModuleMemberName, export *ast.Export, indent string) {
	var mods []string
	if export.Access != nil {
		mods = append(mods, string(*export.Access))
	}
	fmt.Printf("%vexport \"%v\"%v %v\n", indent, name, modString(mods), export.Referent.Tok)
}

func printClass(name tokens.ModuleMemberName, class *ast.Class, exportOnly bool, indent string) {
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
		fmt.Printf("\n%vextends %v", indent+tab+tab, string(class.Extends.Tok))
	}
	if class.Implements != nil {
		for _, impl := range *class.Implements {
			fmt.Printf("\n%vimplements %v", indent+tab+tab, string(impl.Tok))
		}
	}

	fmt.Printf(" {")
	if class.Members != nil {
		fmt.Printf("\n")
		for _, member := range ast.StableClassMembers(*class.Members) {
			printClassMember(member, (*class.Members)[member], exportOnly, indent+tab)
		}
		fmt.Printf(indent)
	}
	fmt.Printf("}\n")
}

func printClassMember(name tokens.ClassMemberName, member ast.ClassMember, exportOnly bool, indent string) {
	printComment(member.GetDescription(), indent)

	acc := member.GetAccess()
	if !exportOnly || (acc != nil && *acc == tokens.PublicClassAccessibility) {
		switch member.GetKind() {
		case ast.ClassPropertyKind:
			printClassProperty(name, member.(*ast.ClassProperty), indent)
		case ast.ClassMethodKind:
			printClassMethod(name, member.(*ast.ClassMethod), indent)
		default:
			contract.Failf("Unexpected ClassMember kind: %v\n", member.GetKind())
		}
	}
}

func printClassProperty(name tokens.ClassMemberName, prop *ast.ClassProperty, indent string) {
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
		fmt.Printf(": %v", prop.Type.Tok)
	}
	fmt.Printf("\n")
}

func printClassMethod(name tokens.ClassMemberName, meth *ast.ClassMethod, indent string) {
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

func printModuleMethod(name tokens.ModuleMemberName, meth *ast.ModuleMethod, indent string) {
	var mods []string
	if meth.Access != nil {
		mods = append(mods, string(*meth.Access))
	}
	fmt.Printf("%vmethod \"%v\"%v: %v\n", indent, name, modString(mods), funcSig(meth))
}

func printModuleProperty(name tokens.ModuleMemberName, prop *ast.ModuleProperty, indent string) {
	var mods []string
	if prop.Access != nil {
		mods = append(mods, string(*prop.Access))
	}
	if prop.Readonly != nil && *prop.Readonly {
		mods = append(mods, "readonly")
	}
	fmt.Printf("%vproperty \"%v\"%v", indent, name, modString(mods))
	if prop.Type != nil {
		fmt.Printf(": %v", prop.Type.Tok)
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

// spaces returns a string with the given number of spaces.
func spaces(num int) string {
	return strings.Repeat(" ", num)
}

// tab is a tab represented as spaces, since some consoles have ridiculously wide true tabs.
var tab = spaces(4)

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
				sig += ": " + string(param.Type.Tok)
			}
		}
	}
	sig += ")"

	// And then the return type, if present.
	ret := fun.GetReturnType()
	if ret != nil {
		sig += ": " + string(ret.Tok)
	}

	return sig
}
