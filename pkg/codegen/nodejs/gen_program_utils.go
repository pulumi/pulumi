package nodejs

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/format"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// Provides code for a method which will be placed in the program preamble if deemed
// necessary. Because many tasks in Go such as reading a file require extensive error
// handling, it is much prettier to encapsulate that error handling boilerplate as its
// own function in the preamble.
func getHelperMethodIfNeeded(functionName string) (string, bool) {
	switch functionName {
	case "filebase64sha256":
		return `func computeFilebase64sha256(path string) string {
	const fileData = Buffer.from(fs.readFileSync(path), 'binary')
	return crypto.createHash('sha256').update(fileData).digest('hex')
}`, true
	default:
		return "", false
	}
}

func linearizeAndSetupGenerator(program *pcl.Program) ([]pcl.Node, *generator, error) {
	// Linearize the nodes into an order appropriate for procedural code generation.
	nodes := pcl.Linearize(program)

	// Setup generator for procedural code generation
	g := &generator{
		program:   program,
		asyncMain: needAsyncMain(&nodes),
	}

	g.Formatter = format.NewFormatter(g)

	// Verify packages
	packages, err := program.PackageSnapshots()
	if err != nil {
		return nil, nil, err
	}
	for _, p := range packages {
		if err := p.ImportLanguages(map[string]schema.Language{"nodejs": Importer}); err != nil {
			return nil, nil, err
		}
	}

	return nodes, g, nil
}

func needAsyncMain(nodes *[]pcl.Node) bool {
	for _, n := range *nodes {
		switch x := n.(type) {
		case *pcl.Resource:
			if resourceRequiresAsyncMain(x) {
				return true
			}
		case *pcl.OutputVariable:
			if outputRequiresAsyncMain(x) {
				return true
			}
		}
	}
	return false
}

func (g *generator) getResourceTypeName(r *pcl.Resource) string {
	pkg, module, memberName, diagnostics := resourceTypeName(r)
	g.diagnostics = append(g.diagnostics, diagnostics...)

	if module != "" {
		module = "." + module
	}

	return fmt.Sprintf("%s%s.%s", pkg, module, memberName)
}

func (g *generator) getModelDestType(r *pcl.Resource, attr *model.Attribute) model.Traversable {
	var destType model.Traversable
	var diagnostics hcl.Diagnostics
	if r.IsComponentResource {
		// Attribute belonged to a Module with custom types not listed in any offcial schema. Defaulting to DynamicType
		destType = model.DynamicType
	} else {
		// Attribute belings to a typical resource, which has all its possible types coneniently listed
		destType, diagnostics = r.InputType.Traverse(hcl.TraverseAttr{Name: attr.Name})
		g.diagnostics = append(g.diagnostics, diagnostics...)
	}
	return destType
}
