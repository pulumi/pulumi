// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package encoding

import (
	"github.com/pulumi/pulumi/pkg/util/contract"
)

func init() {
	// Ensure a marshaler is available for every possible metadata extension.
	Marshalers = make(map[string]Marshaler)
	for _, ext := range Exts {
		switch ext {
		case ".json":
			Marshalers[ext] = JSON
		case ".yml":
			fallthrough
		case ".yaml":
			Marshalers[ext] = YAML
		default:
			contract.Failf("No marshaler available for extension '%s'", ext)
		}
	}
}
