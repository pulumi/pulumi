package afero

import afero "github.com/pulumi/pulumi/sdk/v3/pkg/util/afero"

// Copies all file and directories from src to dst that match the filter
func CopyDir(fs afero.Fs, src, dst string, filter func(os.FileInfo) bool) error {
	return afero.CopyDir(fs, src, dst, filter)
}

// Copies a file from src to dst
func Copy(fs afero.Fs, src, dst string) error {
	return afero.Copy(fs, src, dst)
}

