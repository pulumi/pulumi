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

package pcl

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/zclconf/go-cty/cty"
)

// Hook represents a named hook block. The first label declares the hook's kind: a
// `resource` hook runs at resource lifecycle steps, while an `error` hook runs when an
// operation fails retryably and its command's exit status decides whether to retry.
//
// Example PCL:
//
//	hook resource "myHook" {
//	    command      = ["touch", hookTestFile]
//	    onDryRun     = false
//	    ignoreErrors = false
//	}
//
//	hook error "myErrorHook" {
//	    command = ["touch", hookTestFile]
//	}
//
// A hook can then be referenced in a resource's hooks option:
//
//	options {
//	    hooks = {
//	        beforeCreate = [myHook]
//	        onError      = [myErrorHook]
//	    }
//	}
type Hook struct {
	node

	syntax      *hclsyntax.Block
	logicalName string

	Definition *model.Block

	// Command is the command and its arguments to run when the hook fires.
	Command model.Expression

	// OnDryRun, when set to true, causes the hook to run during preview operations.
	// Defaults to false.
	OnDryRun model.Expression

	// IgnoreErrors, when set to true, causes errors from the hook to be logged as warnings
	// instead of failing the program. Defaults to false.
	IgnoreErrors model.Expression

	// Kind is the kind of the hook, declared by the block's first label. Resource hooks run
	// at resource lifecycle steps (`beforeCreate`, `afterDelete`, etc.). Error hooks run when
	// an operation fails retryably and signal whether it should be retried: the operation is
	// retried if and only if the hook's command exits successfully.
	Kind HookKind
}

// HookKind is the kind of a hook block: resource or error, matching the two hook
// registration kinds in the language SDKs.
type HookKind string

const (
	// HookKindResource is a hook that runs at resource lifecycle steps.
	HookKindResource HookKind = "resource"
	// HookKindError is a hook that runs when an operation fails retryably.
	HookKindError HookKind = "error"
)

// SyntaxNode returns the syntax node associated with the hook.
func (h *Hook) SyntaxNode() hclsyntax.Node {
	return h.syntax
}

func (h *Hook) Value(context *hcl.EvalContext) (cty.Value, hcl.Diagnostics) {
	return cty.StringVal(h.Name()), nil
}

// Traverse implements model.Traversable. Hooks are opaque references; traversal
// is not supported.
func (h *Hook) Traverse(traverser hcl.Traverser) (model.Traversable, hcl.Diagnostics) {
	return model.DynamicType, hcl.Diagnostics{}
}

// VisitExpressions visits expressions contained in the hook's definition.
func (h *Hook) VisitExpressions(pre, post model.ExpressionVisitor) hcl.Diagnostics {
	return model.VisitExpressions(h.Definition, pre, post)
}

// Name returns the logical name of the hook.
func (h *Hook) Name() string {
	return h.logicalName
}

// Type returns the type of the hook node. Hooks are opaque values referenced by
// name, so we use DynamicType.
func (h *Hook) Type() model.Type {
	return model.DynamicType
}

// LogicalName returns the API-visible name of the hook.
func (h *Hook) LogicalName() string {
	return h.logicalName
}
