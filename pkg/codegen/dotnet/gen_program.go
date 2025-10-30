package dotnet

import dotnet "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/dotnet"

type GenerateProgramOptions = dotnet.GenerateProgramOptions

type ObjectTypeFromConfigMetadata = dotnet.ObjectTypeFromConfigMetadata

func GenerateProgramWithOptions(program *pcl.Program, options GenerateProgramOptions) (map[string][]byte, hcl.Diagnostics, error) {
	return dotnet.GenerateProgramWithOptions(program, options)
}

func GenerateProgram(program *pcl.Program) (map[string][]byte, hcl.Diagnostics, error) {
	return dotnet.GenerateProgram(program)
}

func GenerateProject(directory string, project workspace.Project, program *pcl.Program, localDependencies map[string]string) error {
	return dotnet.GenerateProject(directory, project, program, localDependencies)
}

func AnnotateComponentInputs(component *pcl.Component) {
	dotnet.AnnotateComponentInputs(component)
}

