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

// Package predef contains a set of tokens and/or symbols that are so-called "predefined"; they map to real abstractions
// defined elsewhere, like the Lumi standard library, but don't actually define them.
package predef

import (
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/compiler/types"
)

// IsResourceType returns true if the given type symbol represents the standard resource class.
func IsResourceType(t symbols.Type) bool {
	return types.HasBaseName(t, LumiStdlibResourceClass)
}

// IsLatentResourceProperty returns true if the given type symbol t represents the standard resource class and the
// property type symbol pt represents a latent property of that resource type.  A latent property is one whose value may
// not be known during certain phases like planning; the interpreter attempts to proceed despite this.
func IsLatentResourceProperty(t symbols.Type, pt symbols.Type) bool {
	if IsResourceType(t) {
		if _, isfunc := pt.(*symbols.FunctionType); !isfunc {
			return true
		}
	}
	return false
}
