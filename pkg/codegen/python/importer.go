package python

import python "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/python"

// PropertyInfo tracks Python-specific information associated with properties in a package.
type PropertyInfo = python.PropertyInfo

// PackageInfo tracks Python-specific information associated with a package.
type PackageInfo = python.PackageInfo

// Importer implements schema.Language for Python.
var Importer = python.Importer

