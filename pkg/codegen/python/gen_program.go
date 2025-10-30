package python

import python "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/python"

func GenerateProgram(program *pcl.Program) (map[string][]byte, hcl.Diagnostics, error) {
	return python.GenerateProgram(program)
}

func GenerateProject(directory string, project workspace.Project, program *pcl.Program, localDependencies map[string]string, typechecker, toolchain string) error {
	return python.GenerateProject(directory, project, program, localDependencies, typechecker, toolchain)
}

