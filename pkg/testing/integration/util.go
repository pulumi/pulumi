package integration

import integration "github.com/pulumi/pulumi/sdk/v3/pkg/testing/integration"

// DecodeMapString takes a string of the form key1=value1:key2=value2 and returns a go map.
func DecodeMapString(val string) (map[string]string, error) {
	return integration.DecodeMapString(val)
}

// ReplaceInFile does a find and replace for a given string within a file.
func ReplaceInFile(old, new, path string) error {
	return integration.ReplaceInFile(old, new, path)
}

// CopyFile copies a single file from src to dst
// From https://blog.depado.eu/post/copy-files-and-directories-in-go
func CopyFile(src, dst string) error {
	return integration.CopyFile(src, dst)
}

// CopyDir copies a whole directory recursively
// From https://blog.depado.eu/post/copy-files-and-directories-in-go
func CopyDir(src, dst string) error {
	return integration.CopyDir(src, dst)
}

// AssertHTTPResultWithRetry attempts to assert that an HTTP endpoint exists
// and evaluate its response.
func AssertHTTPResultWithRetry(t *testing.T, output any, headers map[string]string, maxWait time.Duration, check func(string) bool) bool {
	return integration.AssertHTTPResultWithRetry(t, output, headers, maxWait, check)
}

func CheckRuntimeOptions(t *testing.T, root string, expected map[string]any) {
	integration.CheckRuntimeOptions(t, root, expected)
}

