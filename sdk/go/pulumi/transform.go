package pulumi

// ResourceTransformArgs is the argument bag passed to a resource transform.
type ResourceTransformArgs struct {
	// If the resource is a custom or component resource
	Custom bool
	// The type of the resource.
	Type string
	// The name of the resource.
	Name string
	// The original properties passed to the resource constructor.
	Props map[string]any
	// The original resource options passed to the resource constructor.
	Opts ResourceOptions
}

// ResourceTransformResult is the result that must be returned by a resource transform
// callback.  It includes new values to use for the `props` and `opts` of the `Resource` in place of
// the originally provided values.
type ResourceTransformResult struct {
	// The new properties to use in place of the original `props`.
	Props map[string]any
	// The new resource options to use in place of the original `opts`.
	Opts ResourceOptions
}

// ResourceTransform is the callback signature for the `transforms` resource option.  A
// transform is passed the same set of inputs provided to the `Resource` constructor, and can
// optionally return back alternate values for the `props` and/or `opts` prior to the resource
// actually being created.  The effect will be as though those props and opts were passed in place
// of the original call to the `Resource` constructor.  If the transform returns nil,
// this indicates that the resource will not be transformed.
type ResourceTransform func(*ResourceTransformArgs) *ResourceTransformResult
