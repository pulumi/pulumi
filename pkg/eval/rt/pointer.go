// Copyright 2017 Pulumi, Inc. All rights reserved.

package rt

import (
	"fmt"

	"github.com/pulumi/coconut/pkg/compiler/symbols"
	"github.com/pulumi/coconut/pkg/util/contract"
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
	if ptr.obj == nil {
		return prefix + "{<nil>}"
	}
	return prefix + "{" + ptr.obj.String() + "}"
}
