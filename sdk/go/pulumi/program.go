package pulumi

import (
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

type Program struct {
	CustomResourceState

	// Name is in the form "Org/Program/Stack"
	Name StringOutput `pulumi:"name"`
	// Outputs resolves with exports from the named stack
	Outputs MapOutput `pulumi:"outputs"`

	ctx *Context
}

// GetOutput returns a stack output keyed by the given name as an AnyOutput
// If the given name is not present in the StackReference, Output<nil> is returned.
func (p *Program) GetOutput(name StringInput) AnyOutput {
	return All(name, p.rawOutputs).
		ApplyT(func(args []interface{}) (interface{}, error) {
			n, stack := args[0].(string), args[1].(resource.PropertyMap)
			if !stack["outputs"].IsObject() {
				return Any(nil), fmt.Errorf("failed to convert %T to object", stack)
			}
			outs := stack["outputs"].ObjectValue()
			v, ok := outs[resource.PropertyKey(n)]
			if !ok {
				if p.ctx.DryRun() {
					// It is a dry run, so it is safe to return an unknown output.
					return UnsafeUnknownOutput([]Resource{p}), nil
				}

				// We don't return an error to remain consistent with other SDKs regarding missing keys.
				return nil, nil
			}

			ret, secret, _ := unmarshalPropertyValue(p.ctx, v)

			if secret {
				ret = ToSecret(ret)
			}
			return ret, nil
		}).(AnyOutput)
}

type programArgs struct {
	Source              string                 `pulumi:"source"`
	Inputs              map[string]interface{} `pulumi:"inputs"`
	PrefixResourceNames bool                   `pulumi:"prefixResourceNames"`
}

type ProgramArgs struct {
	// Source is the source of the program to execute.
	Source StringInput

	// Inputs is the input to configure the stack
	Inputs MapInput

	// PrefixResourceNames is a boolean that indicates whether to prefix the names of resources with the name of the program.
	PrefixResourceNames BoolInput
}

func (ProgramArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*programArgs)(nil)).Elem()
}

func NewProgram(ctx *Context, name string, args *ProgramArgs, opts ...ResourceOption) (*Program, error) {
	if args == nil {
		args = &ProgramArgs{}
	}

	id := StringInput(String(name)).ToStringOutput().ApplyT(func(s string) ID { return ID(s) }).(IDOutput)

	ref := Program{ctx: ctx}
	if err := ctx.ReadResource("pulumi:pulumi:Stack", name, id, args, &ref, opts...); err != nil {
		return nil, err
	}
	return &ref, nil
}
