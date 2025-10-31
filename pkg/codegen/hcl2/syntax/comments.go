package syntax

import syntax "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/syntax"

// A TokenMap is used to map from syntax nodes to information about their tokens and leading whitespace/comments.
type TokenMap = syntax.TokenMap

// NewTokenMapForFiles creates a new token map that can be used to look up tokens for nodes in any of the given files.
func NewTokenMapForFiles(files []*File) TokenMap {
	return syntax.NewTokenMapForFiles(files)
}

