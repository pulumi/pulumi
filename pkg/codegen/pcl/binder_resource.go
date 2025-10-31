package pcl

import pcl "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/pcl"

// ResourceAnnotation is a type that can be used to annotate ObjectTypes that represent resources with their
// corresponding Resource node. We define a wrapper type that does not implement any interfaces so as to reduce the
// chance of the annotation being plucked out by an interface-type query by accident.
type ResourceAnnotation = pcl.ResourceAnnotation

func AnnotateAttributeValue(expr model.Expression, attributeType schema.Type) model.Expression {
	return pcl.AnnotateAttributeValue(expr, attributeType)
}

func AnnotateResourceInputs(node *Resource) {
	pcl.AnnotateResourceInputs(node)
}

