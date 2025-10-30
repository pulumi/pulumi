package workspace

import workspace "github.com/pulumi/pulumi/sdk/v3/pkg/workspace"

// ValidateProjectName ensures a project name is valid, if it is not it returns an error with a message suitable
// for display to an end user.
func ValidateProjectName(s string) error {
	return workspace.ValidateProjectName(s)
}

// ValueOrSanitizedDefaultProjectName returns the value or a sanitized valid project name
// based on defaultNameToSanitize.
func ValueOrSanitizedDefaultProjectName(name string, projectName string, defaultNameToSanitize string) string {
	return workspace.ValueOrSanitizedDefaultProjectName(name, projectName, defaultNameToSanitize)
}

// ValueOrDefaultProjectDescription returns the value or defaultDescription.
func ValueOrDefaultProjectDescription(description string, projectDescription string, defaultDescription string) string {
	return workspace.ValueOrDefaultProjectDescription(description, projectDescription, defaultDescription)
}

// ValidateProjectDescription ensures a project description name is valid, if it is not it returns an error with a
// message suitable for display to an end user.
func ValidateProjectDescription(s string) error {
	return workspace.ValidateProjectDescription(s)
}

