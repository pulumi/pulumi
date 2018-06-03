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

// Input is an input property for a resource.  It is a discriminated union of either a value or another resource's
// output value, which will make the receiving resource dependent on the resource from which the output came.
type Input interface{}

// Inputs is a map of property name to value, one for each resource input property.  Each value can be a prompt,
// JSON serializable primitive -- bool, string, int, array, or map -- or it can be an *Output, in which case the
// input property will carry dependency information from the resource to which the output belongs.
type Inputs map[string]interface{}

// Output helps encode the relationship between resources in a Pulumi application.  Specifically an output property
// holds onto a value and the resource it came from.  An output value can then be provided when constructing new
// resources, allowing that new resource to know both the value as well as the resource the value came from.  This
// allows for a precise "dependency graph" to be created, which properly tracks the relationship between resources.
type Output struct {
	sync chan *valueOrError // the channel for outputs whose values are not yet known.
	voe  *valueOrError      // the value or error, after the channel has been rendezvoused with.
	deps []Resource         // the dependencies associated with this output property.
}

// valueOrError is a discriminated union between a value (possibly nil) or an error.
type valueOrError struct {
	value interface{} // a value, if the output resolved to a value.
	err   error       // an error, if the producer yielded an error instead of a value.
}

// NewOutput returns an output value that can be used to rendezvous with the production of a value or error.  The
// function returns the output itself, plus two functions: one for resolving a value, and another for rejecting with an
// error; exactly one function must be called.  This acts like a promise.
func NewOutput(deps []Resource) (*Output, func(interface{}), func(error)) {
	out := &Output{
		sync: make(chan *valueOrError, 1),
		deps: deps,
	}
	resolve := func(v interface{}) {
		out.sync <- &valueOrError{value: v}
	}
	reject := func(err error) {
		out.sync <- &valueOrError{err: err}
	}
	return out, resolve, reject
}

// Apply transforms the data of the output property using the applier func.  The result remains an output property,
// and accumulates all implicated dependencies, so that resources can be properly tracked using a DAG.  This function
// does not block awaiting the value; instead, it spawns a Goroutine that will await its availability.
func (out *Output) Apply(applier func(v interface{}) (interface{}, error)) *Output {
	result, resolve, reject := NewOutput(out.Deps())
	go func() {
		for {
			v, err := out.Value()
			if err != nil {
				reject(err)
				break
			} else {
				// If we have a value, run the applier to transform it.
				u, err := applier(v)
				if err != nil {
					reject(err)
					break
				} else {
					// Now that we've transformed the value, it's possible we have another output.  If so, pluck it
					// out and go around to await it until we hit a real value.  Note that we are not capturing the
					// resources of this inner output, intentionally, as the output returned should be related to
					// this output already.
					if newout, ok := v.(*Output); ok {
						out = newout
					} else {
						resolve(u)
						break
					}
				}
			}
		}
	}()
	return result
}

// Deps returns the dependencies for this output property.
func (out *Output) Deps() []Resource {
	return out.deps
}

// Value retrieves the underlying value for this output property.
func (out *Output) Value() (interface{}, error) {
	// If neither error nor value are available, first await the channel.  Only one Goroutine will make it through this
	// and is responsible for closing the channel, to signal to other awaiters that it's safe to read the values.
	if out.voe == nil {
		if voe := <-out.sync; voe != nil {
			out.voe = voe
			close(out.sync)
		}
	}
	return out.voe.value, out.voe.err
}

// String retrives the underlying value for this output property as a string.
func (out *Output) String() (string, error) {
	v, err := out.Value()
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

// ID retrives the underlying value for this output property as an ID.
func (out *Output) ID() (ID, error) {
	v, err := out.Value()
	if err != nil {
		return "", err
	}
	return v.(ID), nil
}

// URN retrives the underlying value for this output property as a URN.
func (out *Output) URN() (URN, error) {
	v, err := out.Value()
	if err != nil {
		return "", err
	}
	return v.(URN), nil
}

// Outputs is a map of property name to value, one for each resource output property.
type Outputs map[string]*Output
