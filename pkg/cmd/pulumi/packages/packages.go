package packages

import packages "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/packages"

type LinkPackageContext = packages.LinkPackageContext

type Provider = packages.Provider

// InstallPackage installs a package to the project by generating an SDK and linking it.
// It returns the path to the installed package.
func InstallPackage(proj workspace.BaseProject, pctx *plugin.Context, language, root, schemaSource string, parameters plugin.ParameterizeParameters, registry registry.Registry) (*schema.Package, *workspace.PackageSpec, hcl.Diagnostics, error) {
	return packages.InstallPackage(proj, pctx, language, root, schemaSource, parameters, registry)
}

func GenSDK(language, out string, pkg *schema.Package, overlays string, local bool) (hcl.Diagnostics, error) {
	return packages.GenSDK(language, out, pkg, overlays, local)
}

// LinkPackage links a locally generated SDK to an existing project.
// Currently Java is not supported and will print instructions for manual linking.
func LinkPackage(ctx *LinkPackageContext) error {
	return packages.LinkPackage(ctx)
}

// CopyAll copies src to dst. If src is a directory, its contents will be copied
// recursively.
func CopyAll(dst string, src string) error {
	return packages.CopyAll(dst, src)
}

func NewPluginContext(cwd string) (*plugin.Context, error) {
	return packages.NewPluginContext(cwd)
}

// SchemaFromSchemaSource takes a schema source and returns its associated schema. A
// schema source is either a file (ending with .[json|y[a]ml]) or a plugin with an
// optional version:
// 
// 	FILE.[json|y[a]ml] | PLUGIN[@VERSION] | PATH_TO_PLUGIN
func SchemaFromSchemaSource(pctx *plugin.Context, packageSource string, parameters plugin.ParameterizeParameters, registry registry.Registry) (*schema.Package, *workspace.PackageSpec, error) {
	return packages.SchemaFromSchemaSource(pctx, packageSource, parameters, registry)
}

// ProviderFromSource takes a plugin name or path.
// 
// PLUGIN[@VERSION] | PATH_TO_PLUGIN
func ProviderFromSource(pctx *plugin.Context, packageSource string, reg registry.Registry) (Provider, *workspace.PackageSpec, error) {
	return packages.ProviderFromSource(pctx, packageSource, reg)
}

