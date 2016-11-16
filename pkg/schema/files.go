// Copyright 2016 Marapongo, Inc. All rights reserved.

package schema

// MufileBase is the base name of the Mufile.
const MufileBase = "Mu"

// MufileExts is a map of extension to a Marshaler object for that extension.
var MufileExts = map[string]Marshaler{
	".json": &jsonMarshaler{},
	".yaml": &yamlMarshaler{},
	// Although ".yml" is not a sanctioned YAML extension, it is used quite broadly; so we will support it.
	".yml": &yamlMarshaler{},
}
