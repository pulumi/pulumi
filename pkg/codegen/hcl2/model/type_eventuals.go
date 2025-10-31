package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// ResolveOutputs recursively replaces all output(T) and promise(T) types in the input type with their element type.
func ResolveOutputs(t Type) Type {
	return model.ResolveOutputs(t)
}

// ResolvePromises recursively replaces all promise(T) types in the input type with their element type.
func ResolvePromises(t Type) Type {
	return model.ResolvePromises(t)
}

// ContainsEventuals returns true if the input type contains output or promise types.
func ContainsEventuals(t Type) (containsOutputs , containsPromises bool) {
	return model.ContainsEventuals(t)
}

// ContainsOutputs returns true if the input type contains output types.
func ContainsOutputs(t Type) bool {
	return model.ContainsOutputs(t)
}

// ContainsPromises returns true if the input type contains promise types.
func ContainsPromises(t Type) bool {
	return model.ContainsPromises(t)
}

// InputType returns the result of replacing each type in T with union(T, output(T)).
func InputType(t Type) Type {
	return model.InputType(t)
}

