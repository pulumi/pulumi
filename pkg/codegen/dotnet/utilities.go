package dotnet

import dotnet "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/dotnet"

// LowerCamelCase sets the first character to lowercase
// LowerCamelCase("LowerCamelCase") -> "lowerCamelCase"
func LowerCamelCase(s string) string {
	return dotnet.LowerCamelCase(s)
}

