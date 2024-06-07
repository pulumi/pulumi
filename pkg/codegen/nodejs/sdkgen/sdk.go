package sdkgen

import "github.com/pulumi/pulumi/pkg/v3/codegen/nodejs/codebase"

func pulumiSymbol(name string) codebase.QualifiedSymbol {
	return codebase.QualifiedSymbol{
		Module:        "@pulumi/pulumi",
		Qualification: "pulumi",
		Namespace:     "",
		Name:          name,
	}
}

var PProviderResource = pulumiSymbol("ProviderResource")

var PResourceOptions = pulumiSymbol("ResourceOptions")

var PComponentResource = pulumiSymbol("ComponentResource")

var PComponentResourceOptions = pulumiSymbol("ComponentResourceOptions")

var PCustomResource = pulumiSymbol("CustomResource")

var PCustomResourceOptions = pulumiSymbol("CustomResourceOptions")

var PID = pulumiSymbol("ID")

var PInput = pulumiSymbol("Input")

func InputT(m *codebase.Module, t codebase.Type) codebase.Type {
	return m.QualifiedSymbolImport(PInput).AsType().Apply(t)
}

var POutput = pulumiSymbol("Output")

func OutputT(m *codebase.Module, t codebase.Type) codebase.Type {
	return m.QualifiedSymbolImport(POutput).AsType().Apply(t)
}
