package python

import (
	"io"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
)

// Provides code for a method which will be placed in the program preamble if deemed
// necessary. Because many tasks in Go such as reading a file require extensive error
// handling, it is much prettier to encapsulate that error handling boilerplate as its
// own function in the preamble.
func getHelperMethodIfNeeded(functionName string) (string, bool) {
	switch functionName {
	case "filebase64sha256":
		return `def computeFilebase64sha256(path):
	fileData = open(path).read().encode()
	hashedData = hashlib.sha256(fileData.encode()).digest()
	return base64.b64encode(hashedData).decode()`, true
	default:
		return "", false
	}
}

func linearizeAndSetupGenerator(program *pcl.Program) ([]pcl.Node, *generator, error) {
	// Setup generator for procedural code generation
	g, err := newGenerator(program, true)
	if err != nil {
		return nil, nil, err
	}

	// Linearize the nodes into an order appropriate for procedural code generation.
	nodes := pcl.Linearize(program)
	return nodes, g, nil
}

func (g *generator) getModelDestType(r *pcl.Resource, attr *model.Attribute) model.Traversable {
	var destType model.Traversable
	var diagnostics hcl.Diagnostics
	if r.IsModule {
		// Attribute belonged to a Module with custom types not listed in any offcial schema. Defaulting to DynamicType
		destType = model.DynamicType
	} else {
		// Attribute belings to a typical resource, which has all its possible types coneniently listed
		destType, diagnostics = r.InputType.Traverse(hcl.TraverseAttr{Name: attr.Name})
		g.diagnostics = append(g.diagnostics, diagnostics...)
	}
	return destType
}

// Checks whether the resource has a `count` attribute, which indicates that a loop is required
// to fully initialize it.
func hasCountAttribute(r *pcl.Resource) bool {
	return r.Options != nil && r.Options.Range != nil
}

// Takes the local path from a terraform module, and creates a sensible looking python module reference from it
func makePythonModule(localPath string) string {
	var builder strings.Builder
	for i, c := range localPath {
		switch c {
		case '.':
			// Ignoring the leading period
			continue
		case '/':
			// Ignoring first '/'
			if i > 1 {
				// Replacing '/' with '.'
				builder.WriteRune('.')
			}
		default:
			// Replacing any other illegal runes as per usual
			if !isLegalIdentifierPart(c) {
				builder.WriteRune('_')
			} else {
				builder.WriteRune(c)
			}
		}
	}
	return builder.String()
}

// Prints the given rootname with a special prefix when a component resource is being generated, allowing it
// to reference component resource members or input arguments
func (g *generator) genRootNameWithPrefix(w io.Writer, rootName string, expr *model.ScopeTraversalExpression) {
	fmtString := "%s"
	switch expr.Parts[0].(type) {
	case *pcl.ConfigVariable:
		fmtString = "args.%s"
	}
	g.Fgenf(w, fmtString, rootName)
}
