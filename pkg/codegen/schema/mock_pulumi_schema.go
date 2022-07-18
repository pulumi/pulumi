package schema

import (
	"github.com/blang/semver"
)

const (
	pulumiPkgName = "pulumi"
)

type pulumiPackageReference struct {
	name        string
	version     *semver.Version
	description string
	types       PackageTypes
	config      []*Property
	provider    *Resource
	resources   PackageResources
	functions   PackageFunctions
	module      string
	packageFull *Package
}

func newPulumiPackageReference(name string, version *semver.Version, description string, types PackageTypes,
	config []*Property, provider *Resource, resources PackageResources,
	functions PackageFunctions, module string, packageFull *Package) *pulumiPackageReference {
	return &pulumiPackageReference{
		name:        name,
		version:     version,
		description: description,
		types:       types,
		config:      config,
		provider:    provider,
		resources:   resources,
		functions:   functions,
		module:      module,
		packageFull: packageFull,
	}
}

func (p *pulumiPackageReference) Name() string {
	return p.name
}

func (p *pulumiPackageReference) Version() *semver.Version {
	return p.version
}

func (p *pulumiPackageReference) Description() string {
	return p.description
}

func (p *pulumiPackageReference) Config() ([]*Property, error) {
	return p.config, nil
}

func (p *pulumiPackageReference) Provider() (*Resource, error) {
	return p.provider, nil
}

func (p *pulumiPackageReference) Resources() PackageResources {
	return p.resources
}

func (p *pulumiPackageReference) Functions() PackageFunctions {
	return p.functions
}

func (p *pulumiPackageReference) Types() PackageTypes {
	return p.types
}

func (p *pulumiPackageReference) TokenToModule(token string) string {
	return p.module
}

func (p *pulumiPackageReference) Definition() (*Package, error) {
	return p.packageFull, nil
}

var DefaultPulumiPackageReference = newPulumiPackageReference(DefaultPulumiPackage.Name,
	DefaultPulumiPackage.Version, DefaultPulumiPackage.Description, DefaultPulumiPackage.Reference().Types(),
	DefaultPulumiPackage.Config, DefaultPulumiPackage.Provider, DefaultPulumiPackage.Reference().Resources(),
	DefaultPulumiPackage.Reference().Functions(), pulumiPkgName, &DefaultPulumiPackage)

var DefaultPulumiPackage = Package{
	Name:        pulumiPkgName,
	DisplayName: "Pulumi",
	Version: &semver.Version{
		Major: 1,
		Minor: 0,
		Patch: 0,
	},
	Description: "mock pulumi package",
	Types:       []Type{},
	Config:      []*Property{},
	Provider:    &Resource{},
	Resources: []*Resource{
		&stackReferenceResource,
	},

	resourceTable: map[string]*Resource{
		"pulumi:pulumi:StackReference": &stackReferenceResource,
	},
}

var stackReferenceResource = Resource{
	Token: "pulumi:pulumi:StackReference",
	InputProperties: []*Property{
		{
			Name: "name",
			Type: StringType,
		},
	},
	Properties: []*Property{
		{
			Name: "outputs",
			Type: &MapType{},
		},
	},
}
