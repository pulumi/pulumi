//+build !windows

package python

import "os"

func isReparsePoint(info os.FileInfo) bool {
	return false
}
