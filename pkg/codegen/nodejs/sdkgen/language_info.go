package sdkgen

// Compatibility mode for Kubernetes 2.0 SDK
const kubernetes20 = "kubernetes20"

// Compatibility mode for tfbridge 2.x SDKs
const tfbridge20 = "tfbridge20"

// NodePackageInfo contains NodeJS-specific information for a package.
type NodePackageInfo struct {
	// Custom name for the NPM package.
	PackageName string `json:"packageName,omitempty"`
	// Description for the NPM package.
	PackageDescription string `json:"packageDescription,omitempty"`
	// Readme contains the text for the package's README.md files.
	Readme string `json:"readme,omitempty"`
	// NPM dependencies to add to package.json.
	Dependencies map[string]string `json:"dependencies,omitempty"`
	// NPM dev-dependencies to add to package.json.
	DevDependencies map[string]string `json:"devDependencies,omitempty"`
	// NPM peer-dependencies to add to package.json.
	PeerDependencies map[string]string `json:"peerDependencies,omitempty"`
	// NPM resolutions to add to package.json
	Resolutions map[string]string `json:"resolutions,omitempty"`
	// A specific version of TypeScript to include in package.json.
	TypeScriptVersion string `json:"typescriptVersion,omitempty"`
	// A map containing overrides for module names to package names.
	ModuleToPackage map[string]string `json:"moduleToPackage,omitempty"`
	// Toggle compatibility mode for a specified target.
	Compatibility string `json:"compatibility,omitempty"`
	// Disable support for unions in output types.
	DisableUnionOutputTypes bool `json:"disableUnionOutputTypes,omitempty"`
	// An indicator for whether the package contains enums.
	ContainsEnums bool `json:"containsEnums,omitempty"`
	// A map allowing you to map the name of a provider to the name of the module encapsulating the provider.
	ProviderNameToModuleName map[string]string `json:"providerNameToModuleName,omitempty"`
	// Additional files to include in TypeScript compilation.
	// These paths are added to the `files` section of the
	// generated `tsconfig.json`. A typical use case for this is
	// compiling hand-authored unit test files that check the
	// generated code.
	ExtraTypeScriptFiles []string `json:"extraTypeScriptFiles,omitempty"`
	// Determines whether to make single-return-value methods return an output object or the single value.
	LiftSingleValueMethodReturns bool `json:"liftSingleValueMethodReturns,omitempty"`

	// Respect the Pkg.Version field in the schema
	RespectSchemaVersion bool `json:"respectSchemaVersion,omitempty"`

	// Experimental flag that permits `import type *` style code
	// to be generated to optimize startup time of programs
	// consuming the provider by minimizing the set of Node
	// modules loaded at startup. Turning this on may currently
	// generate non-compiling code for some providers; but if the
	// code compiles it is safe to use. Also, turning this on
	// requires TypeScript 3.8 or higher to compile the generated
	// code.
	UseTypeOnlyReferences bool `json:"useTypeOnlyReferences,omitempty"`
}

// NodeObjectInfo contains NodeJS-specific information for an object.
type NodeObjectInfo struct {
	// List of properties that are required on the input side of a type.
	RequiredInputs []string `json:"requiredInputs"`
	// List of properties that are required on the output side of a type.
	RequiredOutputs []string `json:"requiredOutputs"`
}
