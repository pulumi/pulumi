package nodejs

import nodejs "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/nodejs"

// LanguageResource holds information about a resource to be used by downstream codegen.
type LanguageResource = nodejs.LanguageResource

// LanguageProperty holds information about a resource property to be used by downstream codegen.
type LanguageProperty = nodejs.LanguageProperty

const MinimumValidSDKVersion = nodejs.MinimumValidSDKVersion

const MinimumTypescriptVersion = nodejs.MinimumTypescriptVersion

const MinimumNodeTypesVersion = nodejs.MinimumNodeTypesVersion

// LanguageResources returns a map of resources that can be used by downstream codegen. The map
// key is the resource schema token.
func LanguageResources(pkg *schema.Package) (map[string]LanguageResource, error) {
	return nodejs.LanguageResources(pkg)
}

func GeneratePackage(tool string, pkg *schema.Package, extraFiles map[string][]byte, localDependencies map[string]string, localSDK bool, loader schema.ReferenceLoader) (map[string][]byte, error) {
	return nodejs.GeneratePackage(tool, pkg, extraFiles, localDependencies, localSDK, loader)
}

