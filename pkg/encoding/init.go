// Copyright 2016 Marapongo, Inc. All rights reserved.

package encoding

import (
	"github.com/marapongo/mu/pkg/util"
)

func init() {
	// Ensure a marshaler is available for every possible Mufile extension
	Marshalers = make(map[string]Marshaler)
	for _, ext := range Exts {
		switch ext {
		case ".json":
			Marshalers[ext] = &jsonMarshaler{}
		case ".yml":
			fallthrough
		case ".yaml":
			Marshalers[ext] = &yamlMarshaler{}
		default:
			util.FailMF("No Marshaler available for MufileExt %v", ext)
		}
	}
}
