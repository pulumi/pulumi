package dotnet

import dotnet "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/dotnet"

// LanguageResource is derived from the schema and can be used by downstream codegen.
type LanguageResource = dotnet.LanguageResource

// Title converts the input string to a title case
// where only the initial letter is upper-cased.
func Title(s string) string {
	return dotnet.Title(s)
}

// LanguageResources returns a map of resources that can be used by downstream codegen. The map
// key is the resource schema token.
func LanguageResources(tool string, pkg *schema.Package) (map[string]LanguageResource, error) {
	return dotnet.LanguageResources(tool, pkg)
}

func GeneratePackage(tool string, pkg *schema.Package, extraFiles map[string][]byte, localDependencies map[string]string) (map[string][]byte, error) {
	return dotnet.GeneratePackage(tool, pkg, extraFiles, localDependencies)
}

