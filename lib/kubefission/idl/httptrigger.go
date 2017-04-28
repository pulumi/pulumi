// Copyright 2017 Pulumi, Inc. All rights reserved.

package kubefission

import (
	"github.com/pulumi/coconut/pkg/resource/idl"
)

// HTTPTrigger maps URL patterns to functions.  Function.UID is optional; if absent, the latest version of the function
// will automatically be selected.
type HTTPTrigger struct {
	idl.NamedResource
	URLPattern string    `coco:"urlPattern"`
	Method     string    `coco:"method"`
	Function   *Function `coco:"function"`
}
