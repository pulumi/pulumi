package codegen

import codegen "github.com/pulumi/pulumi/sdk/v3/pkg/codegen"

type StringSet = codegen.StringSet

type Set = codegen.Set

// A simple in memory file system.
type Fs = codegen.Fs

func NewStringSet(values ...string) StringSet {
	return codegen.NewStringSet(values...)
}

// CleanDir removes all existing files from a directory except those in the exclusions list.
// Note: The exclusions currently don't function recursively, so you cannot exclude a single file
// in a subdirectory, only entire subdirectories. This function will need improvements to be able to
// target that use-case.
func CleanDir(dirPath string, exclusions StringSet) error {
	return codegen.CleanDir(dirPath, exclusions)
}

func ExpandShortEnumName(name string) string {
	return codegen.ExpandShortEnumName(name)
}

// Check if two packages are the same.
func PkgEquals(p1, p2 schema.PackageReference) bool {
	return codegen.PkgEquals(p1, p2)
}

