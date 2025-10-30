package pcl

import pcl "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/pcl"

// ComponentProgramBinderFromFileSystem returns the default component program binder which uses the file system
// to parse and bind PCL files into a program.
func ComponentProgramBinderFromFileSystem() ComponentProgramBinder {
	return pcl.ComponentProgramBinderFromFileSystem()
}

