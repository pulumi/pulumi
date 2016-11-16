// Copyright 2016 Marapongo, Inc. All rights reserved.

package workspace

// Muspace is a file that contains optional settings about a workspace, and delimits its boundaries.
var Muspace = ".muspace"

// Mumodules is where dependency modules exist, either local to a workspace, or globally on a machine.
var Mumodules = ".mu_modules"

// MufileBase is the base name of a Mufile.
const MufileBase = "Mu"

// MufileExts contains a list of all the valid Mufile extensions.
var MufileExts = []string{
	".json",
	".yaml",
	// Although ".yml" is not a sanctioned YAML extension, it is used quite broadly; so we will support it.
	".yml",
}
