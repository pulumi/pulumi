// Copyright 2017 Pulumi, Inc. All rights reserved.

package kubefission

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// HTTPTrigger maps URL patterns to functions.  Function.UID is optional; if absent, the latest version of the function
// will automatically be selected.
type HTTPTrigger struct {
	idl.NamedResource
	URLPattern string    `lumi:"urlPattern"`
	Method     string    `lumi:"method"`
	Function   *Function `lumi:"function"`
}
