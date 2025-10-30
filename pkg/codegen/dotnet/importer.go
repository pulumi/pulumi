package dotnet

import dotnet "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/dotnet"

// CSharpPropertyInfo represents the C# language-specific info for a property.
type CSharpPropertyInfo = dotnet.CSharpPropertyInfo

// CSharpResourceInfo represents the C# language-specific info for a resource.
type CSharpResourceInfo = dotnet.CSharpResourceInfo

// CSharpPackageInfo represents the C# language-specific info for a package.
type CSharpPackageInfo = dotnet.CSharpPackageInfo

// Importer implements schema.Language for .NET.
var Importer = dotnet.Importer

