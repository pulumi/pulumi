package importer

import importer "github.com/pulumi/pulumi/sdk/v3/pkg/importer"

type PathedLiteralValue = importer.PathedLiteralValue

// ImportState tracks the state of an import process.
type ImportState = importer.ImportState

// Null represents Pulumi HCL2's `null` variable.
var Null = importer.Null

// GenerateHCL2Definition generates a Pulumi HCL2 definition for a given resource.
func GenerateHCL2Definition(loader schema.Loader, state *resource.State, importState ImportState) (*model.Block, *schema.PackageDescriptor, error) {
	return importer.GenerateHCL2Definition(loader, state, importState)
}

