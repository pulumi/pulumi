package provider

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"golang.org/x/tools/go/packages"
)

func getPulumiAnnotation(comments *ast.CommentGroup) (string, ast.Node) {
	if comments != nil {
		for _, comment := range comments.List {
			if strings.HasPrefix(comment.Text, "//pulumi:") {
				return strings.TrimSpace(comment.Text[len("//pulumi:"):]), comment
			}
		}
	}
	return "", nil
}

func (m *pulumiModule) getToken(name *ast.Ident, caser func(string) string) string {
	return fmt.Sprintf("%s:%s:%s", m.pulumiPackage.name, m.name, caser(name.Name))
}

func (m *pulumiModule) gatherTypeSpec(spec *ast.TypeSpec, doc *ast.CommentGroup) hcl.Diagnostics {
	annotation, comment := getPulumiAnnotation(doc)
	switch annotation {
	case "":
		// nothing to do
		return nil
	case "provider":
		if m.pulumiPackage.provider != nil {
			existing := m.pulumiPackage.provider.typ
			return hcl.Diagnostics{m.errorf(comment, "package already defines a provider '%v'", existing)}
		}

		m.pulumiPackage.providerModule = m
		m.pulumiPackage.provider = &pulumiResource{
			doc:    doc,
			syntax: spec,
		}
		return nil
	case "resource":
		m.resources = append(m.resources, &pulumiResource{
			token:  m.getToken(spec.Name, pascalCase),
			doc:    doc,
			syntax: spec,
		})
		return nil
	default:
		return hcl.Diagnostics{m.errorf(comment, "unrecognized pulumi type annotation '%v'", annotation)}
	}
}

func (m *pulumiModule) gatherTypeDecl(decl *ast.GenDecl) hcl.Diagnostics {
	var diags hcl.Diagnostics
	for _, spec := range decl.Specs {
		typ := spec.(*ast.TypeSpec)
		doc := typ.Doc
		if len(decl.Specs) == 1 && doc == nil {
			doc = decl.Doc
		}
		typeDiags := m.gatherTypeSpec(typ, doc)
		diags = append(diags, typeDiags...)
	}
	return diags
}

func (m *pulumiModule) gatherFuncDecl(decl *ast.FuncDecl) hcl.Diagnostics {
	annotation, comment := getPulumiAnnotation(decl.Doc)
	switch annotation {
	case "":
		// nothing to do
		return nil
	case "constructor":
		m.constructors = append(m.constructors, &pulumiFunction{
			syntax: decl,
		})
		return nil
	case "function":
		m.functions = append(m.functions, &pulumiFunction{
			token:  m.getToken(decl.Name, camelCase),
			syntax: decl,
		})
		return nil
	default:
		return hcl.Diagnostics{m.errorf(comment, "unrecognized pulumi function annotation '%v'", annotation)}
	}
}

func (m *pulumiModule) gatherFile(file *ast.File) hcl.Diagnostics {
	var diags hcl.Diagnostics
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			funcDiags := m.gatherFuncDecl(decl)
			diags = append(diags, funcDiags...)
		case *ast.GenDecl:
			if decl.Tok == token.TYPE {
				typeDiags := m.gatherTypeDecl(decl)
				diags = append(diags, typeDiags...)
			}
		}
	}
	return diags
}

func (p *pulumiPackage) gatherModule(goPackage *packages.Package) hcl.Diagnostics {
	if !strings.HasPrefix(goPackage.PkgPath, p.rootPackagePath) {
		return hcl.Diagnostics{newError(nil, nil, fmt.Sprintf("package %v is not a child of %v", goPackage.PkgPath, p.rootPackagePath))}
	}

	moduleName := goPackage.PkgPath[len(p.rootPackagePath):]
	switch moduleName {
	case "":
		moduleName = "index"
	case "index":
		// TODO(pdg): this should really be deduped against the other module names.
		moduleName = "index_"
	}

	m := &pulumiModule{
		name:           moduleName,
		pulumiPackage:  p,
		goPackage:      goPackage,
		paramsTypes:    typeSet{},
		componentTypes: typeSet{},
	}
	p.modules = append(p.modules, m)

	var diags hcl.Diagnostics
	for _, f := range goPackage.Syntax {
		fileDiags := m.gatherFile(f)
		diags = append(diags, fileDiags...)
	}
	return diags
}
