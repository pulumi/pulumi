// Copyright 2016 Pulumi, Inc. All rights reserved.

package encoding

import (
	"github.com/pulumi/coconut/pkg/util/contract"
)

func init() {
	// Ensure a marshaler is available for every possible Nutfile extension
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
			contract.Failf("No Marshaler available for NutfileExt %v", ext)
		}
	}
}
