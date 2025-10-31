package python

import python "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/python"

// The specification for the pyproject.toml file can be found here.
// https://packaging.python.org/en/latest/specifications/declaring-project-metadata/
type PyprojectSchema = python.PyprojectSchema

// Project is a view layer for a pyproject.toml file.
type Project = python.Project

type BuildSystem = python.BuildSystem

// Contact references someone associated with the project, including
// their contact information. Contacts are used for both Authors and
// Maintainers, since both fields have the same schema and specification.
// It is often easier to specify both fields,
// but the precise rules for specifying either one or the other field
// can be found here:
// https://packaging.python.org/en/latest/specifications/declaring-project-metadata/#authors-maintainers
type Contact = python.Contact

// An Entrypoint is an object reference for an executable Python script. These
// scripts can be applications, plugins, or build-time metadata. Since Pulumi
// distributes libraries, we largely don't use this field, though we include it
// for completeness and consistency with the spec.
type Entrypoints = python.Entrypoints

// The license instance must populate either
// file or text, but not both. File is a path
// to a license file, while text is either the
// name of the license, or the text of the license.
type License = python.License

// OptionalDependencies provides a map from "Extras" (parlance specific to Python)
// to their dependencies. Each value in the array becomes a required dependency
// if the Extra is enabled.
type OptionalDependencies = python.OptionalDependencies

