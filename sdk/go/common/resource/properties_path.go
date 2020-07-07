package resource

import (
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// PropertyPath represents a path to a nested property. The path may be composed of strings (which access properties
// in ObjectProperty values) and integers (which access elements of ArrayProperty values).
type PropertyPath []interface{}

// ParsePropertyPath parses a property path into a PropertyPath value.
//
// A property path string is essentially a Javascript property access expression in which all elements are literals.
// Valid property paths obey the following EBNF-ish grammar:
//
//   propertyName := [a-zA-Z_$] { [a-zA-Z0-9_$] }
//   quotedPropertyName := '"' ( '\' '"' | [^"] ) { ( '\' '"' | [^"] ) } '"'
//   arrayIndex := { [0-9] }
//
//   propertyIndex := '[' ( quotedPropertyName | arrayIndex ) ']'
//   rootProperty := ( propertyName | propertyIndex )
//   propertyAccessor := ( ( '.' propertyName ) |  propertyIndex )
//   path := rootProperty { propertyAccessor }
//
// Examples of valid paths:
// - root
// - root.nested
// - root["nested"]
// - root.double.nest
// - root["double"].nest
// - root["double"]["nest"]
// - root.array[0]
// - root.array[100]
// - root.array[0].nested
// - root.array[0][1].nested
// - root.nested.array[0].double[1]
// - root["key with \"escaped\" quotes"]
// - root["key with a ."]
// - ["root key with \"escaped\" quotes"].nested
// - ["root key with a ."][100]
func ParsePropertyPath(path string) (PropertyPath, error) {
	// We interpret the grammar above a little loosely in order to keep things simple. Specifically, we will accept
	// something close to the following:
	// pathElement := { '.' } ( '[' ( [0-9]+ | '"' ('\' '"' | [^"] )+ '"' ']' | [a-zA-Z_$][a-zA-Z0-9_$] )
	// path := { pathElement }
	var elements []interface{}
	for len(path) > 0 {
		switch path[0] {
		case '.':
			path = path[1:]
		case '[':
			// If the character following the '[' is a '"', parse a string key.
			var pathElement interface{}
			if len(path) > 1 && path[1] == '"' {
				var propertyKey []byte
				var i int
				for i = 2; ; {
					if i >= len(path) {
						return nil, errors.New("missing closing quote in property name")
					} else if path[i] == '"' {
						i++
						break
					} else if path[i] == '\\' && i+1 < len(path) && path[i+1] == '"' {
						propertyKey = append(propertyKey, '"')
						i += 2
					} else {
						propertyKey = append(propertyKey, path[i])
						i++
					}
				}
				if i >= len(path) || path[i] != ']' {
					return nil, errors.New("missing closing bracket in property access")
				}
				pathElement, path = string(propertyKey), path[i:]
			} else {
				// Look for a closing ']'
				rbracket := strings.IndexRune(path, ']')
				if rbracket == -1 {
					return nil, errors.New("missing closing bracket in array index")
				}

				index, err := strconv.ParseInt(path[1:rbracket], 10, 0)
				if err != nil {
					return nil, errors.Wrap(err, "invalid array index")
				}
				pathElement, path = int(index), path[rbracket:]
			}
			elements, path = append(elements, pathElement), path[1:]
		default:
			for i := 0; ; i++ {
				if i == len(path) || path[i] == '.' || path[i] == '[' {
					elements, path = append(elements, path[:i]), path[i:]
					break
				}
			}
		}
	}
	return PropertyPath(elements), nil
}

// Get attempts to get the value located by the PropertyPath inside the given PropertyValue. If any component of the
// path does not exist, this function will return (NullPropertyValue, false).
func (p PropertyPath) Get(v PropertyValue) (PropertyValue, bool) {
	for _, key := range p {
		switch {
		case v.IsArray():
			index, ok := key.(int)
			if !ok || index < 0 || index >= len(v.ArrayValue()) {
				return PropertyValue{}, false
			}
			v = v.ArrayValue()[index]
		case v.IsObject():
			k, ok := key.(string)
			if !ok {
				return PropertyValue{}, false
			}
			v, ok = v.ObjectValue()[PropertyKey(k)]
			if !ok {
				return PropertyValue{}, false
			}
		default:
			return PropertyValue{}, false
		}
	}
	return v, true
}

// Set attempts to set the location inside a PropertyValue indicated by the PropertyPath to the given value. If any
// component of the path besides the last component does not exist, this function will return false.
func (p PropertyPath) Set(dest, v PropertyValue) bool {
	if len(p) == 0 {
		return false
	}

	dest, ok := p[:len(p)-1].Get(dest)
	if !ok {
		return false
	}

	key := p[len(p)-1]
	switch {
	case dest.IsArray():
		index, ok := key.(int)
		if !ok || index < 0 || index >= len(dest.ArrayValue()) {
			return false
		}
		dest.ArrayValue()[index] = v
	case dest.IsObject():
		k, ok := key.(string)
		if !ok {
			return false
		}
		dest.ObjectValue()[PropertyKey(k)] = v
	default:
		return false
	}
	return true
}

// Add sets the location inside a PropertyValue indicated by the PropertyPath to the given value. Any components
// referred to by the path that do not exist will be created. If there is a mismatch between the type of an existing
// component and a key that traverses that component, this function will return false. If the destination is a null
// property value, this function will create and return a new property value.
func (p PropertyPath) Add(dest, v PropertyValue) (PropertyValue, bool) {
	if len(p) == 0 {
		return PropertyValue{}, false
	}

	// set sets the destination referred to by the last element of the path to the given value.
	rv := dest
	set := func(v PropertyValue) {
		dest, rv = v, v
	}
	for _, key := range p {
		switch key := key.(type) {
		case int:
			// This key is an int, so we expect an array.
			switch {
			case dest.IsNull():
				// If the destination array does not exist, create a new array with enough room to store the value at
				// the requested index.
				dest = NewArrayProperty(make([]PropertyValue, key+1))
				set(dest)
			case dest.IsArray():
				// If the destination array does exist, ensure that it is large enough to accommodate the requested
				// index.
				arr := dest.ArrayValue()
				if key >= len(arr) {
					arr = append(make([]PropertyValue, key+1-len(arr)), arr...)
					v.V = arr
				}
			default:
				return PropertyValue{}, false
			}
			destV := dest.ArrayValue()
			set = func(v PropertyValue) {
				destV[key] = v
			}
			dest = destV[key]
		case string:
			// This key is a string, so we expect an object.
			switch {
			case dest.IsNull():
				// If the destination does not exist, create a new object.
				dest = NewObjectProperty(PropertyMap{})
				set(dest)
			case dest.IsObject():
				// OK
			default:
				return PropertyValue{}, false
			}
			destV := dest.ObjectValue()
			set = func(v PropertyValue) {
				destV[PropertyKey(key)] = v
			}
			dest = destV[PropertyKey(key)]
		default:
			return PropertyValue{}, false
		}
	}

	set(v)
	return rv, true
}

// Delete attempts to delete the value located by the PropertyPath inside the given PropertyValue. If any component
// of the path does not exist, this function will return false.
func (p PropertyPath) Delete(dest PropertyValue) bool {
	if len(p) == 0 {
		return false
	}

	dest, ok := p[:len(p)-1].Get(dest)
	if !ok {
		return false
	}

	key := p[len(p)-1]
	switch {
	case dest.IsArray():
		index, ok := key.(int)
		if !ok || index < 0 || index >= len(dest.ArrayValue()) {
			return false
		}
		dest.ArrayValue()[index] = PropertyValue{}
	case dest.IsObject():
		k, ok := key.(string)
		if !ok {
			return false
		}
		delete(dest.ObjectValue(), PropertyKey(k))
	default:
		return false
	}
	return true

}
