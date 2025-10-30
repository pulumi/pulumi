// Copyright 2016-2020, Pulumi Corporation.
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

package model

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model/pretty"
	"github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/sdk/v3/pkg/util/gsync"
)

type noneType int

func (noneType) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

func (noneType) pretty(seenFormatters map[Type]pretty.Formatter) pretty.Formatter {
	return pretty.FromStringer(NoneType)
}

func (noneType) Pretty() pretty.Formatter {
	return pretty.FromStringer(NoneType)
}

func (noneType) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	return NoneType, hcl.Diagnostics{unsupportedReceiverTypeWarning(NoneType, traverser.SourceRange())}
}

func (n noneType) Equals(other Type) bool {
	return n.equals(other, nil)
}

func (noneType) equals(other Type, seen map[Type]struct{}) bool {
	return other == NoneType
}

func (noneType) AssignableFrom(src Type) bool {
	return assignableFrom(NoneType, src, func() bool {
		return false
	})
}

func (noneType) ConversionFrom(src Type) ConversionKind {
	kind, _ := NoneType.conversionFrom(src, false, nil)
	return kind
}

func (noneType) conversionFrom(src Type, unifying bool, seen map[Type]struct{}) (ConversionKind, lazyDiagnostics) {
	return conversionFrom(
		NoneType, src, unifying, seen, &gsync.Map[Type, cacheEntry]{}, func() (ConversionKind, lazyDiagnostics) {
			return NoConversion, nil
		})
}

func (noneType) String() string {
	return "none"
}

func (noneType) string(_ map[Type]struct{}) string {
	return "none"
}

func (noneType) unify(other Type) (Type, ConversionKind) {
	return unify(NoneType, other, func() (Type, ConversionKind) {
		return NoneType, other.ConversionFrom(NoneType)
	})
}

func (noneType) isType()	{}
