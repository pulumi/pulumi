package pulumi

import "reflect"

type StackReference struct {
	CustomResourceState

	Name    StringOutput `pulumi:"name"`
	Outputs MapOutput    `pulumi:"outputs"`
}

func (s StackReference) GetOutput(name StringInput) AnyOutput {
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
// Name is in the form "Org/Program/Stack"
type StackReferenceArgs struct {
	Name StringInput
}

func (StackReferenceArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*stackReferenceArgs)(nil)).Elem()
}

// NewStackReference creates a stack reference that makes available outputs from the specified stack
func NewStackReference(ctx *Context, name string, args *StackReferenceArgs,
	opts ...ResourceOption) (*StackReference, error) {

	stack := StringInput(String(name))
	if args != nil {
		stack = args.Name
	}

	id := stack.ToStringOutput().ApplyT(func(s string) ID { return ID(s) }).(IDOutput)

	var ref StackReference
	if err := ctx.ReadResource("pulumi:pulumi:StackReference", name, id, args, &ref, opts...); err != nil {
		return nil, err
	}

	return &ref, nil
}
