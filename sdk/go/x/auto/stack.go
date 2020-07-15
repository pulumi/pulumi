package auto

import "github.com/pulumi/pulumi/sdk/v2/go/common/workspace"

// Project is a description of a pulumi project and corresponding source code
type Project struct {
	// Name of the project
	Name string
	//
	SourcePath string
	// Overrides is an optional set of values to overwrite in pulumi.yaml
	Overrides *ProjectOverrides
}

// Stack is a description of a pulumi stack
type Stack struct {
	// Name of the the stack
	Name string
	// Project is a description of the project to execute
	Project Project
	// Overrides is an optional set of values to overwrite in pulumi.<stack>.yaml
	Overrides *StackOverrides
}

// ProjectOverrides is an optional set of values to be merged with
// the existing pulumi.yaml
type ProjectOverrides struct {
	// Replace controls merge behavior with existing Pulumi.yaml files
	Replace bool
	Project *workspace.Project
}

// StackOverrides is an optional set of values to be merged with
// the existing pulumi.<stackName>.yaml
type StackOverrides struct {
	// Replace controls merge behavior with existing stack.yaml files.
	Replace bool
	// Config is an optional config bag to `pulumi config set`
	Config map[string]string
	// Secrets is an optional config bag to `pulumi config set --secret`
	Secrets map[string]string
	// TODO we should use a limited struct that prevents setting config directly
	// We want users to explicity handle config/secrets through above param
	// ProjectStack is the optional set of overrides
	ProjectStack *workspace.ProjectStack
}
