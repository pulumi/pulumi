// Copyright 2016 Marapongo, Inc. All rights reserved.

package types

import (
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/tokens"
)

// CanConvert returns true if the from type can be implicitly converted to the to type.
func CanConvert(from symbols.Type, to symbols.Type) bool {
	// Identity conversions are easy.
	if from == to {
		return true
	}

	// Any source type converts to the "any" type.
	if to == Any {
		return true
	}

	// Any source type converts to the "null" type; and "null" converts to anything.
	// TODO: implement nullable types, in which case this won't be true.
	if to == Null || from == Null {
		return true
	}

	// Perform some special symbol specific conversions.
	switch t := from.(type) {
	case *symbols.Class:
		// Class types convert to their base types.
		if t.Extends != nil && CanConvert(t.Extends, to) {
			return true
		}
		for _, impl := range t.Implements {
			if CanConvert(impl, to) {
				return true
			}
		}
	case *symbols.PointerType:
		// A pointer type can be converted to a non-pointer type, since (like Go), dereferences are implicit.
		if CanConvert(t.Element, to) {
			return true
		}
	case *symbols.ArrayType:
		// Array types with the same element type can convert.
		if toArr, toIsArr := to.(*symbols.ArrayType); toIsArr {
			if t.Element == toArr.Element {
				return true
			}
		}
	case *symbols.MapType:
		// Map types with the same key and element types can convert.
		if toMap, toIsMap := to.(*symbols.MapType); toIsMap {
			if t.Key == toMap.Key && t.Element == toMap.Element {
				return true
			}
		}
	case *symbols.FunctionType:
		// Function types with the same or safely variant parameter and return types can convert.
		if toFunc, toIsFunc := to.(*symbols.FunctionType); toIsFunc {
			safe := true
			// Check the parameters.
			if len(t.Parameters) == len(toFunc.Parameters) {
				for i, param := range t.Parameters {
					// Parameter types are contravariant (source may be weaker).
					if !CanConvert(toFunc.Parameters[i], param) {
						safe = false
					}
				}
				if safe {
					// Check the return type.
					if t.Return == nil {
						if toFunc.Return != nil {
							safe = false
						}
					} else {
						if toFunc.Return == nil {
							safe = false
						} else if !CanConvert(t.Return, toFunc.Return) {
							// Return types are covariant (source may be strengthened).
							safe = false
						}
					}
				}
			} else {
				safe = false
			}
			if safe {
				return true
			}
		}
	}

	// Otherwise, we cannot convert; note that an explicit conversion may still exist.
	return false
}

// HasBaseName checks a class hierarchy for a base class or interface by the given name.
func HasBaseName(t symbols.Type, base tokens.Type) bool {
	for {
		class, ok := t.(*symbols.Class)
		if !ok {
			break
		}
		if class.TypeToken() == base {
			return true
		}
		if class.Extends != nil && HasBaseName(class.Extends, base) {
			return true
		}
		for _, impl := range class.Implements {
			if HasBaseName(impl, base) {
				return true
			}
		}
	}
	return false
}
