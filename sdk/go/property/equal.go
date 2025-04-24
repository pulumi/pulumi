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

type EqualOption func(*eqOpts)

// See the doc comment for Value.Equals for the effect of EqualRelaxComputed.
func EqualRelaxComputed(opts *eqOpts) {
	opts.relaxComputed = true
}

type eqOpts struct {
	relaxComputed bool
}

// Check if two Values are equal.
//
// There are two corner cases that need to be called out here:
//
//   - Secret equality is enforced. That means that:
//
//     {"a", secret: false} == {"a", secret: false}
//
//     {"a", secret: true} != {"a", secret: false}
//
//     {"b", secret: false} != {"c", secret: false}
//
//   - Computed value equality has two different modes. By default, it works like Null
//     equality: a.IsComputed() => (a.Equals(b) <=> b.IsComputed()) (up to secrets and
//     dependencies).
//
//     If [EqualRelaxComputed] is passed, then computed values are considered equal to all
//     other values. (up to secrets and dependencies)
func (v Value) Equals(other Value, opts ...EqualOption) bool {
	var eqOpts eqOpts
	for _, o := range opts {
		o(&eqOpts)
	}
	return v.equals(other, eqOpts)
}

func (v Value) equals(other Value, opts eqOpts) bool {
	if v.isSecret != other.isSecret {
		return false
	}

	if len(v.dependencies) != len(other.dependencies) {
		return false
	}
	for i, d := range v.dependencies {
		if other.dependencies[i] != d {
			return false
		}
	}

	if opts.relaxComputed && (v.IsComputed() || other.IsComputed()) {
		return true
	}

	switch {
	case v.IsBool() && other.IsBool():
		return v.AsBool() == other.AsBool()
	case v.IsNumber() && other.IsNumber():
		return v.AsNumber() == other.AsNumber()
	case v.IsString() && other.IsString():
		return v.AsString() == other.AsString()
	case v.IsArray() && other.IsArray():
		a1, a2 := v.AsArray(), other.AsArray()
		if a1.Len() != a2.Len() {
			return false
		}
		for i := range a1.arr {
			if !a1.arr[i].equals(a2.arr[i], opts) {
				return false
			}
		}
		return true
	case v.IsMap() && other.IsMap():
		m1, m2 := v.AsMap(), other.AsMap()
		if m1.Len() != m2.Len() {
			return false
		}
		for k, v1 := range m1.m {
			v2, ok := m2.m[k]
			if !ok || !v1.equals(v2, opts) {
				return false
			}
		}
		return true
	case v.IsAsset() && other.IsAsset():
		a1, a2 := v.asAssetMut(), other.asAssetMut()
		return a1.Equals(a2)
	case v.IsArchive() && other.IsArchive():
		a1, a2 := v.asArchiveMut(), other.asArchiveMut()
		return a1.Equals(a2)
	case v.IsResourceReference() && other.IsResourceReference():
		r1, r2 := v.AsResourceReference(), other.AsResourceReference()
		return r1.Equal(r2)
	case v.IsNull() && other.IsNull():
		return true
	case v.IsComputed() && other.IsComputed():
		return true
	default:
		return false
	}
}
