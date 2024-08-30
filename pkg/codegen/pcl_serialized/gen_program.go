package pcl_serialized

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

func GenerateProgram(program *pcl.Program) (map[string][]byte, hcl.Diagnostics, error) {

	var p = codegen.Package{
		Nodes: nil,
	}

	p.String()

	return nil, nil, nil
}
