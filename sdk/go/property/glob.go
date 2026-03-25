// Copyright 2026, Pulumi Corporation.
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

import "github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

type Glob []GlobSegment

func (g Glob) Get(v Value) ([]Value, error) {
	stack := []Value{v}
	for _, segment := range g {
		for i := len(stack) - 1; i >= 0; i-- {
			var err PathApplyFailure
			expansion, err := segment.globApply(stack[i])
			if err != nil {
				return nil, err
			}
			if len(expansion) == 0 {
				if len(stack) == 1 {
					// We have expanded the last expression out to nothing, so we are done.
					return nil, nil
				}
				stack[i] = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
			stack[i] = expansion[0]
			stack = append(stack, expansion[1:]...)
		}
	}
	return stack, nil
}

type GlobSegment interface {
	globApply(Value) ([]Value, PathApplyFailure)
}

var (
	_ GlobSegment = Splat
	_ GlobSegment = KeySegment{}
	_ GlobSegment = IndexSegment{}
)

var Splat splat

type splat struct{}

func (splat) globApply(v Value) ([]Value, PathApplyFailure) {
	switch {
	case v.IsMap():
		values := make([]Value, 0, v.AsMap().Len())
		for _, v := range v.AsMap().AllStable {
			values = append(values, v)
		}
		return values, nil
	case v.IsArray():
		return v.AsArray().AsSlice(), nil
	default:
		return nil, pathErrorf(v, "expected a map or array, found %s", typeString(v))
	}
}

func (s KeySegment) globApply(v Value) ([]Value, PathApplyFailure) {
	r, err := s.apply(v)
	if err != nil {
		return nil, err
	}
	return []Value{r}, nil
}

func (s IndexSegment) globApply(v Value) ([]Value, PathApplyFailure) {
	r, err := s.apply(v)
	if err != nil {
		return nil, err
	}
	return []Value{r}, nil
}

func (s Path) AsGlob() Glob {
	g := make(Glob, len(s))
	for i, v := range s {
		g[i] = v
	}
	return g
}

// HasPrefix returns true if the receiver property path contains the other property path.
// For example, the path `foo["bar"][1]` contains the path `foo.bar[1].baz`.  The key
// `"*"` is a wildcard which matches any string or int index at that same nesting level.
// So for example, the path `foo.*.baz` contains `foo.bar.baz.bam`, and the path `*`
// contains any path.
func (g Glob) HasPrefix(p Path) bool {
	// g cannot contain p when p is longer then g.
	if len(p) < len(g) {
		return false
	}
	for i := range g {
		op := p[i]
		switch gp := g[i].(type) {
		case KeySegment:
			if v, ok := op.(KeySegment); ok && v == gp {
				continue
			}
			return false
		case IndexSegment:
			if v, ok := op.(IndexSegment); ok && v == gp {
				continue
			}
			return false
		case splat:
			continue
		default:
			contract.Failf("invalid glob segment %T", gp)
		}
	}
	return true
}
