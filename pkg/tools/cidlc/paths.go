// Copyright 2017 Pulumi, Inc. All rights reserved.

package cidlc

import (
	"path/filepath"

	"golang.org/x/tools/go/loader"

	"github.com/pulumi/coconut/pkg/util/contract"
)

// RelFilename gets the target filename for any given position relative to the root.
func RelFilename(root string, prog *loader.Program, p goPos) string {
	pos := p.Pos()
	source := prog.Fset.Position(pos).Filename // the source filename.`
	rel, err := filepath.Rel(root, source)     // make it relative to the root.
	contract.Assert(err == nil)
	return rel
}
