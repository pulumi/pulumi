//+build !windows

package python

import "os"

// Check if file supports reparse point in windows.
// This is to trigger a workaround for https://github.com/golang/go/issues/42919
func isReparsePoint(info os.FileInfo) bool {
	return false
}
