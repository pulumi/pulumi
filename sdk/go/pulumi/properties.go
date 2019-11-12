// Copyright 2016-2018, Pulumi Corporation.
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

// nolint: lll
package pulumi

import (
	"context"
	"reflect"
	"sync"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/go/pulumi/asset"
)

const (
	outputPending = iota
	outputResolved
	outputRejected
)

// Output helps encode the relationship between resources in a Pulumi application.  Specifically an output property
// holds onto a value and the resource it came from.  An output value can then be provided when constructing new
// resources, allowing that new resource to know both the value as well as the resource the value came from.  This
// allows for a precise "dependency graph" to be created, which properly tracks the relationship between resources.
type Output struct {
	s *outputState // protect against value aliasing.
}

// outputState is a heap-allocated block of state for each output property, in case of aliasing.
type outputState struct {
	mutex sync.Mutex
	cond  *sync.Cond

	state uint32 // one of output{Pending,Resolved,Rejected}

	value interface{} // the value of this output if it is resolved.
	err   error       // the error associated with this output if it is rejected.
	known bool        // true if this output's value is known.

	deps []Resource // the dependencies associated with this output property.
}

func (o *outputState) dependencies() []Resource {
	if o == nil {
		return nil
	}
	return o.deps
}

func (o *outputState) fulfill(value interface{}, known bool, err error) {
	if o == nil {
		return
	}

	o.mutex.Lock()
	defer func() {
		o.mutex.Unlock()
		o.cond.Broadcast()
	}()

	if o.state != outputPending {
		return
	}

	if err != nil {
		o.state, o.err, o.known = outputRejected, err, true
	} else {
		o.state, o.value, o.known = outputResolved, value, known
	}
}

func (o *outputState) resolve(value interface{}, known bool) {
	o.fulfill(value, known, nil)
}

func (o *outputState) reject(err error) {
	o.fulfill(nil, true, err)
}

func (o *outputState) await(ctx context.Context) (interface{}, bool, error) {
	for {
		if o == nil {
			// If the state is nil, treat its value as resolved and unknown.
			return nil, false, nil
		}

		o.mutex.Lock()
		for o.state == outputPending {
			if ctx.Err() != nil {
				return nil, true, ctx.Err()
			}
			o.cond.Wait()
		}
		o.mutex.Unlock()

		if !o.known || o.err != nil {
			return nil, o.known, o.err
		}

		ov, ok := isOutput(o.value)
		if !ok {
			return o.value, true, nil
		}
		o = ov.s
	}
}

func newOutput(deps ...Resource) Output {
	out := Output{
		s: &outputState{
			deps: deps,
		},
	}
	out.s.cond = sync.NewCond(&out.s.mutex)

	return out
}

var outputType = reflect.TypeOf(Output{})

func isOutput(v interface{}) (Output, bool) {
	if v != nil {
		rv := reflect.ValueOf(v)
		if rv.Type().ConvertibleTo(outputType) {
			return rv.Convert(outputType).Interface().(Output), true
		}
	}
	return Output{}, false
}

// NewOutput returns an output value that can be used to rendezvous with the production of a value or error.  The
// function returns the output itself, plus two functions: one for resolving a value, and another for rejecting with an
// error; exactly one function must be called. This acts like a promise.
func NewOutput() (Output, func(interface{}), func(error)) {
	out := newOutput()

	resolve := func(v interface{}) {
		out.s.resolve(v, true)
	}
	reject := func(err error) {
		out.s.reject(err)
	}

	return out, resolve, reject
}

// ApplyWithContext transforms the data of the output property using the applier func. The result remains an output
// property, and accumulates all implicated dependencies, so that resources can be properly tracked using a DAG.
// This function does not block awaiting the value; instead, it spawns a Goroutine that will await its availability.
func (out Output) Apply(applier func(v interface{}) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext transforms the data of the output property using the applier func. The result remains an output
// property, and accumulates all implicated dependencies, so that resources can be properly tracked using a DAG.
// This function does not block awaiting the value; instead, it spawns a Goroutine that will await its availability.
// The provided context can be used to reject the output as canceled.
func (out Output) ApplyWithContext(ctx context.Context,
	applier func(ctx context.Context, v interface{}) (interface{}, error)) Output {

	result := newOutput(out.s.deps...)
	go func() {
		v, known, err := out.s.await(ctx)
		if err != nil || !known {
			result.s.fulfill(nil, known, err)
			return
		}

		// If we have a known value, run the applier to transform it.
		u, err := applier(ctx, v)
		if err != nil {
			result.s.reject(err)
			return
		}

		// Fulfill the result.
		result.s.fulfill(u, true, nil)
	}()
	return result
}

// Outputs is a map of property name to value, one for each resource output property.
type Outputs map[string]Output

// ArchiveOutput is an Output that is typed to return archive values.
type ArchiveOutput Output

var archiveType = reflect.TypeOf((*asset.Archive)(nil)).Elem()

// Apply applies a transformation to the archive value when it is available.
func (out ArchiveOutput) Apply(applier func(asset.Archive) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v asset.Archive) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the archive value when it is available.
func (out ArchiveOutput) ApplyWithContext(ctx context.Context, applier func(context.Context, asset.Archive) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, convert(v, archiveType).(asset.Archive))
	})
}

// ArrayOutput is an Output that is typed to return arrays of values.
type ArrayOutput Output

var arrayType = reflect.TypeOf((*[]interface{})(nil)).Elem()

// Apply applies a transformation to the archive value when it is available.
func (out ArrayOutput) Apply(applier func([]interface{}) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v []interface{}) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the archive value when it is available.
func (out ArrayOutput) ApplyWithContext(ctx context.Context, applier func(context.Context, []interface{}) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, convert(v, arrayType).([]interface{}))
	})
}

// AssetOutput is an Output that is typed to return asset values.
type AssetOutput Output

var assetType = reflect.TypeOf((*asset.Asset)(nil)).Elem()

// Apply applies a transformation to the archive value when it is available.
func (out AssetOutput) Apply(applier func(asset.Asset) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v asset.Asset) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the archive value when it is available.
func (out AssetOutput) ApplyWithContext(ctx context.Context, applier func(context.Context, asset.Asset) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, convert(v, assetType).(asset.Asset))
	})
}

// BoolOutput is an Output that is typed to return bool values.
type BoolOutput Output

var boolType = reflect.TypeOf(false)

// Apply applies a transformation to the archive value when it is available.
func (out BoolOutput) Apply(applier func(bool) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v bool) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the archive value when it is available.
func (out BoolOutput) ApplyWithContext(ctx context.Context, applier func(context.Context, bool) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, convert(v, boolType).(bool))
	})
}

// Float32Output is an Output that is typed to return float32 values.
type Float32Output Output

var float32Type = reflect.TypeOf(float32(0))

// Apply applies a transformation to the archive value when it is available.
func (out Float32Output) Apply(applier func(float32) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v float32) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the archive value when it is available.
func (out Float32Output) ApplyWithContext(ctx context.Context, applier func(context.Context, float32) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, convert(v, float32Type).(float32))
	})
}

// Float64Output is an Output that is typed to return float64 values.
type Float64Output Output

var float64Type = reflect.TypeOf(float64(0))

// Apply applies a transformation to the archive value when it is available.
func (out Float64Output) Apply(applier func(float64) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v float64) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the archive value when it is available.
func (out Float64Output) ApplyWithContext(ctx context.Context, applier func(context.Context, float64) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, convert(v, float64Type).(float64))
	})
}

// IDOutput is an Output that is typed to return ID values.
type IDOutput Output

var stringType = reflect.TypeOf("")

func (out IDOutput) await(ctx context.Context) (ID, bool, error) {
	id, known, err := out.s.await(ctx)
	if !known || err != nil {
		return "", known, err
	}
	return ID(convert(id, stringType).(string)), true, nil
}

// Apply applies a transformation to the archive value when it is available.
func (out IDOutput) Apply(applier func(ID) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v ID) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the archive value when it is available.
func (out IDOutput) ApplyWithContext(ctx context.Context, applier func(context.Context, ID) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, ID(convert(v, stringType).(string)))
	})
}

// IntOutput is an Output that is typed to return int values.
type IntOutput Output

var intType = reflect.TypeOf(int(0))

// Apply applies a transformation to the archive value when it is available.
func (out IntOutput) Apply(applier func(int) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v int) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the archive value when it is available.
func (out IntOutput) ApplyWithContext(ctx context.Context, applier func(context.Context, int) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, convert(v, intType).(int))
	})
}

// Int8Output is an Output that is typed to return int8 values.
type Int8Output Output

var int8Type = reflect.TypeOf(int8(0))

// Apply applies a transformation to the archive value when it is available.
func (out Int8Output) Apply(applier func(int8) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v int8) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the archive value when it is available.
func (out Int8Output) ApplyWithContext(ctx context.Context, applier func(context.Context, int8) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, convert(v, int8Type).(int8))
	})
}

// Int16Output is an Output that is typed to return int16 values.
type Int16Output Output

var int16Type = reflect.TypeOf(int16(0))

// Apply applies a transformation to the archive value when it is available.
func (out Int16Output) Apply(applier func(int16) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v int16) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the archive value when it is available.
func (out Int16Output) ApplyWithContext(ctx context.Context, applier func(context.Context, int16) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, convert(v, int16Type).(int16))
	})
}

// Int32Output is an Output that is typed to return int32 values.
type Int32Output Output

var int32Type = reflect.TypeOf(int32(0))

// Apply applies a transformation to the archive value when it is available.
func (out Int32Output) Apply(applier func(int32) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v int32) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the archive value when it is available.
func (out Int32Output) ApplyWithContext(ctx context.Context, applier func(context.Context, int32) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, convert(v, int32Type).(int32))
	})
}

// Int64Output is an Output that is typed to return int64 values.
type Int64Output Output

var int64Type = reflect.TypeOf(int64(0))

// Apply applies a transformation to the archive value when it is available.
func (out Int64Output) Apply(applier func(int64) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v int64) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the archive value when it is available.
func (out Int64Output) ApplyWithContext(ctx context.Context, applier func(context.Context, int64) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, convert(v, int64Type).(int64))
	})
}

// MapOutput is an Output that is typed to return map values.
type MapOutput Output

var mapType = reflect.TypeOf(map[string]interface{}{})

// Apply applies a transformation to the number value when it is available.
func (out MapOutput) Apply(applier func(map[string]interface{}) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v map[string]interface{}) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the number value when it is available.
func (out MapOutput) ApplyWithContext(ctx context.Context, applier func(context.Context, map[string]interface{}) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, convert(v, mapType).(map[string]interface{}))
	})
}

// StringOutput is an Output that is typed to return number values.
type StringOutput Output

// Apply applies a transformation to the archive value when it is available.
func (out StringOutput) Apply(applier func(string) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v string) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the archive value when it is available.
func (out StringOutput) ApplyWithContext(ctx context.Context, applier func(context.Context, string) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, convert(v, stringType).(string))
	})
}

// UintOutput is an Output that is typed to return uint values.
type UintOutput Output

var uintType = reflect.TypeOf(uint(0))

// Apply applies a transformation to the archive value when it is available.
func (out UintOutput) Apply(applier func(uint) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v uint) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the archive value when it is available.
func (out UintOutput) ApplyWithContext(ctx context.Context, applier func(context.Context, uint) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, convert(v, uintType).(uint))
	})
}

// Uint8Output is an Output that is typed to return uint8 values.
type Uint8Output Output

var uint8Type = reflect.TypeOf(uint8(0))

// Apply applies a transformation to the archive value when it is available.
func (out Uint8Output) Apply(applier func(uint8) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v uint8) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the archive value when it is available.
func (out Uint8Output) ApplyWithContext(ctx context.Context, applier func(context.Context, uint8) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, convert(v, uint8Type).(uint8))
	})
}

// Uint16Output is an Output that is typed to return uint16 values.
type Uint16Output Output

var uint16Type = reflect.TypeOf(uint16(0))

// Apply applies a transformation to the archive value when it is available.
func (out Uint16Output) Apply(applier func(uint16) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v uint16) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the archive value when it is available.
func (out Uint16Output) ApplyWithContext(ctx context.Context, applier func(context.Context, uint16) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, convert(v, uint16Type).(uint16))
	})
}

// Uint32Output is an Output that is typed to return uint32 values.
type Uint32Output Output

var uint32Type = reflect.TypeOf(uint32(0))

// Apply applies a transformation to the archive value when it is available.
func (out Uint32Output) Apply(applier func(uint32) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v uint32) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the archive value when it is available.
func (out Uint32Output) ApplyWithContext(ctx context.Context, applier func(context.Context, uint32) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, convert(v, uint32Type).(uint32))
	})
}

// Uint64Output is an Output that is typed to return uint64 values.
type Uint64Output Output

var uint64Type = reflect.TypeOf(uint64(0))

// Apply applies a transformation to the archive value when it is available.
func (out Uint64Output) Apply(applier func(uint64) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v uint64) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the archive value when it is available.
func (out Uint64Output) ApplyWithContext(ctx context.Context, applier func(context.Context, uint64) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, convert(v, uint64Type).(uint64))
	})
}

// URNOutput is an Output that is typed to return URN values.
type URNOutput Output

func (out URNOutput) await(ctx context.Context) (URN, bool, error) {
	urn, known, err := out.s.await(ctx)
	if !known || err != nil {
		return "", known, err
	}
	return URN(convert(urn, stringType).(string)), true, nil
}

// Apply applies a transformation to the archive value when it is available.
func (out URNOutput) Apply(applier func(URN) (interface{}, error)) Output {
	return out.ApplyWithContext(context.Background(), func(_ context.Context, v URN) (interface{}, error) {
		return applier(v)
	})
}

// ApplyWithContext applies a transformation to the archive value when it is available.
func (out URNOutput) ApplyWithContext(ctx context.Context, applier func(context.Context, URN) (interface{}, error)) Output {
	return Output(out).ApplyWithContext(ctx, func(ctx context.Context, v interface{}) (interface{}, error) {
		return applier(ctx, URN(convert(v, stringType).(string)))
	})
}

func convert(v interface{}, to reflect.Type) interface{} {
	rv := reflect.ValueOf(v)
	if !rv.Type().ConvertibleTo(to) {
		panic(errors.Errorf("cannot convert output value of type %s to %s", rv.Type(), to))
	}
	return rv.Convert(to).Interface()
}
