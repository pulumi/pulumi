package convert

import convert "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/convert"

func LoadConverterPlugin(ctx *plugin.Context, name string, log func(diag.Severity, string)) (plugin.Converter, error) {
	return convert.LoadConverterPlugin(ctx, name, log)
}

