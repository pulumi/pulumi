package pcl

import pcl "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/pcl"

type NameInfo = pcl.NameInfo

func HasEventualTypes(t model.Type) bool {
	return pcl.HasEventualTypes(t)
}

// RewriteApplies transforms all expressions that observe the resolved values of outputs and promises into calls to the
// __apply intrinsic. Expressions that generate or inspect outputs or promises are passed as arguments to these calls,
// and are replaced by references to the corresponding parameter.
// 
// As an example, assuming that resource.id is an output, this transforms the following expression:
// 
// 	toJSON({
// 	    Version = "2012-10-17"
// 	    Statement = [{
// 	        Effect = "Allow"
// 	        Principal = "*"
// 	        Action = [ "s3:GetObject" ]
// 	        Resource = [ "arn:aws:s3:::${resource.id}/*" ]
// 	    }]
// 	})
// 
// into this expression:
// 
// 	__apply(resource.id, eval(id, toJSON({
// 	    Version = "2012-10-17"
// 	    Statement = [{
// 	        Effect = "Allow"
// 	        Principal = "*"
// 	        Action = [ "s3:GetObject" ]
// 	        Resource = [ "arn:aws:s3:::${id}/*" ]
// 	    }]
// 	})))
// 
// Here is a more advanced example, assuming that resource is an object whose properties are all outputs, this
// expression:
// 
// 	"v: ${resource[resource.id]}"
// 
// is transformed into this expression:
// 
// 	__apply(__apply(resource.id,eval(id, resource[id])),eval(id, "v: ${id}"))
// 
// This form is amenable to code generation for targets that require that outputs are resolved before their values are
// accessible (e.g. Pulumi's JS/TS libraries).
func RewriteApplies(expr model.Expression, nameInfo NameInfo, applyPromises bool) (model.Expression, hcl.Diagnostics) {
	return pcl.RewriteApplies(expr, nameInfo, applyPromises)
}

func RewriteAppliesWithSkipToJSON(expr model.Expression, nameInfo NameInfo, applyPromises bool, skipToJSON bool) (model.Expression, hcl.Diagnostics) {
	return pcl.RewriteAppliesWithSkipToJSON(expr, nameInfo, applyPromises, skipToJSON)
}

