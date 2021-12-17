package pulumi

import (
	"fmt"
	"reflect"
)

// StackReference manages a reference to a Pulumi stack.
type StackReference struct {
	CustomResourceState

	// Name is in the form "Org/Program/Stack"
	Name Output[string] `pulumi:"name"`
	// Outputs resolves with exports from the named stack
	Outputs Output[map[string]interface{}] `pulumi:"outputs"`
}

// GetOutput returns a stack output keyed by the given name as an AnyOutput
func (s *StackReference) GetOutput(name Output[string]) AnyOutput {
	return All(name, s.Outputs).
		ApplyT(func(arr interface{}) interface{} {
			args := arr.([]interface{})
			n, outs := args[0].(string), args[1].(map[string]interface{})
			return outs[n]
		})
}

// GetStringOutput returns a stack output keyed by the given name as an StringOutput
func (s *StackReference) GetStringOutput(name Output[string]) Output[string] {
	return Apply(Weak(s.GetOutput(name)), func(out interface{}) string {
		var res string
		if out != nil {
			res = out.(string)
		}
		return res
	})
}

// GetIDOutput returns a stack output keyed by the given name as an IDOutput
func (s *StackReference) GetIDOutput(name Output[string]) Output[ID] {
	return Apply(s.GetStringOutput(name), func(out string) ID {
		return ID(out)
	})
}

// GetFloat64Output returns a stack output keyed by the given name as an Float64Output
func (s *StackReference) GetFloat64Output(name Output[string]) Output[float64] {
	return ApplyErr(Weak(s.GetOutput(name)), func(out interface{}) (float64, error) {
		if numf, ok := out.(float64); ok {
			return numf, nil
		}
		return 0.0, fmt.Errorf("failed to convert %T to float64", out)
	})
}

// GetIntOutput returns a stack output keyed by the given name as an IntOutput
func (s *StackReference) GetIntOutput(name Output[string]) Output[int] {
	return ApplyErr(Weak(s.GetOutput(name)), func(out interface{}) (int, error) {
		numf, ok := out.(float64)
		if !ok {
			return 0, fmt.Errorf("failed to convert %T to int", out)
		}
		return int(numf), nil
	})
}

type stackReferenceArgs struct {
	Name string `pulumi:"name"`
}

// StackReferenceArgs is the input to NewStackReference that allows specifying a stack name
type StackReferenceArgs struct {
	// Name is in the form "Org/Program/Stack"
	Name *Output[string]
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
		nout := ToOutput(name)
		args.Name = &nout
	}

	argsMap := Map{
		"Name": *args.Name,
	}

	var ref StackReference
	id := Apply(*args.Name, func(s string) ID { return ID(s) })
	if err := ctx.ReadResource("pulumi:pulumi:StackReference", name, id, argsMap, &ref, opts...); err != nil {
		return nil, err
	}

	return &ref, nil
}
