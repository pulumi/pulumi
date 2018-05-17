// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package workspace

// Settings defines workspace settings shared amongst many related projects.
// nolint: lll
type Settings struct {
	Stack string `json:"stack,omitempty" yaml:"env,omitempty"` // an optional default stack to use.
}

// IsEmpty returns true when the settings object is logically empty (no selected stack and nothing in the deprecated
// configuration bag).
func (s *Settings) IsEmpty() bool {
	return s.Stack == ""
}
