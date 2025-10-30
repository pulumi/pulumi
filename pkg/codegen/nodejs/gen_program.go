package nodejs

import nodejs "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/nodejs"

const PulumiToken = nodejs.PulumiToken

func GenerateProgram(program *pcl.Program) (map[string][]byte, hcl.Diagnostics, error) {
	return nodejs.GenerateProgram(program)
}

func GenerateProject(directory string, project workspace.Project, program *pcl.Program, localDependencies map[string]string, forceTsc bool) error {
	return nodejs.GenerateProject(directory, project, program, localDependencies, forceTsc)
}

