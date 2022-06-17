package python

import (
	"bytes"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type ResourceOptionsGenerator struct{}

var _ codegen.ResourceOptionsGenerator = (*ResourceOptionsGenerator)(nil)

func (g *ResourceOptionsGenerator) GenerateResourceOptions(opts codegen.ResourceOptions) ([]byte, error) {
	var w bytes.Buffer
	ctx := &modContext{pkg: &schema.Package{Name: "pulumi"}}

	err := ctx.genType(&w, "ResourceOptions", opts.DocComment, opts.Properties, false, false, true)
	if err != nil {
		return nil, err
	}
	merge, err := g.appendMergeFunction(opts.Properties)
	if err != nil {
		return nil, err
	}
	return append(w.Bytes(), merge...), nil
}

func (_ *ResourceOptionsGenerator) appendMergeFunction(props []*schema.Property) ([]byte, error) {
	var buf bytes.Buffer
	w := NewWriter(&buf).IncrIndent(4)
	w.Printf("\n# pyling: disable=method-hidden")
	w.Printf("\n@staticmethod")
	w.Printf("\ndef merge(")
	w.IncrIndent(4).Printf("\n" + `opts1: Optional["ResourceOptions"], opts2: Optional["ResourceOptions"]`)
	w.Printf("\n) -> \"ResourceOptions\":")

	// We are now in the method body
	w = w.IncrIndent(4)
	w.Printf(`
opts1 = ResourceOptions() if opts1 is None else opts1
opts2 = ResourceOptions() if opts2 is None else opts2

if not isinstance(opts1, ResourceOptions):
    raise TypeError("Expected opts1 to be a ResourceOptions instance")

if not isinstance(opts2, ResourceOptions):
    raise TypeError("Expected opts2 to be a ResourceOptions instance")

dest = copy.copy(opts1)
source = copy.copy(opts2)
`)
	w.Printf("\nreturn ResourceOptions(")
	for _, prop := range props {
		w := w.IncrIndent(4)
		pname := PyName(prop.Name)
		w.Printf("\n%[1]s = dest.%[1]s if source.%[1]s is None else source.%[1]s,", pname)
	}

	w.Printf("\n)")

	return buf.Bytes(), nil
}
