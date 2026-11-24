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

// Package noosexit provides a golangci-lint module plugin that reports
// process-terminating calls (os.Exit and the cmdutil.Exit/cmdutil.ExitError
// helpers) outside of the main and TestMain functions. Exiting elsewhere skips
// deferred cleanup and makes code untestable; functions should return an error
// and let main decide the process exit code. See .custom-gcl.yml for how the
// plugin is wired into the custom golangci-lint binary.
package noosexit

import (
	"go/ast"
	"go/types"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

func init() {
	register.Plugin("noosexit", New)
}

func New(any) (register.LinterPlugin, error) {
	return &plugin{}, nil
}

type plugin struct{}

func (p *plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{Analyzer}, nil
}

func (p *plugin) GetLoadMode() string {
	return register.LoadModeTypesInfo
}

// cmdutilPath is the import path of the package that owns the Exit helpers.
const cmdutilPath = "github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"

// forbiddenExits maps an import path to the set of function names within that
// package whose calls terminate the process and so are permitted only in main
// and TestMain.
var forbiddenExits = map[string]map[string]bool{
	"os":        {"Exit": true},
	cmdutilPath: {"Exit": true, "ExitError": true},
}

// Analyzer reports calls to os.Exit, cmdutil.Exit, and cmdutil.ExitError that
// appear outside of the main and TestMain functions.
var Analyzer = &analysis.Analyzer{
	Name: "noosexit",
	Doc:  "reports process-terminating calls outside of the main and TestMain functions",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || isEntrypoint(fn) {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if exit := forbiddenExit(pass, call); exit != nil {
					pass.Reportf(call.Pos(),
						"do not call %s.%s outside of main or TestMain; return an error instead",
						exit.Pkg().Name(), exit.Name())
				}
				return true
			})
		}
	}
	return nil, nil
}

// isEntrypoint reports whether fn is a function in which a process-terminating
// call is permitted: a package's main entrypoint or a test binary's TestMain.
func isEntrypoint(fn *ast.FuncDecl) bool {
	return fn.Recv == nil && (fn.Name.Name == "main" || fn.Name.Name == "TestMain")
}

// forbiddenExit returns the function call invokes if it is a process-terminating
// call listed in forbiddenExits, or nil otherwise. The callee is resolved
// through type information so that import aliases are handled correctly. Calls
// written as bare identifiers (i.e. from within the defining package) are
// intentionally not matched, so the helpers may compose with one another.
func forbiddenExit(pass *analysis.Pass, call *ast.CallExpr) *types.Func {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok || fn.Pkg() == nil {
		return nil
	}
	if forbiddenExits[fn.Pkg().Path()][fn.Name()] {
		return fn
	}
	return nil
}
