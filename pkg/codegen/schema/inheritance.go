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

// This file implements component inheritance for the schema binder: resolving `extends` chains, detecting cycles,
// flattening inherited members into the bound model, validating overrides, and the requiredFeatures gate. It is invoked
// from bindSpec after every resource is bound. The resulting bound model is always flattened; the Own/Inherited helpers
// on *Resource (schema.go) partition that flattened set against the base.

package schema

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
)

// featureInheritance is the requiredFeatures flag a sparse (non-flattened) schema must declare so that consumers which
// do not understand component inheritance reject the schema rather than silently dropping inherited members.
const featureInheritance = "inheritance"

// resolveResourceExtends resolves and flattens a single component's `extends` chain. It is invoked from bindResourceDef
// right after the component's own members are bound, so inheritance is resolved uniformly on every bind path — full
// BindSpec, lazy PartialPackage.Get, and PartialPackage.Definition alike. It wires up the immediate base
// (BaseResource), flattens the inherited member set into the bound model, and validates re-listed copies, method
// overrides, and collisions. Same-package bases are resolved recursively through the memoized bindResourceDef, which
// flattens the base first; a resolution stack on the binder detects extends cycles. Materialization of an omitted
// member sets sawSparseInheritance, which the requiredFeatures gate consumes.
//
// The bound InputProperties/Properties are always the full flattened set; OwnProperties/InheritedProperties partition
// that set against the base. Methods are never flattened (their tokens are same-package by construction), so overrides
// are validated but not copied.
func (t *types) resolveResourceExtends(
	path, token string, spec ResourceSpec, res *Resource, options ValidationOptions,
) hcl.Diagnostics {
	var diags hcl.Diagnostics
	if spec.Extends.Ref == "" {
		return diags.Append(errorf(path+"/extends", "extends must specify a $ref"))
	}
	refPath := path + "/extends/$ref"
	ref, refDiags := t.parseTypeSpecRef(refPath, spec.Extends.Ref)
	diags = diags.Extend(refDiags)
	if refDiags.HasErrors() {
		return diags
	}
	if ref.Kind != resourcesRef {
		return diags.Append(errorf(refPath,
			"extends must reference a resource, but %q is not a resource reference", spec.Extends.Ref))
	}

	samePackage := ref.Package == t.pkg.Name && versionEquals(ref.Version, t.pkg.Version)

	// Push this token onto the resolution stack so that a same-package base whose own chain refers back to it closes a
	// cycle. The base is resolved (and, for same-package, flattened) while this frame is on the stack.
	t.extendsInProgress[token] = len(t.extendsPath)
	t.extendsPath = append(t.extendsPath, token)
	defer func() {
		delete(t.extendsInProgress, token)
		t.extendsPath = t.extendsPath[:len(t.extendsPath)-1]
	}()

	if samePackage {
		if pos, ok := t.extendsInProgress[ref.Token]; ok {
			cycle := make([]string, 0, len(t.extendsPath)-pos+1)
			cycle = append(cycle, t.extendsPath[pos:]...)
			cycle = append(cycle, ref.Token)
			return diags.Append(errorf(path+"/extends", "extends cycle detected: %s", strings.Join(cycle, " -> ")))
		}
	}

	base, desc, baseDiags := t.resolveExtendsBase(refPath, ref, samePackage, options)
	diags = diags.Extend(baseDiags)
	if base == nil {
		return diags
	}
	if !base.IsComponent {
		return diags.Append(errorf(path+"/extends",
			"%v cannot extend %v because %v is not a component", token, base.Token, base.Token))
	}

	res.BaseResource = base
	res.baseDescriptor = desc

	flatDiags, materialized := t.flattenAgainstBase(res)
	diags = diags.Extend(flatDiags)
	if materialized {
		t.sawSparseInheritance = true
	}
	return diags
}

// resolveExtendsBase loads the base component named by an already-parsed and cycle-checked reference. Same-package
// references resolve through the memoized bindResourceDef (which flattens the base first); cross-package references
// reuse the external-ref machinery and return the descriptor used to load the base. Unlike ordinary property
// references, an unresolvable base is always an error — a dangling base class has no usable degradation, even under
// AllowDanglingReferences.
//
// The same-package base's own diagnostics are propagated to the caller: because bindResourceDef is memoized and
// resolution runs inline, a base first bound through this extends edge would otherwise have its diagnostics (a cycle
// deeper in the chain, say) silently dropped when finishResources later re-requests it and gets the memoized copy.
func (t *types) resolveExtendsBase(
	refPath string, ref typeSpecRef, samePackage bool, options ValidationOptions,
) (*Resource, *PackageDescriptor, hcl.Diagnostics) {
	if !samePackage {
		pkgdesc := t.dependencyDescriptor(ref)
		pkg, err := LoadPackageReferenceV2(context.TODO(), t.loader, pkgdesc)
		if err != nil {
			return nil, nil, hcl.Diagnostics{errorf(refPath, "resolving base package %v: %v", ref.Package, err)}
		}
		base, ok, err := pkg.Resources().Get(ref.Token)
		if err != nil {
			return nil, nil, hcl.Diagnostics{errorf(refPath, "loading base resource %v: %v", ref.Token, err)}
		}
		if !ok || base == nil {
			return nil, nil, hcl.Diagnostics{errorf(refPath, "base resource %v not found in package %v", ref.Token, ref.Package)}
		}
		return base, pkgdesc, nil
	}

	base, baseDiags, err := t.bindResourceDef(ref.Token, options)
	if err != nil {
		return nil, nil, hcl.Diagnostics{errorf(refPath, "binding base resource %v: %v", ref.Token, err)}
	}
	if base == nil {
		return nil, nil, hcl.Diagnostics{errorf(refPath, "base resource %v not found", ref.Token)}
	}
	return base, nil, baseDiags
}

// dependencyDescriptor finds the package descriptor for an external reference in the package's dependency list, falling
// back to a bare name/version descriptor when the dependency is not listed. This mirrors the resolution in
// bindTypeSpecRef.
func (t *types) dependencyDescriptor(ref typeSpecRef) *PackageDescriptor {
	for i := range t.pkg.Dependencies {
		d := t.pkg.Dependencies[i]
		name, version := d.Name, d.Version
		if d.Parameterization != nil {
			name, version = d.Parameterization.Name, &d.Parameterization.Version
		}
		if name == ref.Package && versionEquals(version, ref.Version) {
			return &d
		}
	}
	return &PackageDescriptor{Name: ref.Package, Version: ref.Version}
}

// flattenAgainstBase merges a component's inherited members into its (already flattened) base. Members the derived spec
// re-lists must match the base exactly; members it omits are materialized from the base. It also validates method
// overrides and cross-level name collisions. It returns whether any member had to be materialized.
func (t *types) flattenAgainstBase(res *Resource) (hcl.Diagnostics, bool) {
	var diags hcl.Diagnostics
	base := res.BaseResource
	path := memberPath("resources", res.Token)

	inputs, inputMaterialized, inputDiags := mergeInheritedProperties(
		path+"/inputProperties", res.InputProperties, base.InputProperties, base.Token)
	diags = diags.Extend(inputDiags)

	outputs, outputMaterialized, outputDiags := mergeInheritedProperties(
		path+"/properties", res.Properties, base.Properties, base.Token)
	diags = diags.Extend(outputDiags)

	res.InputProperties = inputs
	res.Properties = outputs

	// Validate method overrides and cross-level property/method collisions. Methods are never flattened, so the base's
	// methods surface through AllMethods; a same-named derived method is an override and must match the base signature.
	inheritedProps := map[string]struct{}{}
	for _, p := range base.Properties {
		inheritedProps[p.Name] = struct{}{}
	}
	for _, p := range base.InputProperties {
		inheritedProps[p.Name] = struct{}{}
	}
	inheritedMethods := map[string]*Method{}
	for _, m := range base.AllMethods() {
		inheritedMethods[m.Name] = m
	}
	for _, m := range res.Methods {
		mpath := path + "/methods/" + m.Name
		if _, ok := inheritedProps[m.Name]; ok {
			diags = diags.Append(errorf(mpath, "method %q collides with a property inherited from %v", m.Name, base.Token))
		}
		if bm, ok := inheritedMethods[m.Name]; ok && !equalMethodSignature(m.Function, bm.Function) {
			diags = diags.Append(errorf(mpath,
				"override of method %q changes its signature; overrides must match the base signature exactly", m.Name))
		}
	}
	for _, p := range res.ownDeclaredNames(inheritedProps) {
		if _, ok := inheritedMethods[p]; ok {
			diags = diags.Append(errorf(path+"/properties/"+url.PathEscape(p),
				"property %q collides with a method inherited from %v", p, base.Token))
		}
	}

	return diags, inputMaterialized || outputMaterialized
}

// ownDeclaredNames returns the distinct names of this resource's own (non-inherited) input and output properties, i.e.
// the flattened members whose names are absent from the inherited set.
func (r *Resource) ownDeclaredNames(inherited map[string]struct{}) []string {
	seen := map[string]struct{}{}
	var names []string
	for _, list := range [][]*Property{r.Properties, r.InputProperties} {
		for _, p := range list {
			if _, isInherited := inherited[p.Name]; isInherited {
				continue
			}
			if _, ok := seen[p.Name]; ok {
				continue
			}
			seen[p.Name] = struct{}{}
			names = append(names, p.Name)
		}
	}
	sort.Strings(names)
	return names
}

// mergeInheritedProperties folds a base's flattened property list into a derived component's declared list. Re-listed
// members are validated against the base; omitted members are materialized (copied from the base). The result is the
// flattened list, sorted by name, plus whether any member was materialized.
func mergeInheritedProperties(
	path string, declared, base []*Property, baseToken string,
) ([]*Property, bool, hcl.Diagnostics) {
	var diags hcl.Diagnostics
	byName := make(map[string]*Property, len(declared))
	for _, p := range declared {
		byName[p.Name] = p
	}
	result := make([]*Property, len(declared))
	copy(result, declared)
	materialized := false
	for _, bp := range base {
		if dp, ok := byName[bp.Name]; ok {
			if !equalProperty(dp, bp) {
				diags = diags.Append(errorf(path+"/"+url.PathEscape(bp.Name),
					"inherited property %q does not match base %v; flattened copies must match the base exactly",
					bp.Name, baseToken))
			}
			continue
		}
		result = append(result, bp)
		materialized = true
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, materialized, diags
}

// validateRequiredFeatures checks the package's requiredFeatures list. Unknown features are errors (that is the
// forward-compatibility lever: a schema may demand a feature this binder does not provide). A schema that relied on
// inheritance materialization must declare the "inheritance" feature so that old consumers reject it rather than
// silently drop members.
func validateRequiredFeatures(features []string, sparse bool) hcl.Diagnostics {
	var diags hcl.Diagnostics
	declared := false
	for i, f := range features {
		if f == featureInheritance {
			declared = true
			continue
		}
		diags = diags.Append(errorf(fmt.Sprintf("#/requiredFeatures/%d", i),
			"this schema requires feature %q which this version of Pulumi does not support", f))
	}
	if sparse && !declared {
		diags = diags.Append(errorf("#/requiredFeatures",
			"this schema relies on inheritance materialization and must declare requiredFeatures: [%q]", featureInheritance))
	}
	return diags
}

// equalType reports whether two bound types are structurally equal. Named types (objects, enums, resources, tokens)
// compare by token — which encodes package identity, so an external binding and a local binding of the same named type
// compare equal without needing pointer identity, and recursion terminates on recursive types. Wrapper types recurse.
func equalType(a, b Type) bool {
	if a == b {
		return true
	}
	switch at := a.(type) {
	case *OptionalType:
		bt, ok := b.(*OptionalType)
		return ok && equalType(at.ElementType, bt.ElementType)
	case *InputType:
		bt, ok := b.(*InputType)
		return ok && equalType(at.ElementType, bt.ElementType)
	case *ArrayType:
		bt, ok := b.(*ArrayType)
		return ok && equalType(at.ElementType, bt.ElementType)
	case *MapType:
		bt, ok := b.(*MapType)
		return ok && equalType(at.ElementType, bt.ElementType)
	case *UnionType:
		bt, ok := b.(*UnionType)
		if !ok || len(at.ElementTypes) != len(bt.ElementTypes) || at.Discriminator != bt.Discriminator {
			return false
		}
		for i := range at.ElementTypes {
			if !equalType(at.ElementTypes[i], bt.ElementTypes[i]) {
				return false
			}
		}
		return true
	case *ObjectType:
		bt, ok := b.(*ObjectType)
		return ok && at.Token == bt.Token && at.IsInputShape() == bt.IsInputShape()
	case *EnumType:
		bt, ok := b.(*EnumType)
		return ok && at.Token == bt.Token
	case *ResourceType:
		bt, ok := b.(*ResourceType)
		return ok && at.Token == bt.Token
	case *TokenType:
		bt, ok := b.(*TokenType)
		return ok && at.Token == bt.Token
	default:
		return a.String() == b.String()
	}
}

// equalProperty reports whether two bound properties are structurally identical for the purpose of the flattened-copy
// check: same type (which captures required-ness via the optional wrapper), and equal plain/secret/const/default.
// Documentation and deprecation are deliberately excluded — a derived spec may override those.
func equalProperty(a, b *Property) bool {
	return equalType(a.Type, b.Type) &&
		a.Plain == b.Plain &&
		a.Secret == b.Secret &&
		reflect.DeepEqual(a.ConstValue, b.ConstValue) &&
		reflect.DeepEqual(a.DefaultValue, b.DefaultValue)
}

// equalMethodSignature reports whether two method functions have the same signature: equal inputs (ignoring the
// receiver __self__, whose type is naturally the enclosing resource) and equal outputs. This is the exact-signature
// rule that overrides must satisfy in v1.
func equalMethodSignature(a, b *Function) bool {
	return equalObjectProperties(a.Inputs, b.Inputs, true) && equalObjectProperties(a.Outputs, b.Outputs, false)
}

// equalObjectProperties reports whether two object types carry the same set of properties, comparing each by
// equalProperty. When skipSelf is set the __self__ receiver property is ignored on both sides.
func equalObjectProperties(a, b *ObjectType, skipSelf bool) bool {
	ap := objectProperties(a, skipSelf)
	bp := objectProperties(b, skipSelf)
	if len(ap) != len(bp) {
		return false
	}
	byName := make(map[string]*Property, len(bp))
	for _, p := range bp {
		byName[p.Name] = p
	}
	for _, p := range ap {
		q, ok := byName[p.Name]
		if !ok || !equalProperty(p, q) {
			return false
		}
	}
	return true
}

// objectProperties returns an object type's properties, optionally omitting the __self__ receiver. A nil object has no
// properties.
func objectProperties(o *ObjectType, skipSelf bool) []*Property {
	if o == nil {
		return nil
	}
	if !skipSelf {
		return o.Properties
	}
	result := slice.Prealloc[*Property](len(o.Properties))
	for _, p := range o.Properties {
		if p.Name == "__self__" {
			continue
		}
		result = append(result, p)
	}
	return result
}
