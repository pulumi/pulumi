// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/lumi/pkg/compiler/ast"
	"github.com/pulumi/lumi/pkg/pack"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/cmdutil"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/workspace"
)

func newPackInfoCmd() *cobra.Command {
	var printAll bool
	var printIL bool
	var printSymbols bool
	var printExportedSymbols bool
	var cmd = &cobra.Command{
		Use:   "info [packages...]",
		Short: "Print information about one or more packages",
		Long: "Print information about one or more packages\n" +
			"\n" +
			"This command prints metadata, symbol, and/or IL from one or more packages.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// If printAll is true, flip all the flags.
			if printAll {
				printIL = true
				printSymbols = true
				printExportedSymbols = true
			}

			if len(args) == 0 {
				// No package specified, just load from the current directory.
				pwd, _ := os.Getwd()
				pkgpath, err := workspace.DetectPackage(pwd, cmdutil.Sink())
				if err != nil {
					return errors.Errorf("could not locate a package to load: %v", err)
				}

				if pkg := cmdutil.ReadPackage(pkgpath); pkg != nil {
					printPackage(pkg, printSymbols, printExportedSymbols, printIL)
				}
			} else {
				// Enumerate the list of packages, deserialize them, and print information.
				for _, arg := range args {
					pkg := cmdutil.ReadPackageFromArg(arg)
					if pkg == nil {
						break
					}
					printPackage(pkg, printSymbols, printExportedSymbols, printIL)
				}
			}

			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&printSymbols, "all", "a", false,
		"Print everything: the package, symbols, and IL")
	cmd.PersistentFlags().BoolVarP(
		&printExportedSymbols, "exports", "e", false,
		"Print just the exported symbols")
	cmd.PersistentFlags().BoolVarP(
		&printIL, "il", "i", false,
		"Pretty-print the package's IL")
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
		for _, dep := range pack.StableDependencies(*pkg.Dependencies) {
			fmt.Printf("%v%v: \"%v\"\n", tab+tab, dep, (*pkg.Dependencies)[dep])
		}
		fmt.Printf("%v", tab)
	}
	fmt.Printf("]\n")

	// Print the modules (just names by default, or full symbols and/or IL if requested).
	printModules(pkg, printSymbols, printExports, printIL, tab)

	fmt.Printf("}\n")
}

func printModules(pkg *pack.Package, printSymbols bool, printExports bool, printIL bool, indent string) {
	if pkg.Modules != nil {
		pkgtok := tokens.NewPackageToken(pkg.Name)
		for _, name := range ast.StableModules(*pkg.Modules) {
			mod := (*pkg.Modules)[name]
			modtok := tokens.NewModuleToken(pkgtok, name)

			// Print the name.
			fmt.Printf("%vmodule \"%v\" {", indent, name)

			// Now, if requested, print the tokens.
			if printSymbols || printExports {
				if mod.Exports != nil || mod.Members != nil {
					fmt.Printf("\n")

					exports := make(map[tokens.Token]bool)
					if mod.Exports != nil {
						// Print the exports.
						fmt.Printf("%vexports [", indent+tab)
						if mod.Exports != nil && len(*mod.Exports) > 0 {
							fmt.Printf("\n")
							for _, exp := range ast.StableModuleExports(*mod.Exports) {
								ref := (*mod.Exports)[exp].Referent.Tok
								fmt.Printf("%v\"%v\" -> \"%v\"\n", indent+tab+tab, exp, ref)
								exports[ref] = true
							}
							fmt.Printf("%v", indent+tab)
						}
						fmt.Printf("]\n")
					}

					if mod.Members != nil {
						// Print the members.
						for _, member := range ast.StableModuleMembers(*mod.Members) {
							memtok := tokens.NewModuleMemberToken(modtok, member)
							printModuleMember(memtok, (*mod.Members)[member], printExports, exports, indent+tab)
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
}

func printModuleMember(tok tokens.ModuleMember, member ast.ModuleMember,
	exportOnly bool, exports map[tokens.Token]bool, indent string) {
	printComment(member.GetDescription(), indent)

	if !exportOnly || exports[tokens.Token(tok)] {
		switch member.GetKind() {
		case ast.ClassKind:
			printClass(tokens.Type(tok), member.(*ast.Class), exportOnly, indent)
		case ast.ModulePropertyKind:
			printModuleProperty(tok, member.(*ast.ModuleProperty), indent)
		case ast.ModuleMethodKind:
			printModuleMethod(tok, member.(*ast.ModuleMethod), indent)
		default:
			contract.Failf("Unexpected ModuleMember kind: %v (tok %v)\n", member.GetKind(), tok)
		}
	}
}

func printClass(tok tokens.Type, class *ast.Class, exportOnly bool, indent string) {
	fmt.Printf("%vclass \"%v\"", indent, tok.Name())

	var mods []string
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
			memtok := tokens.NewClassMemberToken(tok, member)
			printClassMember(memtok, (*class.Members)[member], exportOnly, indent+tab)
		}
		fmt.Printf(indent)
	}
	fmt.Printf("}\n")
}

func printClassMember(tok tokens.ClassMember, member ast.ClassMember, exportOnly bool, indent string) {
	printComment(member.GetDescription(), indent)

	acc := member.GetAccess()
	if !exportOnly || (acc != nil && *acc == tokens.PublicAccessibility) {
		switch member.GetKind() {
		case ast.ClassPropertyKind:
			printClassProperty(tok, member.(*ast.ClassProperty), indent)
		case ast.ClassMethodKind:
			printClassMethod(tok, member.(*ast.ClassMethod), indent)
		default:
			contract.Failf("Unexpected ClassMember kind: %v\n", member.GetKind())
		}
	}
}

func printClassProperty(tok tokens.ClassMember, prop *ast.ClassProperty, indent string) {
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
	fmt.Printf("%vproperty \"%v\"%v", indent, tok.Name(), modString(mods))
	if prop.Type != nil {
		fmt.Printf(": %v", prop.Type.Tok)
	}
	fmt.Printf("\n")
}

func printClassMethod(tok tokens.ClassMember, meth *ast.ClassMethod, indent string) {
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
	fmt.Printf("%vmethod \"%v\"%v: %v\n", indent, tok.Name(), modString(mods), funcSig(meth))
}

func printModuleMethod(tok tokens.ModuleMember, meth *ast.ModuleMethod, indent string) {
	fmt.Printf("%vmethod \"%v\": %v\n", indent, tok.Name(), funcSig(meth))
}

func printModuleProperty(tok tokens.ModuleMember, prop *ast.ModuleProperty, indent string) {
	var mods []string
	if prop.Readonly != nil && *prop.Readonly {
		mods = append(mods, "readonly")
	}
	fmt.Printf("%vproperty \"%v\"%v", indent, tok.Name(), modString(mods))
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
