package pcl

import pcl "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/pcl"

const IntrinsicApply = pcl.IntrinsicApply

const IntrinsicConvert = pcl.IntrinsicConvert

// NewApplyCall returns a new expression that represents a call to IntrinsicApply.
func NewApplyCall(args []model.Expression, then *model.AnonymousFunctionExpression) *model.FunctionCallExpression {
	return pcl.NewApplyCall(args, then)
}

// ParseApplyCall extracts the apply arguments and the continuation from a call to the apply intrinsic.
func ParseApplyCall(c *model.FunctionCallExpression) (applyArgs []model.Expression, then *model.AnonymousFunctionExpression) {
	return pcl.ParseApplyCall(c)
}

// NewConvertCall returns a new expression that represents a call to IntrinsicConvert.
func NewConvertCall(from model.Expression, to model.Type) *model.FunctionCallExpression {
	return pcl.NewConvertCall(from, to)
}

// ParseConvertCall extracts the value being converted and the type it is being converted to from a call to the convert
// intrinsic.
func ParseConvertCall(c *model.FunctionCallExpression) (model.Expression, model.Type) {
	return pcl.ParseConvertCall(c)
}

