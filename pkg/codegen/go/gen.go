package gen

import gen "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/go"

// A threadsafe cache for sharing between invocations of GenerateProgram.
type Cache = gen.Cache

// LanguageResource is derived from the schema and can be used by downstream codegen.
type LanguageResource = gen.LanguageResource

// A signifier that the module is external, and will never match.
// 
// This token is always an invalid module since ':' is not allowed within modules.
const ExternalModuleSig = gen.ExternalModuleSig

const GenericsSettingNone = gen.GenericsSettingNone

const GenericsSettingSideBySide = gen.GenericsSettingSideBySide

const GenericsSettingGenericsOnly = gen.GenericsSettingGenericsOnly

// The name of the method used to instantiate defaults.
const ProvideDefaultsMethodName = gen.ProvideDefaultsMethodName

// Title converts the input string to a title case
// where only the initial letter is upper-cased.
// It also removes $-prefix if any.
func Title(s string) string {
	return gen.Title(s)
}

func NewCache() *Cache {
	return gen.NewCache()
}

func NeedsGoOutputVersion(f *schema.Function) bool {
	return gen.NeedsGoOutputVersion(f)
}

// LanguageResources returns a map of resources that can be used by downstream codegen. The map
// key is the resource schema token.
func LanguageResources(tool string, pkg *schema.Package) (map[string]LanguageResource, error) {
	return gen.LanguageResources(tool, pkg)
}

func GeneratePackage(tool string, pkg *schema.Package, localDependencies map[string]string) (map[string][]byte, error) {
	return gen.GeneratePackage(tool, pkg, localDependencies)
}

