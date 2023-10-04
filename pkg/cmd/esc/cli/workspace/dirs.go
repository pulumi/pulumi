// Copyright 2023, Pulumi Corporation. All rights reserved.

package workspace

const bookkeepingDir = ".esc"

// getBookkeepingDir returns the path to the esc CLI's bookkeeping directory.
func (w *Workspace) getBookkeepingDir() (string, error) {
	return w.pulumi.GetPulumiPath(bookkeepingDir)
}
