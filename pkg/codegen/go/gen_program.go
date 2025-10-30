package gen

import gen "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/go"

// GenerateProgramOptions are used to configure optional generator behavior.
type GenerateProgramOptions = gen.GenerateProgramOptions

type ObjectTypeFromConfigMetadata = gen.ObjectTypeFromConfigMetadata

const IndexToken = gen.IndexToken

func GenerateProgram(program *pcl.Program) (map[string][]byte, hcl.Diagnostics, error) {
	return gen.GenerateProgram(program)
}

func GenerateProgramWithOptions(program *pcl.Program, opts GenerateProgramOptions) (map[string][]byte, hcl.Diagnostics, error) {
	return gen.GenerateProgramWithOptions(program, opts)
}

func GenerateProjectFiles(project workspace.Project, program *pcl.Program, localDependencies map[string]string) (map[string][]byte, hcl.Diagnostics, error) {
	return gen.GenerateProjectFiles(project, program, localDependencies)
}

func GenerateProject(directory string, project workspace.Project, program *pcl.Program, localDependencies map[string]string) error {
	return gen.GenerateProject(directory, project, program, localDependencies)
}

func AnnotateComponentInputs(component *pcl.Component) {
	gen.AnnotateComponentInputs(component)
}

