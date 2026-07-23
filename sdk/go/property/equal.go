// Copyright 2016, Pulumi Corporation.
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
		return v.AsArray().equals(other.AsArray(), opts)
	case v.IsMap() && other.IsMap():
		return v.AsMap().equals(other.AsMap(), opts)
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

// Check if two Maps are equal.
//
// See Value.Equals for the detailed semantics of equality and the effect of EqualOption.
func (m Map) Equals(other Map, opts ...EqualOption) bool {
	var eqOpts eqOpts
	for _, o := range opts {
		o(&eqOpts)
	}
	return m.equals(other, eqOpts)
}

func (m Map) equals(other Map, opts eqOpts) bool {
	if m.Len() != other.Len() {
		return false
	}

	for k, v := range m.m {
		otherV, ok := other.m[k]
		if !ok || !v.equals(otherV, opts) {
			return false
		}
	}

	return true
}

// Check if two Arrays are equal.
//
// See Value.Equals for the detailed semantics of equality and the effect of EqualOption.
func (a Array) Equals(other Array, opts ...EqualOption) bool {
	var eqOpts eqOpts
	for _, o := range opts {
		o(&eqOpts)
	}
	return a.equals(other, eqOpts)
}

func (a Array) equals(other Array, opts eqOpts) bool {
	if a.Len() != other.Len() {
		return false
	}

	for i := range a.arr {
		if !a.arr[i].equals(other.arr[i], opts) {
			return false
		}
	}

	return true
}
