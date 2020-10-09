package provider

import (
	"go/ast"
	"go/token"
	"io"
	"io/ioutil"

	"github.com/hashicorp/hcl/v2"
	"golang.org/x/tools/go/packages"
)

func hclPos(files *token.FileSet, pos token.Pos) (hcl.Pos, string) {
	if !pos.IsValid() {
		return hcl.Pos{}, ""
	}

	position := files.Position(pos)
	return hcl.Pos{
		Line:   position.Line,
		Column: position.Column,
		Byte:   position.Offset,
	}, position.Filename
}

func nodeRange(files *token.FileSet, node ast.Node) *hcl.Range {
	if node == nil {
		return nil
	}

	startPos, endPos := node.Pos(), node.End()
	if !startPos.IsValid() || !endPos.IsValid() {
		return nil
	}

	start, filename := hclPos(files, startPos)
	end, _ := hclPos(files, endPos)
	return &hcl.Range{
		Filename: filename,
		Start:    start,
		End:      end,
	}
}

func newError(files *token.FileSet, node ast.Node, message string) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Subject:  nodeRange(files, node),
		Severity: hcl.DiagError,
		Summary:  message,
	}
}

func newWarning(files *token.FileSet, node ast.Node, message string) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Subject:  nodeRange(files, node),
		Severity: hcl.DiagWarning,
		Summary:  message,
	}
}

func NewDiagnosticWriter(w io.Writer, width uint, color bool, pkgs ...*packages.Package) (hcl.DiagnosticWriter, error) {
	fileMap := map[string]*hcl.File{}
	for _, pkg := range pkgs {
		for _, filePath := range pkg.CompiledGoFiles {
			contents, err := ioutil.ReadFile(filePath)
			if err != nil {
				return nil, err
			}
			fileMap[filePath] = &hcl.File{Bytes: contents}
		}
	}
	return hcl.NewDiagnosticTextWriter(w, fileMap, width, color), nil
}
