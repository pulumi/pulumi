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

package types

import (
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/tokens"
)

// Conversion represents the kind of conversion required to convert from a value of one type to another.
type Conversion int

const (
	NoConversion          Conversion = iota // there is no known conversion (though a cast could work)
	ImplicitConversion                      // an implicit conversion exists (including identity)
	AutoDynamicConversion                   // an automatic dynamic cast should be used (with possible runtime failure)
	ComputedConversion                      // the type converts but the precise value cannot be known yet.
)

// Convert returns the sort of conversion available from a given type to another.
func Convert(from symbols.Type, to symbols.Type) Conversion {
	// Identity conversions are easy.
	if from == to {
		return ImplicitConversion
	}

	// If the from type is `dynamic`, we should auto-cast.
	if from == Dynamic {
		return AutoDynamicConversion
	}

	// Any source type converts to the top-most object type as well as `dynamic`.
	if to == Object || to == Dynamic {
		return ImplicitConversion
	}

	// Any source type converts to the "null" type; and "null" converts to anything.
	// TODO[pulumi/lumi#64]: implement nullable types, in which case this won't be true.
	if to == Null || from == Null {
		return ImplicitConversion
	}

	// Perform some special symbol specific conversions.
	switch t := from.(type) {
	case *symbols.Class:
		// Class types convert to their base types.
		if t.Extends != nil {
			if HasBase(t.Extends, to) {
				return ImplicitConversion
			}
		}
		for _, impl := range t.Implements {
			if HasBase(impl, to) {
				return ImplicitConversion
			}
		}
	case *symbols.ComputedType:
		// A computed type can convert to a regular type provided the underlying type converts.
		if c := Convert(t.Element, to); c != NoConversion {
			return ComputedConversion
		}
	case *symbols.PointerType:
		// A pointer type can be converted to a non-pointer type, since (like Go), dereferences are implicit.
		if c := Convert(t.Element, to); c != NoConversion {
			return c
		}
	case *symbols.ArrayType:
		// Array types with the same element type can convert.
		if toArr, toIsArr := to.(*symbols.ArrayType); toIsArr {
			if t.Element == toArr.Element {
				return ImplicitConversion
			} else if t.Element == Dynamic || toArr.Element == Dynamic {
				return AutoDynamicConversion
			}
		}
	case *symbols.MapType:
		// Map types with the same key and element types can convert.
		if toMap, toIsMap := to.(*symbols.MapType); toIsMap {
			k := NoConversion
			if t.Key == toMap.Key {
				k = ImplicitConversion
			} else if t.Key == Dynamic || toMap.Key == Dynamic {
				k = AutoDynamicConversion
			}
			e := NoConversion
			if t.Element == toMap.Element {
				e = ImplicitConversion
			} else if t.Element == Dynamic || toMap.Element == Dynamic {
				e = AutoDynamicConversion
			}
			if k != NoConversion && e != NoConversion {
				if k < e {
					return e
				}
				return k
			}
		}
	case *symbols.FunctionType:
		// Function types with the same or safely variant parameter and return types can convert.
		if toFunc, toIsFunc := to.(*symbols.FunctionType); toIsFunc {
			ok := true
			// Check the parameters.
			if len(t.Parameters) == len(toFunc.Parameters) {
				for i, param := range t.Parameters {
					// Parameter types are contravariant (source may be weaker).
					if Convert(toFunc.Parameters[i], param) != ImplicitConversion {
						ok = false
					}
				}
				if ok {
					// Check the return type.
					if t.Return == nil {
						if toFunc.Return != nil {
							ok = false
						}
					} else {
						if toFunc.Return == nil {
							ok = false
						} else if Convert(t.Return, toFunc.Return) != ImplicitConversion {
							// Return types are covariant (source may be strengthened).
							ok = false
						}
					}
				}
			} else {
				ok = false
			}
			if ok {
				return ImplicitConversion
			}
		}
	}

	// If the from is the underlying element type of a computed type, this converts implicitly.
	if ev, isev := to.(*symbols.ComputedType); isev {
		return Convert(from, ev.Element)
	}

	// Otherwise, we cannot convert.
	return NoConversion
}

// CanConvert returns true if there's a conversion from a given type to another.
func CanConvert(from symbols.Type, to symbols.Type) bool {
	return Convert(from, to) != NoConversion
}

// HasBase checks a class hierarchy for a base class or interface.
func HasBase(t symbols.Type, base symbols.Type) bool {
	// If the types are equal, we are good.
	if t == base {
		return true
	}

	// Otherwise, look to see if there is a base class conversion.
	if class, isclass := t.(*symbols.Class); isclass {
		if class.Extends != nil && HasBase(class.Extends, base) {
			return true
		}
		for _, impl := range class.Implements {
			if HasBase(impl, base) {
				return true
			}
		}
	}

	// If we got here, we didn't have a match.
	return false
}

// HasBaseName checks a class hierarchy for a base class or interface by the given name.
func HasBaseName(t symbols.Type, base tokens.Type) bool {
	// If the tokens are equal, we are good.
	if t.TypeToken() == base {
		return true
	}

	// Otherwise, look to see if there is a base class conversion.
	if class, isclass := t.(*symbols.Class); isclass {
		if class.Extends != nil && HasBaseName(class.Extends, base) {
			return true
		}
		for _, impl := range class.Implements {
			if HasBaseName(impl, base) {
				return true
			}
		}
	}

	// If we got here, we didn't have a match.
	return false
}
