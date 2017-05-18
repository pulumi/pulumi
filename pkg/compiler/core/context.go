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

package core

import (
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/diag"
)

// Context is a bag of state common throughout all compiler passes.
type Context struct {
	Path       string             // the root directory.
	Diag       diag.Sink          // the diagnostics sink to use.
	Opts       *Options           // the options used for this compilation.
	Pkgs       symbols.PackageMap // all imported/bound packages.
	Currpkg    *symbols.Package   // the current package being compiled.
	Currmodule *symbols.Module    // the current module being compiled.
	Currclass  *symbols.Class     // the current class being compiled.
}

// NewContext creates a new context with the given state.
func NewContext(path string, d diag.Sink, opts *Options) *Context {
	return &Context{
		Path: path,
		Diag: d,
		Opts: opts,
		Pkgs: make(symbols.PackageMap),
	}
}
