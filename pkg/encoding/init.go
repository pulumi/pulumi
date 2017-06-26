// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package encoding

import (
	"github.com/pulumi/lumi/pkg/util/contract"
)

func init() {
	// Ensure a marshaler is available for every possible Lumifile extension
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
			contract.Failf("No Marshaler available for LumifileExt %v", ext)
		}
	}
}
