package pcl_test

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
)

func ParseAndBindProgram(t *testing.T,
	text string,
	name string,
	options ...pcl.BindOption,
) (*pcl.Program, hcl.Diagnostics, error) {
	parser := syntax.NewParser()
	err := parser.ParseFile(strings.NewReader(text), name)
	if err != nil {
		t.Fatalf("could not read %v: %v", name, err)
	}
	if parser.Diagnostics.HasErrors() {
		t.Fatalf("failed to parse files: %v", parser.Diagnostics)
	}

	options = append(options, pcl.PluginHost(utils.NewHost(testdataPath)))

	return pcl.BindProgram(parser.Files, options...)
}
