// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package archive

type ignorer interface {
	IsIgnored(f string) bool
}

type ignoreState struct {
	ignorer ignorer
	next    *ignoreState
}

func (s *ignoreState) Append(ignorer ignorer) *ignoreState {
	return &ignoreState{ignorer: ignorer, next: s}
}

func (s *ignoreState) IsIgnored(path string) bool {
	if s == nil {
		return false
	}

	if s.ignorer.IsIgnored(path) {
		return true
	}

	return s.next.IsIgnored(path)
}
