// Copyright 2016-2017, Pulumi Corporation
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

// Package predef contains a set of tokens and/or symbols that are so-called "predefined"; they map to real abstractions
// defined elsewhere, like the Lumi standard library, but don't actually define them.
package predef

import (
	"github.com/pulumi/lumi/pkg/tokens"
)

var (
	// LumiStdlib is the predefined Lumi standard library.
	LumiStdlib = tokens.NewPackageToken("lumi")
	// LumiStdlibMainModule is the predefined Lumi standard library's main module.
	LumiStdlibMainModule = tokens.NewModuleToken(LumiStdlib, tokens.DefaultModule)
	// LumiStdlibResourceModule is the predefined Lumi standard library's resource module.
	LumiStdlibResourceModule = tokens.NewModuleToken(LumiStdlib, "resource")
	// LumiStdlibResourceClass is the penultimate resource base class, from which all resources derive.
	LumiStdlibResourceClass = tokens.NewTypeToken(LumiStdlibResourceModule, "Resource")
)
