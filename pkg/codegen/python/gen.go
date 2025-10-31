package python

import python "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/python"

// LanguageResource is derived from the schema and can be used by downstream codegen.
type LanguageResource = python.LanguageResource

const InputTypesSettingClasses = python.InputTypesSettingClasses

const InputTypesSettingClassesAndDicts = python.InputTypesSettingClassesAndDicts

// Require the SDK to fall within the same major version.
var MinimumValidSDKVersion = python.MinimumValidSDKVersion

// PyPack returns the suggested package name for the given string.
func PyPack(namespace, name string) string {
	return python.PyPack(namespace, name)
}

// InitParamName returns a PyName-encoded name but also deduplicates the name against built-in parameters of resource __init__.
func InitParamName(name string) string {
	return python.InitParamName(name)
}

// LanguageResources returns a map of resources that can be used by downstream codegen. The map
// key is the resource schema token.
func LanguageResources(tool string, pkg *schema.Package) (map[string]LanguageResource, error) {
	return python.LanguageResources(tool, pkg)
}

func GeneratePackage(tool string, pkg *schema.Package, extraFiles map[string][]byte, loader schema.ReferenceLoader) (map[string][]byte, error) {
	return python.GeneratePackage(tool, pkg, extraFiles, loader)
}

