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

package codegen

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type StringSet map[string]struct{}

func NewStringSet(values ...string) StringSet {
	s := StringSet{}
	for _, v := range values {
		s.Add(v)
	}
	return s
}

func (ss StringSet) Add(s string) {
	ss[s] = struct{}{}
}

func (ss StringSet) Any() bool {
	return len(ss) > 0
}

func (ss StringSet) Delete(s string) {
	delete(ss, s)
}

func (ss StringSet) Has(s string) bool {
	_, ok := ss[s]
	return ok
}

// StringSet.Except returns the string set setminus s.
func (ss StringSet) Except(s string) StringSet {
	return ss.Subtract(NewStringSet(s))
}

func (ss StringSet) SortedValues() []string {
	values := slice.Prealloc[string](len(ss))
	for v := range ss {
		values = append(values, v)
	}
	sort.Strings(values)
	return values
}

// Contains returns true if all elements of the subset are also present in the current set. It also returns true
// if subset is empty.
func (ss StringSet) Contains(subset StringSet) bool {
	for v := range subset {
		if !ss.Has(v) {
			return false
		}
	}
	return true
}

// Subtract returns a new string set with all elements of the current set that are not present in the other set.
func (ss StringSet) Subtract(other StringSet) StringSet {
	result := NewStringSet()
	for v := range ss {
		if !other.Has(v) {
			result.Add(v)
		}
	}
	return result
}

func (ss StringSet) Union(other StringSet) StringSet {
	result := NewStringSet()
	for v := range ss {
		result.Add(v)
	}
	for v := range other {
		result.Add(v)
	}
	return result
}

type Set map[any]struct{}

func (s Set) Add(v any) {
	s[v] = struct{}{}
}

func (s Set) Delete(v any) {
	delete(s, v)
}

func (s Set) Has(v any) bool {
	_, ok := s[v]
	return ok
}

// CleanDir removes all existing files from a directory except those in the exclusions list.
// Note: The exclusions currently don't function recursively, so you cannot exclude a single file
// in a subdirectory, only entire subdirectories. This function will need improvements to be able to
// target that use-case.
func CleanDir(dirPath string, exclusions StringSet) error {
	subPaths, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	if len(subPaths) > 0 {
		for _, path := range subPaths {
			if !exclusions.Has(path.Name()) {
				err = os.RemoveAll(filepath.Join(dirPath, path.Name()))
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

var commonEnumNameReplacements = map[string]string{
	"*": "Asterisk",
	"0": "Zero",
	"1": "One",
	"2": "Two",
	"3": "Three",
	"4": "Four",
	"5": "Five",
	"6": "Six",
	"7": "Seven",
	"8": "Eight",
	"9": "Nine",
}

func ExpandShortEnumName(name string) string {
	if replacement, ok := commonEnumNameReplacements[name]; ok {
		return replacement
	}
	return name
}

// A simple in memory file system.
type Fs map[string][]byte

// Add a new file to the Fs.
//
// Panic if the file is a duplicate.
func (fs Fs) Add(path string, contents []byte) {
	_, has := fs[path]
	contract.Assertf(!has, "duplicate file: %s", path)
	fs[path] = contents
}

// Check if two packages are the same.
func PkgEquals(p1, p2 schema.PackageReference) bool {
	if p1 == p2 {
		return true
	} else if p1 == nil || p2 == nil {
		return false
	}

	if p1.Name() != p2.Name() {
		return false
	}

	v1, v2 := p1.Version(), p2.Version()
	if v1 == v2 {
		return true
	} else if v1 == nil || v2 == nil {
		return false
	}
	return v1.Equals(*v2)
}

// A .gitattributes file allows us to mark filepath patterns as containing generated content. This enables git tools to
// treat these files differently, for example GitHub will show these files as collapsed by default.
func GenGitAttributesFile() []byte {
	return []byte("* linguist-generated\n")
}

// IsWireDiscriminatableUnionType checks whether u is wire discriminatable when known: given a fully-known protobuf
// wire value and the union's type information, the member the value belongs to can always be determined.
//
// The union's members are flattened through optional, input, token, and nested union wrappers; the union is
// discriminatable when at most one member admits null and no wire value is comatchable with two members: no value
// belongs to both. Types are compared by their wire shape (bool, number, string, list, dict, asset, archive,
// resource reference); asset, archive, and resource-reference values carry signature keys that separate them from
// plain dicts, and resource references of different types are separated by the type embedded in their URNs. Within
// a shape:
//   - int and number share the number shape, since all wire numbers arrive as float64.
//   - Enums reduce to their sets of literal values: two members of one primitive shape are disjoint only when both
//     are literal-bounded with disjoint sets.
//   - Lists never separate and neither do two maps: their empty values are indistinguishable. A map and an object
//     are disjoint only when some required property of the object cannot co-match the map's element type.
//   - Two objects share a value exactly when each one's required properties are all declared by the other and
//     every property required by either side admits a common value — recursively by type, with a constant property
//     narrowed to a single-valued enum so its bound distributes through unions and optionals. This assumes closed
//     objects: a value carries only the properties its member declares.
//     The union's declared Discriminator is not trusted; only this structural evidence counts. A common value is
//     finite, so co-matchability is a least fixpoint: recursion through cyclic object graphs reports false on
//     revisits, which makes required-recursive object pairs — types with no finite values at all — vacuously
//     disjoint.
//
// Must match sdk/python/lib/pulumi/_types.py's _wire_matches.
func IsWireDiscriminatableUnionType(u *schema.UnionType) bool {
	members, nullable, ok := flattenWireMembers(u)
	if !ok || nullable > 1 {
		return false
	}
	for i, a := range members {
		for _, b := range members[i+1:] {
			if comatchable(a, b, map[typePair]bool{}) {
				return false
			}
		}
	}
	return true
}

// flattenWireMembers resolves u's members to concrete types, counting how many admit null. ok is false when a
// member admits values whose type cannot be determined from the wire (Any, JSON, an opaque token type).
func flattenWireMembers(u *schema.UnionType) (members []schema.Type, nullable int, ok bool) {
	var visit func(t schema.Type) bool
	visit = func(t schema.Type) bool {
		switch t := t.(type) {
		case *schema.OptionalType:
			nullable++
			return visit(t.ElementType)
		case *schema.InputType:
			return visit(t.ElementType)
		case *schema.TokenType:
			if t.UnderlyingType == nil {
				return false
			}
			return visit(t.UnderlyingType)
		case *schema.UnionType:
			for _, e := range t.ElementTypes {
				if !visit(e) {
					return false
				}
			}
			return true
		default:
			if _, known := wireShapeOf(t); !known {
				return false
			}
			members = append(members, t)
			return true
		}
	}
	for _, e := range u.ElementTypes {
		if !visit(e) {
			return nil, 0, false
		}
	}
	return members, nullable, true
}

type wireShape int

const (
	wireBool wireShape = iota
	wireNumber
	wireString
	wireList
	wireDict
	wireAsset
	wireArchive
	wireResource
)

// wireShapeOf classifies the wire form of a concrete (unwrapped) type; ok is false for types that admit values of
// every shape (Any, JSON, an opaque token).
func wireShapeOf(t schema.Type) (wireShape, bool) {
	switch t {
	case schema.BoolType:
		return wireBool, true
	case schema.IntType, schema.NumberType:
		return wireNumber, true
	case schema.StringType:
		return wireString, true
	case schema.AssetType:
		return wireAsset, true
	case schema.ArchiveType:
		return wireArchive, true
	}
	switch t := t.(type) {
	case *schema.EnumType:
		return wireShapeOf(t.ElementType)
	case *schema.ArrayType:
		return wireList, true
	case *schema.MapType, *schema.ObjectType:
		return wireDict, true
	case *schema.ResourceType:
		return wireResource, true
	}
	return 0, false
}

// typePair keys the cycle guard for object-pair recursion.
type typePair [2]schema.Type

// comatchable reports whether some finite, fully-known wire value belongs to both a and b. seen holds the object
// pairs on the current recursion stack: a common value is finite, so co-matchability that depends on itself has no
// witness and revisits report false, computing a least fixpoint.
func comatchable(a, b schema.Type, seen map[typePair]bool) bool {
	switch t := a.(type) {
	case *schema.InputType:
		return comatchable(t.ElementType, b, seen)
	case *schema.TokenType:
		if t.UnderlyingType == nil {
			return true // Opaque: assume it admits any value.
		}
		return comatchable(t.UnderlyingType, b, seen)
	case *schema.UnionType:
		for _, e := range t.ElementTypes {
			if comatchable(e, b, seen) {
				return true
			}
		}
		return false
	case *schema.OptionalType:
		if _, optional := b.(*schema.OptionalType); optional {
			return true // Null belongs to both.
		}
		return comatchable(t.ElementType, b, seen)
	}
	switch b.(type) {
	case *schema.InputType, *schema.TokenType, *schema.UnionType, *schema.OptionalType:
		return comatchable(b, a, seen)
	}

	shapeA, okA := wireShapeOf(a)
	shapeB, okB := wireShapeOf(b)
	if !okA || !okB {
		return true // Any and friends admit values of every shape.
	}
	if shapeA != shapeB {
		return false
	}
	switch shapeA {
	case wireBool, wireNumber, wireString:
		la, lb := literalSet(a), literalSet(b)
		return la == nil || lb == nil || !setsDisjoint(la, lb)
	case wireResource:
		ra, _ := a.(*schema.ResourceType)
		rb, _ := b.(*schema.ResourceType)
		return ra.Token == rb.Token
	case wireDict:
		return dictsComatchable(a, b, seen)
	case wireList, wireAsset, wireArchive:
		return true // Empty lists — and blobs of one kind — are indistinguishable.
	}
	return true
}

// dictsComatchable resolves the dict wire shape: two maps always share the empty dict, a map and an object share a
// value unless a required property of the object cannot co-match the map's element type, and two objects delegate
// to objectsComatchable.
func dictsComatchable(a, b schema.Type, seen map[typePair]bool) bool {
	oa, aIsObject := a.(*schema.ObjectType)
	ob, bIsObject := b.(*schema.ObjectType)
	switch {
	case aIsObject && bIsObject:
		return objectsComatchable(oa, ob, seen)
	case aIsObject:
		return mapComatchable(b.(*schema.MapType), oa, seen)
	case bIsObject:
		return mapComatchable(a.(*schema.MapType), ob, seen)
	default:
		return true
	}
}

// mapComatchable reports whether some dict of m's element values also matches o: every required property of o must
// admit a value of the element type. Optional properties are omitted from the witness.
func mapComatchable(m *schema.MapType, o *schema.ObjectType, seen map[typePair]bool) bool {
	for _, p := range o.Properties {
		if !p.IsRequired() {
			continue
		}
		if !comatchable(effectivePropertyType(p), m.ElementType, seen) {
			return false
		}
	}
	return true
}

// objectsComatchable reports whether some finite wire value matches both a and b under the closed-object reading: a
// value carries only the properties its member declares. Each side's required properties must all be declared by
// the other, every property required by either side must admit a common value, and properties optional on both
// sides are simply omitted from the witness.
func objectsComatchable(a, b *schema.ObjectType, seen map[typePair]bool) bool {
	pair := typePair{a, b}
	if seen[pair] {
		// Co-matchability that depends on itself has no finite witness.
		return false
	}
	seen[pair] = true
	defer delete(seen, pair)

	for _, x := range [][2]*schema.ObjectType{{a, b}, {b, a}} {
		for _, p := range x[0].Properties {
			if !p.IsRequired() {
				continue
			}
			if _, has := x[1].Property(p.Name); !has {
				return false
			}
		}
	}
	for _, p := range a.Properties {
		q, has := b.Property(p.Name)
		if !has || (!p.IsRequired() && !q.IsRequired()) {
			continue
		}
		if !comatchable(effectivePropertyType(p), effectivePropertyType(q), seen) {
			return false
		}
	}
	return true
}

// effectivePropertyType is the type whose values p admits: the declared type, with a constant property narrowed to
// a single-valued enum so the bound distributes through unions and optionals during co-matching.
func effectivePropertyType(p *schema.Property) schema.Type {
	if p.ConstValue == nil {
		return p.Type
	}
	var t schema.Type = &schema.EnumType{
		ElementType: UnwrapType(p.Type),
		Elements:    []*schema.Enum{{Value: p.ConstValue}},
	}
	if !p.IsRequired() {
		t = &schema.OptionalType{ElementType: t}
	}
	return t
}

// literalSet is the set of wire values t admits when t is literal-bounded (a non-empty enum); nil means unbounded.
func literalSet(t schema.Type) map[any]struct{} {
	e, ok := t.(*schema.EnumType)
	if !ok || len(e.Elements) == 0 {
		return nil
	}
	set := make(map[any]struct{}, len(e.Elements))
	for _, el := range e.Elements {
		set[normalizeWireValue(el.Value)] = struct{}{}
	}
	return set
}

func setsDisjoint(a, b map[any]struct{}) bool {
	for v := range a {
		if _, shared := b[v]; shared {
			return false
		}
	}
	return true
}

// normalizeWireValue widens numeric literals to float64, the representation every number has on the wire.
func normalizeWireValue(v any) any {
	switch v := v.(type) {
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case float32:
		return float64(v)
	default:
		return v
	}
}
