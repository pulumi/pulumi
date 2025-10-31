package newcmd

import newcmd "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/newcmd"

// Ensure the directory exists and uses it as the current working
// directory.
func UseSpecifiedDir(dir string) (string, error) {
	return newcmd.UseSpecifiedDir(dir)
}

// ErrorIfNotEmptyDirectory returns an error if path is not empty.
func ErrorIfNotEmptyDirectory(path string) error {
	return newcmd.ErrorIfNotEmptyDirectory(path)
}

