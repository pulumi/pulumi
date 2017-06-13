// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package eval

import (
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/eval/rt"
)

// Hooks is a set of callbacks that can be used to hook into interesting interpreter events.
type Hooks interface {
	// OnStart is invoked just before interpretation begins.
	OnStart()
	// OnEnterPackage is invoked whenever we enter a package.
	OnEnterPackage(pkg *symbols.Package) func()
	// OnEnterModule is invoked whenever we enter a module.
	OnEnterModule(sym *symbols.Module) func()
	// OnEnterFunction is invoked whenever we enter a function.
	OnEnterFunction(fnc symbols.Function) func()
	// OnObjectInit is invoked after an object has been allocated and initialized.  This means that its constructor, if
	// any, has been run to completion.  The diagnostics tree is the AST node responsible for the allocation.
	OnObjectInit(tree diag.Diagable, o *rt.Object)
	// OnDone is invoked after interpretation has completed.  It is given access to the final unwind information.
	OnDone(uw *rt.Unwind)
}
