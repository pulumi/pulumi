//+build !windows

package python

// This is to trigger a workaround for https://github.com/golang/go/issues/42919
func needsPythonShim(pythonPath string) bool {
	return false
}
