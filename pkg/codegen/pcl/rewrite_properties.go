package pcl

import pcl "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/pcl"

func RewritePropertyReferences(expr model.Expression) model.Expression {
	return pcl.RewritePropertyReferences(expr)
}

