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

// Hook represents a named resource lifecycle hook block.
//
// Example PCL:
//
//	hook "myHook" {
//	    command  = ["touch", hookTestFile]
//	    onDryRun = false
//	}
//
// A hook can then be referenced in a resource's hooks option:
//
//	options {
//	    hooks = {
//	        beforeCreate = [myHook]
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
}

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
