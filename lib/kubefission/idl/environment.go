// Copyright 2017 Pulumi, Inc. All rights reserved.

package kubefission

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// Environment identifies the language and OS specific resources that a function depends on.  For now this includes
// only the function run container image.  Later, this will also include build containers, as well as support tools
// like debuggers, profilers, etc.
type Environment struct {
	idl.NamedResource
	RunContainerImageURL string `lumi:"runContainerImageURL"`
}
