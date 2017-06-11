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

package main

import (
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/pulumi/lumi/pkg/compiler"
	"github.com/pulumi/lumi/pkg/compiler/binder"
	"github.com/pulumi/lumi/pkg/compiler/core"
	"github.com/pulumi/lumi/pkg/compiler/errors"
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/pack"
	"github.com/pulumi/lumi/pkg/util/cmdutil"
	"github.com/pulumi/lumi/pkg/util/contract"
)

func NewLumiCmd() *cobra.Command {
	var logFlow bool
	var logToStderr bool
	var verbose int
	cmd := &cobra.Command{
		Use:   "lumi",
		Short: "Lumi is a framework and toolset for reusable stacks of services",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cmdutil.InitLogging(logToStderr, verbose, logFlow)
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			glog.Flush()
		},
	}

	cmd.PersistentFlags().BoolVar(&logFlow, "logflow", false, "Flow log settings to child processes (like plugins)")
	cmd.PersistentFlags().BoolVar(&logToStderr, "logtostderr", false, "Log to stderr instead of to files")
	cmd.PersistentFlags().IntVarP(
		&verbose, "verbose", "v", 0, "Enable verbose logging (e.g., v=3); anything >3 is very verbose")

	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newDeployCmd())
	cmd.AddCommand(newDestroyCmd())
	cmd.AddCommand(newEnvCmd())
	cmd.AddCommand(newPackCmd())
	cmd.AddCommand(newPlanCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}

func prepareCompiler(cmd *cobra.Command, args []string) (compiler.Compiler, *pack.Package) {
	// TODO[pulumi/lumi#88]: enable arguments to flow to the package itself.  In that case, we want to split the
	//     arguments at the --, if any, so we can still pass arguments to the compiler itself in these cases.
	var pkgarg string
	if len(args) > 0 {
		pkgarg = args[0]
	}

	// Create a compiler options object and map any flags and arguments to settings on it.
	opts := core.DefaultOptions()

	// If a package argument is present, try to load that package (either via stdin or a path).
	var pkg *pack.Package
	var root string
	if pkgarg != "" {
		pkg, root = readPackageFromArg(pkgarg)
	}

	// Now create a compiler object based on whether we loaded a package or just have a root to deal with.
	var comp compiler.Compiler
	var err error
	if root == "" {
		comp, err = compiler.Newwd(opts)
	} else {
		comp, err = compiler.New(root, opts)
	}
	if err != nil {
		cmdutil.Diag().Errorf(errors.ErrorCantCreateCompiler, err)
	}

	return comp, pkg
}

// compile just uses the standard logic to parse arguments, options, and to locate/compile a package.  It returns the
// compilation result, or nil if an error occurred (in which case, we would expect diagnostics to have been output).
func compile(cmd *cobra.Command, args []string) *compileResult {
	// Prepare the compiler info and, provided it succeeds, perform the compilation.
	if comp, pkg := prepareCompiler(cmd, args); comp != nil {
		var b binder.Binder
		var pkgsym *symbols.Package
		if pkg == nil {
			b, pkgsym = comp.Compile()
		} else {
			b, pkgsym = comp.CompilePackage(pkg)
		}
		contract.Assert(b != nil)
		contract.Assert(pkgsym != nil)
		return &compileResult{
			C:   comp,
			B:   b,
			Pkg: pkgsym,
		}
	}

	return nil
}

type compileResult struct {
	C   compiler.Compiler
	B   binder.Binder
	Pkg *symbols.Package
}
