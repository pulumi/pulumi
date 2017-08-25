package engine

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/pulumi/pulumi-fabric/pkg/compiler/ast"
	"github.com/pulumi/pulumi-fabric/pkg/pack"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

func (eng *Engine) PackInfo(printExportedSymbols bool, printIL bool, printSymbols bool, packages []string) error {
	var pkg *pack.Package
	var err error
	if len(packages) == 0 {
		// No package specified, just load from the current directory.
		pwd, locerr := os.Getwd()
		if locerr != nil {
			return locerr
		}
		if pkg, err = eng.detectPackage(pwd); err != nil {
			return err
		}
		eng.printPackage(pkg, printSymbols, printExportedSymbols, printIL)
	} else {
		// Enumerate the list of packages, deserialize them, and print information.
		var path string
		for _, arg := range packages {
			pkg, path = eng.readPackageFromArg(arg)
			if pkg == nil {
				if pkg, err = eng.detectPackage(path); err != nil {
					return err
				}
				eng.printPackage(pkg, printSymbols, printExportedSymbols, printIL)
			}
		}
	}

	return nil
}

func (eng *Engine) printComment(pc *string, indent string) {
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
			fmt.Fprint(eng.Stdout, indent+prefix)
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
			fmt.Fprintf(eng.Stdout, "%v\n", string(c[:six]))

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
func (eng *Engine) printPackage(pkg *pack.Package, printSymbols bool, printExports bool, printIL bool) {
	eng.printComment(pkg.Description, "")
	fmt.Fprintf(eng.Stdout, "package \"%v\" {\n", pkg.Name)

	if pkg.Author != nil {
		fmt.Fprintf(eng.Stdout, "%vauthor \"%v\"\n", tab, *pkg.Author)
	}
	if pkg.Website != nil {
		fmt.Fprintf(eng.Stdout, "%vwebsite \"%v\"\n", tab, *pkg.Website)
	}
	if pkg.License != nil {
		fmt.Fprintf(eng.Stdout, "%vlicense \"%v\"\n", tab, *pkg.License)
	}

	// Print the dependencies:
	fmt.Fprintf(eng.Stdout, "%vdependencies [", tab)
	if pkg.Dependencies != nil && len(*pkg.Dependencies) > 0 {
		fmt.Fprintf(eng.Stdout, "\n")
		for _, dep := range pack.StableDependencies(*pkg.Dependencies) {
			fmt.Fprintf(eng.Stdout, "%v%v: \"%v\"\n", tab+tab, dep, (*pkg.Dependencies)[dep])
		}
		fmt.Fprintf(eng.Stdout, "%v", tab)
	}
	fmt.Fprintf(eng.Stdout, "]\n")

	// Print the modules (just names by default, or full symbols and/or IL if requested).
	eng.printModules(pkg, printSymbols, printExports, printIL, tab)

	fmt.Fprintf(eng.Stdout, "}\n")
}

func (eng *Engine) printModules(pkg *pack.Package, printSymbols bool, printExports bool, printIL bool, indent string) {
	if pkg.Modules != nil {
		pkgtok := tokens.NewPackageToken(pkg.Name)
		for _, name := range ast.StableModules(*pkg.Modules) {
			mod := (*pkg.Modules)[name]
			modtok := tokens.NewModuleToken(pkgtok, name)

			// Print the name.
			fmt.Fprintf(eng.Stdout, "%vmodule \"%v\" {", indent, name)

			// Now, if requested, print the tokens.
			if printSymbols || printExports {
				if mod.Exports != nil || mod.Members != nil {
					fmt.Fprintf(eng.Stdout, "\n")

					exports := make(map[tokens.Token]bool)
					if mod.Exports != nil {
						// Print the exports.
						fmt.Fprintf(eng.Stdout, "%vexports [", indent+tab)
						if mod.Exports != nil && len(*mod.Exports) > 0 {
							fmt.Fprintf(eng.Stdout, "\n")
							for _, exp := range ast.StableModuleExports(*mod.Exports) {
								ref := (*mod.Exports)[exp].Referent.Tok
								fmt.Fprintf(eng.Stdout, "%v\"%v\" -> \"%v\"\n", indent+tab+tab, exp, ref)
								exports[ref] = true
							}
							fmt.Fprintf(eng.Stdout, "%v", indent+tab)
						}
						fmt.Fprintf(eng.Stdout, "]\n")
					}

					if mod.Members != nil {
						// Print the members.
						for _, member := range ast.StableModuleMembers(*mod.Members) {
							memtok := tokens.NewModuleMemberToken(modtok, member)
							eng.printModuleMember(memtok, (*mod.Members)[member], printExports, exports, indent+tab)
						}
						fmt.Fprintf(eng.Stdout, "%v", indent)
					}
				}
			} else {
				// Print a "..." so that it's clear we're omitting information, versus the module being empty.
				fmt.Fprintf(eng.Stdout, "...")
			}
			fmt.Fprintf(eng.Stdout, "}\n")
		}
	}
}

func (eng *Engine) printModuleMember(tok tokens.ModuleMember, member ast.ModuleMember,
	exportOnly bool, exports map[tokens.Token]bool, indent string) {
	eng.printComment(member.GetDescription(), indent)

	if !exportOnly || exports[tokens.Token(tok)] {
		switch member.GetKind() {
		case ast.ClassKind:
			eng.printClass(tokens.Type(tok), member.(*ast.Class), exportOnly, indent)
		case ast.ModulePropertyKind:
			eng.printModuleProperty(tok, member.(*ast.ModuleProperty), indent)
		case ast.ModuleMethodKind:
			eng.printModuleMethod(tok, member.(*ast.ModuleMethod), indent)
		default:
			contract.Failf("Unexpected ModuleMember kind: %v (tok %v)\n", member.GetKind(), tok)
		}
	}
}

func (eng *Engine) printClass(tok tokens.Type, class *ast.Class, exportOnly bool, indent string) {
	fmt.Fprintf(eng.Stdout, "%vclass \"%v\"", indent, tok.Name())

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
	if class.Attributes != nil {
		for _, att := range *class.Attributes {
			mods = append(mods, "@"+att.Decorator.Tok.String())
		}
	}
	fmt.Fprint(eng.Stdout, modString(mods))

	if class.Extends != nil {
		fmt.Fprintf(eng.Stdout, "\n%vextends %v", indent+tab+tab, string(class.Extends.Tok))
	}
	if class.Implements != nil {
		for _, impl := range *class.Implements {
			fmt.Fprintf(eng.Stdout, "\n%vimplements %v", indent+tab+tab, string(impl.Tok))
		}
	}

	fmt.Fprintf(eng.Stdout, " {")
	if class.Members != nil {
		fmt.Fprintf(eng.Stdout, "\n")
		for _, member := range ast.StableClassMembers(*class.Members) {
			memtok := tokens.NewClassMemberToken(tok, member)
			eng.printClassMember(memtok, (*class.Members)[member], exportOnly, indent+tab)
		}
		fmt.Fprint(eng.Stdout, indent)
	}
	fmt.Fprintf(eng.Stdout, "}\n")
}

func (eng *Engine) printClassMember(tok tokens.ClassMember, member ast.ClassMember, exportOnly bool, indent string) {
	eng.printComment(member.GetDescription(), indent)

	acc := member.GetAccess()
	if !exportOnly || (acc != nil && *acc == tokens.PublicAccessibility) {
		switch member.GetKind() {
		case ast.ClassPropertyKind:
			eng.printClassProperty(tok.Name(), member.(*ast.ClassProperty), indent)
		case ast.ClassMethodKind:
			eng.printClassMethod(tok.Name(), member.(*ast.ClassMethod), indent)
		default:
			contract.Failf("Unexpected ClassMember kind: %v\n", member.GetKind())
		}
	}
}

func (eng *Engine) printClassProperty(name tokens.ClassMemberName, prop *ast.ClassProperty, indent string) {
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
	if prop.Attributes != nil {
		for _, att := range *prop.Attributes {
			mods = append(mods, "@"+att.Decorator.Tok.String())
		}
	}
	fmt.Fprintf(eng.Stdout, "%vproperty \"%v\"%v", indent, name, modString(mods))
	if prop.Type != nil {
		fmt.Fprintf(eng.Stdout, ": %v", prop.Type.Tok)
	}

	if prop.Getter != nil || prop.Setter != nil {
		fmt.Fprintf(eng.Stdout, " {\n")
		if prop.Getter != nil {
			eng.printClassMethod(tokens.ClassMemberName("get"), prop.Getter, indent+"    ")
		}
		if prop.Setter != nil {
			eng.printClassMethod(tokens.ClassMemberName("set"), prop.Setter, indent+"    ")
		}
		fmt.Fprintf(eng.Stdout, "%v}\n", indent)
	} else {
		fmt.Fprintf(eng.Stdout, "\n")
	}
}

func (eng *Engine) printClassMethod(name tokens.ClassMemberName, meth *ast.ClassMethod, indent string) {
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
	if meth.Attributes != nil {
		for _, att := range *meth.Attributes {
			mods = append(mods, "@"+att.Decorator.Tok.String())
		}
	}
	fmt.Fprintf(eng.Stdout, "%vmethod \"%v\"%v: %v\n", indent, name, modString(mods), funcSig(meth))
}

func (eng *Engine) printModuleMethod(tok tokens.ModuleMember, meth *ast.ModuleMethod, indent string) {
	fmt.Fprintf(eng.Stdout, "%vmethod \"%v\": %v\n", indent, tok.Name(), funcSig(meth))
}

func (eng *Engine) printModuleProperty(tok tokens.ModuleMember, prop *ast.ModuleProperty, indent string) {
	var mods []string
	if prop.Readonly != nil && *prop.Readonly {
		mods = append(mods, "readonly")
	}
	fmt.Fprintf(eng.Stdout, "%vproperty \"%v\"%v", indent, tok.Name(), modString(mods))
	if prop.Type != nil {
		fmt.Fprintf(eng.Stdout, ": %v", prop.Type.Tok)
	}
	fmt.Fprintf(eng.Stdout, "\n")
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

			var mods []string
			if param.Attributes != nil {
				for _, att := range *param.Attributes {
					mods = append(mods, "@"+att.Decorator.Tok.String())
				}
			}
			sig += modString(mods)

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
