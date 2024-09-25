// Copyright 2020-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

// StackReferenceOutputDetails holds a stack output value.
// At most one of the Value and SecretValue fields will be set.
//
// You can build a StackReferenceOutputDetails with
// the [StackReference.GetOutputDetails] method.
type StackReferenceOutputDetails struct {
	// Output value returned by the StackReference.
	// This field is nil if the value is a secret
	// or it does not exist.
	Value interface{}

	// Secret output value returned by the StackReference.
	// This field is nil if the value is not a secret
	// or it does not exist.
	SecretValue interface{}
}

// GetOutputDetails retrieves a stack output keyed by the given name
// and returns the value inside a [StackReferenceOutputDetails] object.
//
// It sets the Value or the SecretValue fields of StackReferenceOutputDetails
// depending on whether the stack output is a secret.
// If the given name is not present in the StackReference,
// both fields are nil.
func (s *StackReference) GetOutputDetails(name string) (*StackReferenceOutputDetails, error) {
	value, _, secret, _, err := awaitWithContext(s.ctx.Context(), s.GetOutput(String(name)))
	if err != nil {
		return nil, err
	}

	var d StackReferenceOutputDetails
	if secret {
		d.SecretValue = value
	} else {
		d.Value = value
	}
	return &d, nil
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
	opts ...ResourceOption,
) (*StackReference, error) {
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
