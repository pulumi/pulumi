//go:build exclude

package utilities

import utilities "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/go"

func ParseEnvBool(v string) interface{} {
	return utilities.ParseEnvBool(v)
}

func ParseEnvInt(v string) interface{} {
	return utilities.ParseEnvInt(v)
}

func ParseEnvFloat(v string) interface{} {
	return utilities.ParseEnvFloat(v)
}

func ParseEnvStringArray(v string) interface{} {
	return utilities.ParseEnvStringArray(v)
}

func GetEnvOrDefault(def interface{}, parser envParser, vars ...string) interface{} {
	return utilities.GetEnvOrDefault(def, parser, vars...)
}

// PkgVersion uses reflection to determine the version of the current package.
// If a version cannot be determined, v1 will be assumed. The second return
// value is always nil.
func PkgVersion() (semver.Version, error) {
	return utilities.PkgVersion()
}

// isZero is a null safe check for if a value is it's types zero value.
func IsZero(v interface{}) bool {
	return utilities.IsZero(v)
}

func CallPlain(ctx *pulumi.Context, tok string, args pulumi.Input, output pulumi.Output, self pulumi.Resource, property string, resultPtr reflect.Value, errorPtr *error, opts ...pulumi.InvokeOption) {
	utilities.CallPlain(ctx, tok, args, output, self, property, resultPtr, errorPtr, opts...)
}

