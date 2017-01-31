// Copyright 2016 Marapongo, Inc. All rights reserved.

package eval

import (
	"fmt"

	"github.com/marapongo/mu/pkg/util/contract"
)

// Pointer is a slot that can be used for indirection purposes (since Go maps are not stable).
type Pointer struct {
	obj      *Object // the object to which the value refers.
	readonly bool    // true prevents writes to this slot (by abandoning).
}

var _ fmt.Stringer = (*Pointer)(nil)

func (ptr *Pointer) Readonly() bool { return ptr.readonly }
func (ptr *Pointer) Obj() *Object   { return ptr.obj }

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
