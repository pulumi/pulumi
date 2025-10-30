package codegen

import codegen "github.com/pulumi/pulumi/sdk/v3/pkg/codegen"

// DocLanguageHelper is an interface for extracting language-specific information from a Pulumi schema.
// See the implementation for this interface under each of the language code generators.
type DocLanguageHelper = codegen.DocLanguageHelper

// FilterExamples filters the code snippets in a schema docstring to include only those that target the given language.
func FilterExamples(description string, lang string) string {
	return codegen.FilterExamples(description, lang)
}

