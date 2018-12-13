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

package pulumi

import (
	"reflect"

	"github.com/spf13/cast"

	"github.com/pulumi/pulumi/sdk/go/pulumi/asset"
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
	sync chan *valueOrError // the channel for outputs whose values are not yet known.
	voe  *valueOrError      // the value or error, after the channel has been rendezvoused with.
}

// valueOrError is a discriminated union between a value (possibly nil) or an error.
type valueOrError struct {
	value interface{} // a value, if the output resolved to a value.
	err   error       // an error, if the producer yielded an error instead of a value.
	known bool        // true if this value is known, versus just being a placeholder during previews.
	deps  []Resource  // the dependencies associated with this value.
}

// NewOutput returns an output value that can be used to rendezvous with the production of a value or error.  The
// function returns the output itself, plus two functions: one for resolving a value, and another for rejecting with an
// error; exactly one function must be called.  This acts like a promise.
func NewOutput(deps []Resource) (*Output, func(interface{}, bool), func(error)) {
	out := &Output{
		s: &outputState{
			sync: make(chan *valueOrError, 1),
		},
	}

	resolve := func(v interface{}, known bool) {
		out.resolve(v, known, deps)
	}

	return out, resolve, out.reject
}

// resolve will resolve the output.  It is not exported, because we want to control the capabilities tightly, such
// that anybody who happens to have an Output is not allowed to resolve it; only those who created it can.
func (out *Output) resolve(v interface{}, known bool, deps []Resource) {
	// If v is another output, chain this rather than resolving to an output directly.
	if other, isOut := v.(*Output); isOut {
		go func() {
			real, otherKnown, otherDeps, err := other.value()
			if err != nil {
				out.reject(err)
			} else {
				out.resolve(real, known && otherKnown, append(deps, otherDeps...))
			}
		}()
	} else {
		out.s.sync <- &valueOrError{value: v, known: known, deps: deps}
	}
}

// reject will reject the output.  It is not exported, because we want to control the capabilities tightly, such
// that anybody who happens to have an Output is not allowed to reject it; only those who created it can.
func (out *Output) reject(err error) {
	out.s.sync <- &valueOrError{err: err}
}

// Apply transforms the data of the output property using the applier func.  The result remains an output property,
// and accumulates all implicated dependencies, so that resources can be properly tracked using a DAG.  This function
// does not block awaiting the value; instead, it spawns a Goroutine that will await its availability.
func (out *Output) Apply(applier func(v interface{}) (interface{}, error)) *Output {
	result, _, _ := NewOutput(nil)
	go func() {
		v, known, deps, err := out.value()
		if err != nil {
			result.reject(err)
			return
		}
		// If the value isn't known, skip the apply function.
		if !known {
			result.resolve(nil, false, deps)
			return
		}

		// If we have a known value, run the applier to transform it.
		u, err := applier(v)
		if err != nil {
			result.reject(err)
			return
		}

		result.resolve(u, true, deps)
	}()
	return result
}

func (out *Output) value() (interface{}, bool, []Resource, error) {
	// If neither error nor value are available, first await the channel.  Only one Goroutine will make it through this
	// and is responsible for closing the channel, to signal to other awaiters that it's safe to read the values.
	if out.s.voe == nil {
		if voe := <-out.s.sync; voe != nil {
			out.s.voe = voe   // first time through, publish the value.
			close(out.s.sync) // and close the channel to signal to others that the memozied value is available.
		}
	}
	return out.s.voe.value, out.s.voe.known, out.s.voe.deps, out.s.voe.err
}

// Value retrieves the underlying value for this output property.
func (out *Output) Value() (interface{}, bool, error) {
	v, known, _, err := out.value()
	return v, known, err
}

// Archive retrives the underlying value for this output property as an archive.
func (out *Output) Archive() (asset.Archive, bool, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return nil, known, err
	}
	return v.(asset.Archive), true, nil
}

// Array retrives the underlying value for this output property as an array.
func (out *Output) Array() ([]interface{}, bool, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return nil, known, err
	}
	return cast.ToSlice(v), true, nil
}

// Asset retrives the underlying value for this output property as an asset.
func (out *Output) Asset() (asset.Asset, bool, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return nil, known, err
	}
	return v.(asset.Asset), true, nil
}

// Bool retrives the underlying value for this output property as a bool.
func (out *Output) Bool() (bool, bool, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return false, known, err
	}
	return cast.ToBool(v), true, nil
}

// Map retrives the underlying value for this output property as a map.
func (out *Output) Map() (map[string]interface{}, bool, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return nil, known, err
	}
	return cast.ToStringMap(v), true, nil
}

// Float32 retrives the underlying value for this output property as a float32.
func (out *Output) Float32() (float32, bool, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return 0, known, err
	}
	return cast.ToFloat32(v), true, nil
}

// Float64 retrives the underlying value for this output property as a float64.
func (out *Output) Float64() (float64, bool, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return 0, known, err
	}
	return cast.ToFloat64(v), true, nil
}

// ID retrives the underlying value for this output property as an ID.
func (out *Output) ID() (ID, bool, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return "", known, err
	}
	return ID(toString(v)), true, nil
}

// Int retrives the underlying value for this output property as a int.
func (out *Output) Int() (int, bool, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return 0, known, err
	}
	return cast.ToInt(v), true, nil
}

// Int8 retrives the underlying value for this output property as a int8.
func (out *Output) Int8() (int8, bool, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return 0, known, err
	}
	return cast.ToInt8(v), true, nil
}

// Int16 retrives the underlying value for this output property as a int16.
func (out *Output) Int16() (int16, bool, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return 0, known, err
	}
	return cast.ToInt16(v), true, nil
}

// Int32 retrives the underlying value for this output property as a int32.
func (out *Output) Int32() (int32, bool, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return 0, known, err
	}
	return cast.ToInt32(v), true, nil
}

// Int64 retrives the underlying value for this output property as a int64.
func (out *Output) Int64() (int64, bool, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return 0, known, err
	}
	return cast.ToInt64(v), true, nil
}

// String retrives the underlying value for this output property as a string.
func (out *Output) String() (string, bool, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return "", known, err
	}
	return toString(v), true, nil
}

// Uint retrives the underlying value for this output property as a uint.
func (out *Output) Uint() (uint, bool, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return 0, known, err
	}
	return cast.ToUint(v), true, nil
}

// Uint8 retrives the underlying value for this output property as a uint8.
func (out *Output) Uint8() (uint8, bool, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return 0, known, err
	}
	return cast.ToUint8(v), true, nil
}

// Uint16 retrives the underlying value for this output property as a uint16.
func (out *Output) Uint16() (uint16, bool, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return 0, known, err
	}
	return cast.ToUint16(v), true, nil
}

// Uint32 retrives the underlying value for this output property as a uint32.
func (out *Output) Uint32() (uint32, bool, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return 0, known, err
	}
	return cast.ToUint32(v), true, nil
}

// Uint64 retrives the underlying value for this output property as a uint64.
func (out *Output) Uint64() (uint64, bool, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return 0, known, err
	}
	return cast.ToUint64(v), true, nil
}

// URN retrives the underlying value for this output property as a URN.
func (out *Output) URN() (URN, error) {
	v, known, err := out.Value()
	if err != nil || !known {
		return "", err
	}
	return URN(toString(v)), nil
}

// Outputs is a map of property name to value, one for each resource output property.
type Outputs map[string]*Output

// ArchiveOutput is an Output that is typed to return archive values.
type ArchiveOutput Output

// Value returns the underlying archive value.
func (out *ArchiveOutput) Value() (asset.Archive, bool, error) {
	return (*Output)(out).Archive()
}

// Apply applies a transformation to the archive value when it is available.
func (out *ArchiveOutput) Apply(applier func(asset.Archive) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(v.(asset.Archive))
	})
}

// ArrayOutput is an Output that is typed to return arrays of values.
type ArrayOutput Output

// Value returns the underlying array value.
func (out *ArrayOutput) Value() ([]interface{}, bool, error) {
	return (*Output)(out).Array()
}

// Apply applies a transformation to the array value when it is available.
func (out *ArrayOutput) Apply(applier func([]interface{}) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(cast.ToSlice(v))
	})
}

// AssetOutput is an Output that is typed to return asset values.
type AssetOutput Output

// Value returns the underlying asset value.
func (out *AssetOutput) Value() (asset.Asset, bool, error) {
	return (*Output)(out).Asset()
}

// Apply applies a transformation to the asset value when it is available.
func (out *AssetOutput) Apply(applier func(asset.Asset) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(v.(asset.Asset))
	})
}

// BoolOutput is an Output that is typed to return bool values.
type BoolOutput Output

// Value returns the underlying bool value.
func (out *BoolOutput) Value() (bool, bool, error) {
	return (*Output)(out).Bool()
}

// Apply applies a transformation to the bool value when it is available.
func (out *BoolOutput) Apply(applier func(bool) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(v.(bool))
	})
}

// Float32Output is an Output that is typed to return float32 values.
type Float32Output Output

// Value returns the underlying number value.
func (out *Float32Output) Value() (float32, bool, error) {
	return (*Output)(out).Float32()
}

// Apply applies a transformation to the float32 value when it is available.
func (out *Float32Output) Apply(applier func(float32) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(cast.ToFloat32(v))
	})
}

// Float64Output is an Output that is typed to return float64 values.
type Float64Output Output

// Value returns the underlying number value.
func (out *Float64Output) Value() (float64, bool, error) {
	return (*Output)(out).Float64()
}

// Apply applies a transformation to the float64 value when it is available.
func (out *Float64Output) Apply(applier func(float64) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(cast.ToFloat64(v))
	})
}

// IDOutput is an Output that is typed to return ID values.
type IDOutput Output

// Value returns the underlying number value.
func (out *IDOutput) Value() (ID, bool, error) {
	return (*Output)(out).ID()
}

// Apply applies a transformation to the ID value when it is available.
func (out *IDOutput) Apply(applier func(ID) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(ID(toString(v)))
	})
}

// IntOutput is an Output that is typed to return int values.
type IntOutput Output

// Value returns the underlying number value.
func (out *IntOutput) Value() (int, bool, error) {
	return (*Output)(out).Int()
}

// Apply applies a transformation to the int value when it is available.
func (out *IntOutput) Apply(applier func(int) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(cast.ToInt(v))
	})
}

// Int8Output is an Output that is typed to return int8 values.
type Int8Output Output

// Value returns the underlying number value.
func (out *Int8Output) Value() (int8, bool, error) {
	return (*Output)(out).Int8()
}

// Apply applies a transformation to the int8 value when it is available.
func (out *Int8Output) Apply(applier func(int8) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(cast.ToInt8(v))
	})
}

// Int16Output is an Output that is typed to return int16 values.
type Int16Output Output

// Value returns the underlying number value.
func (out *Int16Output) Value() (int16, bool, error) {
	return (*Output)(out).Int16()
}

// Apply applies a transformation to the int16 value when it is available.
func (out *Int16Output) Apply(applier func(int16) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(cast.ToInt16(v))
	})
}

// Int32Output is an Output that is typed to return int32 values.
type Int32Output Output

// Value returns the underlying number value.
func (out *Int32Output) Value() (int32, bool, error) {
	return (*Output)(out).Int32()
}

// Apply applies a transformation to the int32 value when it is available.
func (out *Int32Output) Apply(applier func(int32) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(cast.ToInt32(v))
	})
}

// Int64Output is an Output that is typed to return int64 values.
type Int64Output Output

// Value returns the underlying number value.
func (out *Int64Output) Value() (int64, bool, error) { return (*Output)(out).Int64() }

// Apply applies a transformation to the int64 value when it is available.
func (out *Int64Output) Apply(applier func(int64) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(cast.ToInt64(v))
	})
}

// MapOutput is an Output that is typed to return map values.
type MapOutput Output

// Value returns the underlying number value.
func (out *MapOutput) Value() (map[string]interface{}, bool, error) {
	return (*Output)(out).Map()
}

// Apply applies a transformation to the number value when it is available.
func (out *MapOutput) Apply(applier func(map[string]interface{}) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(cast.ToStringMap(v))
	})
}

// StringOutput is an Output that is typed to return number values.
type StringOutput Output

// Value returns the underlying number value.
func (out *StringOutput) Value() (string, bool, error) {
	return (*Output)(out).String()
}

// Apply applies a transformation to the number value when it is available.
func (out *StringOutput) Apply(applier func(string) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(toString(v))
	})
}

// UintOutput is an Output that is typed to return uint values.
type UintOutput Output

// Value returns the underlying number value.
func (out *UintOutput) Value() (uint, bool, error) {
	return (*Output)(out).Uint()
}

// Apply applies a transformation to the uint value when it is available.
func (out *UintOutput) Apply(applier func(uint) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(cast.ToUint(v))
	})
}

// Uint8Output is an Output that is typed to return uint8 values.
type Uint8Output Output

// Value returns the underlying number value.
func (out *Uint8Output) Value() (uint8, bool, error) {
	return (*Output)(out).Uint8()
}

// Apply applies a transformation to the uint8 value when it is available.
func (out *Uint8Output) Apply(applier func(uint8) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(cast.ToUint8(v))
	})
}

// Uint16Output is an Output that is typed to return uint16 values.
type Uint16Output Output

// Value returns the underlying number value.
func (out *Uint16Output) Value() (uint16, bool, error) {
	return (*Output)(out).Uint16()
}

// Apply applies a transformation to the uint16 value when it is available.
func (out *Uint16Output) Apply(applier func(uint16) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(cast.ToUint16(v))
	})
}

// Uint32Output is an Output that is typed to return uint32 values.
type Uint32Output Output

// Value returns the underlying number value.
func (out *Uint32Output) Value() (uint32, bool, error) {
	return (*Output)(out).Uint32()
}

// Apply applies a transformation to the uint32 value when it is available.
func (out *Uint32Output) Apply(applier func(uint32) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(cast.ToUint32(v))
	})
}

// Uint64Output is an Output that is typed to return uint64 values.
type Uint64Output Output

// Value returns the underlying number value.
func (out *Uint64Output) Value() (uint64, bool, error) {
	return (*Output)(out).Uint64()
}

// Apply applies a transformation to the uint64 value when it is available.
func (out *Uint64Output) Apply(applier func(uint64) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(cast.ToUint64(v))
	})
}

// URNOutput is an Output that is typed to return URN values.
type URNOutput Output

// Value returns the underlying number value.
func (out *URNOutput) Value() (URN, error) {
	return (*Output)(out).URN()
}

// Apply applies a transformation to the URN value when it is available.
func (out *URNOutput) Apply(applier func(URN) (interface{}, error)) *Output {
	return (*Output)(out).Apply(func(v interface{}) (interface{}, error) {
		return applier(URN(toString(v)))
	})
}

// toString attempts to convert v to a string.
func toString(v interface{}) string {
	if s := cast.ToString(v); s != "" {
		return s
	}

	// See if this can convert through reflection (e.g., for type aliases).
	st := reflect.TypeOf("")
	sv := reflect.ValueOf(v)
	if sv.Type().ConvertibleTo(st) {
		return sv.Convert(st).Interface().(string)
	}

	return ""
}
