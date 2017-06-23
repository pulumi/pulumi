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

package symbols

import (
	"fmt"

	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/tokens"
)

// Symbol is the base interface for all LumiIL symbol types.
type Symbol interface {
	Name() tokens.Name   // the simple name for this symbol.
	Token() tokens.Token // the unique qualified name token for this symbol.
	Special() bool       // indicates whether this is a "special" symbol; these are inaccessible to user code.
	Tree() diag.Diagable // the diagnosable tree associated with this symbol.
	String() string      // implement Stringer for easy formatting (e.g., in error messages).
}

var _ fmt.Stringer = (Symbol)(nil)
