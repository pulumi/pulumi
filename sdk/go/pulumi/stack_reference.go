package pulumi

import (
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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
// If the given name is not present in the StackReference, Output<nil> is returned.
func (s *StackReference) GetOutput(name StringInput) AnyOutput {
	return All(name, s.rawOutputs).
		ApplyT(func(args []interface{}) (interface{}, error) {
			n, stack := args[0].(string), args[1].(resource.PropertyMap)
			if !stack["outputs"].IsObject() {
				return Any(nil), fmt.Errorf("failed to convert %T to object", stack)
			}
			outs := stack["outputs"].ObjectValue()
			v, ok := outs[resource.PropertyKey(n)]
			if !ok {
				if s.ctx.DryRun() {
					// It is a dry run, so it is safe to return an unknown output.
					return UnsafeUnknownOutput([]Resource{s}), nil
				}

				// We don't return an error to remain consistent with other SDKs regarding missing keys.
				return nil, nil
			}

			ret, secret, _ := unmarshalPropertyValue(s.ctx, v)

			if secret {
				ret = ToSecret(ret)
			}
			return ret, nil
		}).(AnyOutput)
}

// GetStringOutput returns a stack output keyed by the given name as an StringOutput
func (s *StackReference) GetStringOutput(name StringInput) StringOutput {
	return All(name, s.GetOutput(name)).ApplyT(func(args []interface{}) (string, error) {
		name, out := args[0].(string), args[1]
		if out == nil {
			return "", fmt.Errorf(
				"stack reference output %q does not exist on stack %q",
				name,
				s.name)
		}
		str, ok := out.(string)
		if !ok {
			return "", fmt.Errorf(
				"getting stack reference output %q on stack %q, failed to convert %T to string",
				name,
				s.name,
				out)
		}
		return str, nil
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
	return All(name, s.GetOutput(name)).ApplyT(func(args []interface{}) (float64, error) {
		name, out := args[0].(string), args[1]
		if out == nil {
			return 0.0, fmt.Errorf(
				"stack reference output %q does not exist on stack %q",
				name,
				s.name)
		}
		numf, ok := out.(float64)
		if !ok {
			return 0.0, fmt.Errorf(
				"getting stack reference output %q on stack %q, failed to convert %T to float64",
				name,
				s.name,
				out)
		}
		return numf, nil
	}).(Float64Output)
}

// GetIntOutput returns a stack output keyed by the given name as an IntOutput
func (s *StackReference) GetIntOutput(name StringInput) IntOutput {
	return All(name, s.GetOutput(name)).ApplyT(func(args []interface{}) (int, error) {
		name, out := args[0].(string), args[1]
		if out == nil {
			return 0, fmt.Errorf(
				"stack reference output %q does not exist on stack %q",
				name,
				s.name)
		}
		numf, ok := out.(float64)
		if !ok {
			return 0, fmt.Errorf(
				"getting stack reference output %q on stack %q, failed to convert %T to int",
				name,
				s.name,
				out)
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
