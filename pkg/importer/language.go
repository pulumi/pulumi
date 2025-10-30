package importer

import importer "github.com/pulumi/pulumi/sdk/v3/pkg/importer"

// A LangaugeGenerator generates code for a given Pulumi program to an io.Writer.
type LanguageGenerator = importer.LanguageGenerator

// A NameTable maps URNs to language-specific variable names.
type NameTable = importer.NameTable

// A DiagnosticsError captures HCL2 diagnostics.
type DiagnosticsError = importer.DiagnosticsError

// GenerateLanguageDefintions generates a list of resource definitions for the given resource states. The current stack
// snapshot is also provided in order to allow the importer to resolve package providers.
func GenerateLanguageDefinitions(w io.Writer, loader schema.Loader, gen LanguageGenerator, states []*resource.State, snapshot []*resource.State, names NameTable) error {
	return importer.GenerateLanguageDefinitions(w, loader, gen, states, snapshot, names)
}

