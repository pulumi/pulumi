package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Keywords in various languages should be renamed and work.
		class := "class_output_string"
		export := "export_output_string"
		_import := "import_output_string"
		mod := "mod_output_string"
		object := map[string]interface{}{
			"object": "object_output_string",
		}
		self := "self_output_string"
		this := "this_output_string"
		_if := "if_output_string"
		ctx.Export("class", pulumi.String(class))
		ctx.Export("export", pulumi.String(export))
		ctx.Export("import", pulumi.String(_import))
		ctx.Export("mod", pulumi.String(mod))
		ctx.Export("object", pulumi.ToMap(object))
		ctx.Export("self", pulumi.String(self))
		ctx.Export("this", pulumi.String(this))
		ctx.Export("if", pulumi.String(_if))
		return nil
	})
}
