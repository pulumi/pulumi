// Copyright 2016 Marapongo, Inc. All rights reserved.

package types

import (
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/tokens"
)

// Conversion represents the kind of conversion required to convert from a value of one type to another.
type Conversion int

const (
	NoConversion       Conversion = iota // there is no known conversion (though a cast could work)
	ImplicitConversion                   // an implicit conversion exists (including identity)
	AutoCastConversion                   // an automatic cast should be used (with possible runtime failure)
)

// Convert returns the sort of conversion available from a given type to another.
func Convert(from symbols.Type, to symbols.Type) Conversion {
	// Identity conversions are easy.
	if from == to {
		return ImplicitConversion
	}

	// If the from type is `dynamic`, we should auto-cast.
	if from == Dynamic {
		return AutoCastConversion
	}

	// Any source type converts to the top-most object type.
	if to == Object {
		return ImplicitConversion
	}

	// Any source type converts to the "null" type; and "null" converts to anything.
	// TODO: implement nullable types, in which case this won't be true.
	if to == Null || from == Null {
		return ImplicitConversion
	}

	// Perform some special symbol specific conversions.
	switch t := from.(type) {
	case *symbols.Class:
		// Class types convert to their base types.
		if t.Extends != nil {
			if c := Convert(t.Extends, to); c != NoConversion {
				return c
			}
		}
		for _, impl := range t.Implements {
			if c := Convert(impl, to); c != NoConversion {
				return c
			}
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
			}
		}
	case *symbols.MapType:
		// Map types with the same key and element types can convert.
		if toMap, toIsMap := to.(*symbols.MapType); toIsMap {
			if t.Key == toMap.Key && t.Element == toMap.Element {
				return ImplicitConversion
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

	// Otherwise, we cannot convert.
	return NoConversion
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
