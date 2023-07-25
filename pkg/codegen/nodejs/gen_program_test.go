package nodejs

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/test"
	"github.com/stretchr/testify/assert"
)

func TestGenerateProgramVersionSelection(t *testing.T) {
	t.Parallel()

	expectedVersion := map[string]test.PkgVersionInfo{
		"aws-resource-options-4.26": {
			Pkg:          "\"@pulumi/aws\"",
			OpAndVersion: "\"4.26.0\"",
		},
		"aws-resource-options-5.16.2": {
			Pkg:          "\"@pulumi/aws\"",
			OpAndVersion: "\"5.16.2\"",
		},
	}

	test.TestProgramCodegen(t,
		test.ProgramCodegenOptions{
			Language:   "nodejs",
			Extension:  "ts",
			OutputFile: "index.ts",
			Check: func(t *testing.T, path string, dependencies codegen.StringSet) {
				Check(t, path, dependencies, true)
			},
			GenProgram: GenerateProgram,
			TestCases: []test.ProgramTest{
				{
					Directory:   "aws-resource-options-4.26",
					Description: "Resource Options",
				},
				{
					Directory:   "aws-resource-options-5.16.2",
					Description: "Resource Options",
				},
			},

			IsGenProject:    true,
			GenProject:      GenerateProject,
			ExpectedVersion: expectedVersion,
			DependencyFile:  "package.json",
		})
}

func TestEnumReferencesCorrectIdentifier(t *testing.T) {
	t.Parallel()
	s := &schema.Package{
		Name: "pulumiservice",
		Language: map[string]interface{}{
			"nodejs": NodePackageInfo{
				PackageName: "@pulumi/bar",
			},
		},
	}
	result, err := enumNameWithPackage("pulumiservice:index:WebhookFilters", s.Reference())
	assert.NoError(t, err)
	assert.Equal(t, "pulumiservice.WebhookFilters", result)

	// These are redundant, but serve to clarify our expectations around package alias names.
	assert.NotEqual(t, "bar.WebhookFilters", result)
	assert.NotEqual(t, "@pulumi/bar.WebhookFilters", result)
}
