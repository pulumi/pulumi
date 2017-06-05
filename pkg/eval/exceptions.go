// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package eval

import (
	"fmt"
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// NewException produces a new exception in the evaluator using the current location and stack frames.
func (e *evaluator) NewException(node diag.Diagable, msg string, args ...interface{}) *rt.Unwind {
	contract.Require(node != nil, "node")
	thrown := e.alloc.NewString(node, fmt.Sprintf(msg, args...))
	return rt.NewThrowUnwind(thrown, node, e.stack)
}

func (e *evaluator) NewNullObjectException(node diag.Diagable) *rt.Unwind {
	return e.NewException(node, "Target object is null")
}

func (e *evaluator) NewNameNotDefinedException(node diag.Diagable, name tokens.Name) *rt.Unwind {
	return e.NewException(node, "Name '%v' is not defined", name)
}

func (e *evaluator) NewNegativeArrayLengthException(node diag.Diagable) *rt.Unwind {
	return e.NewException(node, "Invalid array size (must be >= 0)")
}

func (e *evaluator) NewIncorrectArrayElementCountException(node diag.Diagable, expect int, got int) *rt.Unwind {
	return e.NewException(node, "Invalid number of array elements; expected <=%v, got %v", expect, got)
}

func (e *evaluator) NewInvalidCastException(node diag.Diagable, from symbols.Type, to symbols.Type) *rt.Unwind {
	return e.NewException(node, "Cannot cast object of type '%v' to '%v'", from, to)
}

func (e *evaluator) NewIllegalInvokeTargetException(node diag.Diagable, target symbols.Type) *rt.Unwind {
	return e.NewException(node, "Expected a function as the target of an invoke; got '%v'", target)
}

func (e *evaluator) NewUnexpectedComputedValueException(node diag.Diagable, obj *rt.Object) *rt.Unwind {
	return e.NewException(node, "Unexpected computed value '%v' encountered; a concrete value is required", obj.Type())
}
