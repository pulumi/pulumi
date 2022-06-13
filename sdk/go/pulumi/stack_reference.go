package pulumi

import (
	"fmt"
	"reflect"
)

// StackReference manages a reference to a Pulumi stack.
type StackReference struct {
	CustomResourceState

	// Name is in the form "Org/Program/Stack"
	Name StringOutput `pulumi:"name"`
	// Outputs resolves with exports from the named stack
	Outputs MapOutput `pulumi:"outputs"`

	// ctx is a reference to the context used to create the stack reference. It must be
	// valid and non-nil to call `GetOutput`.
	ctx *Context
}

// GetOutput returns a stack output keyed by the given name as an AnyOutput
func (s *StackReference) GetOutput(name StringInput) AnyOutput {
	return All(name, s.Outputs).
		ApplyT(func(args []interface{}) interface{} {
			n, outs := args[0].(string), args[1].(map[string]interface{})
			v, ok := outs[n]
			if ok {
				return v
			}
			if s.ctx.DryRun() {
				// It is a dry run, so it is safe to return an unknown output.
				return UnsafeUnknownOutput([]Resource{s})
			}

			// We need to return a knowable output to avoid hanging the program, so we
			// warn and return nil.
			s.ctx.Log.Warn(fmt.Sprintf("stack referenced missing output '%s'", n), &LogArgs{Resource: s})
			return nil
		}).(AnyOutput)
}

// GetStringOutput returns a stack output keyed by the given name as an StringOutput
func (s *StackReference) GetStringOutput(name StringInput) StringOutput {
	return s.GetOutput(name).ApplyT(func(out interface{}) string {
		var res string
		if out != nil {
			res = out.(string)
		}
		return res
	}).(StringOutput)
}

// GetIDOutput returns a stack output keyed by the given name as an IDOutput
func (s *StackReference) GetIDOutput(name StringInput) IDOutput {
	return s.GetStringOutput(name).ApplyT(func(out string) ID {
		return ID(out)
	}).(IDOutput)
}

// GetFloat64Output returns a stack output keyed by the given name as an Float64Output
func (s *StackReference) GetFloat64Output(name StringInput) Float64Output {
	return s.GetOutput(name).ApplyT(func(out interface{}) (float64, error) {
		if numf, ok := out.(float64); ok {
			return numf, nil
		}
		return 0.0, fmt.Errorf("failed to convert %T to float64", out)
	}).(Float64Output)
}

// GetIntOutput returns a stack output keyed by the given name as an IntOutput
func (s *StackReference) GetIntOutput(name StringInput) IntOutput {
	return s.GetOutput(name).ApplyT(func(out interface{}) (int, error) {
		numf, ok := out.(float64)
		if !ok {
			return 0, fmt.Errorf("failed to convert %T to int", out)
		}
		return int(numf), nil
	}).(IntOutput)
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

	ref := StackReference{ctx: ctx}
	if err := ctx.ReadResource("pulumi:pulumi:StackReference", name, id, args, &ref, opts...); err != nil {
		return nil, err
	}

	return &ref, nil
}
