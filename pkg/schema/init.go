// Copyright 2016 Marapongo, Inc. All rights reserved.

package schema

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/workspace"
)

func init() {
	// Ensure a marshaler is available for every possible Mufile extension
	Marshalers = make(map[string]Marshaler)
	for _, ext := range workspace.MufileExts {
		switch ext {
		case ".json":
			Marshalers[ext] = &jsonMarshaler{}
		case ".yml":
			fallthrough
		case ".yaml":
			Marshalers[ext] = &yamlMarshaler{}
		default:
			glog.Fatalf("No Marshaler available for MufileExt %v", ext)
		}
	}
}
