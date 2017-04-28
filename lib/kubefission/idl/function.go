// Copyright 2017 Pulumi, Inc. All rights reserved.

package kubefission

import (
	"github.com/pulumi/coconut/pkg/resource/idl"
)

// Function is a unit of executable code.  Though it's called a function, the code may have more than one function;
// it's usually some sort of module or package.
type Function struct {
	idl.NamedResource
	Environment *Environment `coco:"environment"`
	Code        *idl.Asset   `coco:"code"`
}
