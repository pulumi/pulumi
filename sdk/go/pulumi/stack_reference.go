package pulumi

import "reflect"

// StackReference manages a reference to a Pulumi stack.
type StackReference struct {
	CustomResourceState

	// Name is in the form "Org/Program/Stack"
	Name StringOutput `pulumi:"name"`
	// Outputs resolves with exports from the named stack
	Outputs MapOutput `pulumi:"outputs"`
}

func (s *StackReference) GetOutput(name StringInput) AnyOutput {
	return All(name, s.Outputs).
		ApplyT(func(args []interface{}) interface{} {
			n, outs := args[0].(string), args[1].(map[string]interface{})
			return outs[n]
		}).(AnyOutput)
}

type stackReferenceArgs struct {
	Name string `pulumi:"name"`
}

// StackReferenceArgs is the input to NewStackReference that allows specifying a stack name
type StackReferenceArgs struct {
	// Name is in the form "Org/Program/Stack"
	Name StringInput
}

func (StackReferenceArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*stackReferenceArgs)(nil)).Elem()
}

// NewStackReference creates a stack reference that makes available outputs from the specified stack
func NewStackReference(ctx *Context, name string, args *StackReferenceArgs,
	opts ...ResourceOption) (*StackReference, error) {

	if args == nil {
		args = &StackReferenceArgs{}
	}
	if args.Name == nil {
		args.Name = StringInput(String(name))
	}

	id := args.Name.ToStringOutput().ApplyT(func(s string) ID { return ID(s) }).(IDOutput)

	var ref StackReference
	if err := ctx.ReadResource("pulumi:pulumi:StackReference", name, id, args, &ref, opts...); err != nil {
		return nil, err
	}

	return &ref, nil
}
