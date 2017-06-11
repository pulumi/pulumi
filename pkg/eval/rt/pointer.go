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

package rt

import (
	"fmt"

	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// Pointer is a slot that can be used for indirection purposes (since Go maps are not stable).
type Pointer struct {
	obj      *Object          // the object to which the value refers.
	readonly bool             // true prevents writes to this slot (by abandoning).
	get      symbols.Function // an optional custom getter function.
	set      symbols.Function // an optional custom setter function.
}

var _ fmt.Stringer = (*Pointer)(nil)

func NewPointer(obj *Object, readonly bool, get symbols.Function, set symbols.Function) *Pointer {
	return &Pointer{
		obj:      obj,
		readonly: readonly,
		get:      get,
		set:      set,
	}
}

func (ptr *Pointer) Readonly() bool           { return ptr.readonly }
func (ptr *Pointer) Getter() symbols.Function { return ptr.get }
func (ptr *Pointer) Setter() symbols.Function { return ptr.set }
func (ptr *Pointer) Obj() *Object             { return ptr.obj }
func (ptr *Pointer) Freeze()                  { ptr.readonly = true }

// Set sets the underlying value of the object.
func (ptr *Pointer) Set(obj *Object) {
	contract.Assertf(!ptr.readonly, "Unexpected write to readonly reference")
	ptr.obj = obj
}

func (ptr *Pointer) String() string {
	var prefix string
	if ptr.readonly {
		prefix = "&"
	} else {
		prefix = "*"
	}
	if ptr.get != nil {
		return prefix + "{<not invoking getter>}"
	}
	if ptr.obj == nil {
		return prefix + "{<nil>}"
	}
	return prefix + "{" + ptr.obj.String() + "}"
}
