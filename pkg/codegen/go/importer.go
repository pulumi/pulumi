package gen

import gen "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/go"

// GoPackageInfo holds information required to generate the Go SDK from a schema.
type GoPackageInfo = gen.GoPackageInfo

// Importer implements schema.Language for Go.
var Importer = gen.Importer

