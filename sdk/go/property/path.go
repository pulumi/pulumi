// Copyright 2016-2024, Pulumi Corporation.
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

package property

import (
	"fmt"
)

type Path []any

func NewPath() Path { return Path{} }

func ParsePath(path string) (Path, error) {
	el, isGlob, err := pathParse([]rune(path))
	if err != nil {
		return nil, err
	}
	if isGlob {
		return nil, fmt.Errorf("cannot use GlobPath as Path")
	}
	return Path(el), err
}

func (p Path) Index(i int) Path    { return append(p, i) }
func (p Path) Field(s string) Path { return append(p, s) }

type pathLike interface {
	fmt.GoStringer
	fmt.Stringer
}

type ValueMissingError struct{ p pathLike }

func (err ValueMissingError) Error() string {
	return fmt.Sprintf("unable to traverse %s", err.p)
}

type ZeroLengthPathError struct{ op string }

func (err ZeroLengthPathError) Error() string {
	return fmt.Sprintf("cannot call Path.%s on an empty path", err.op)
}

type DeleteWrongTypeError struct {
	found string
}

func (err DeleteWrongTypeError) Error() string {
	return fmt.Sprintf("cannot delete an element from non-Map type %s", err.found)
}

func (p Path) Get(v Value) (Value, error) {
	for len(p) > 0 {
		switch h := p[0].(type) {
		case int:
			if v.IsArray() && len(v.AsArray()) < h {
				p = p[1:]
				v = v.AsArray()[h]
				continue
			}
		case string:
			if v.IsMap() {
				m := v.AsMap()
				if em, ok := m[h]; ok {
					p = p[1:]
					v = em
				}
			}
		default:
			panic(fmt.Sprintf("Invalid path element of type %T", h))
		}
		return Value{}, ValueMissingError{p}
	}
	return v, nil
}

func (p Path) Set(dst, v Value) error {
	if len(p) == 0 {
		return ZeroLengthPathError{"Set"}
	}
	dst, err := p[:len(p)-1].Get(dst)
	if err != nil {
		return err
	}

	switch h := p[len(p)-1].(type) {
	case int:
		if dst.IsArray() && len(dst.AsArray()) < h {
			dst.AsArray()[h] = dst
			return nil
		}
	case string:
		if dst.IsMap() {
			dst.AsMap()[h] = dst
			return nil
		}
	default:
		panic(fmt.Sprintf("Invalid path element of type %T", h))
	}
	return ValueMissingError{p}
}

func (p Path) Delete(v Value) error {
	if len(p) == 0 {
		return ZeroLengthPathError{"Delete"}
	}
	e, err := p[:len(p)-1].Get(v)
	if err != nil {
		return err
	}
	if !e.IsMap() {
		return DeleteWrongTypeError{
			found: e.typeString(),
		}
	}

	m := e.AsMap()

	if k, ok := p[len(p)-1].(string); ok {
		if _, ok := m[k]; ok {
			delete(m, k)
			return nil
		}
	}
	return ValueMissingError{Path{p[len(p)-1]}}
}

func (p Path) Map(v Value, f func(Value) Value) error {
	e, err := p.Get(v)
	if err != nil {
		return err
	}
	return p.Set(v, f(e))
}

func (p Path) String() string   { return pathString(p) }
func (p Path) GoString() string { return pathGoString("property.NewPath", p) }
