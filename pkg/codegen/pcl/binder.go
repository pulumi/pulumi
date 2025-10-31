package pcl

import pcl "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/pcl"

type ComponentProgramBinderArgs = pcl.ComponentProgramBinderArgs

type ComponentProgramBinder = pcl.ComponentProgramBinder

type BindOption = pcl.BindOption

const LogicalNamePropertyKey = pcl.LogicalNamePropertyKey

func AllowMissingVariables(options *bindOptions) {
	pcl.AllowMissingVariables(options)
}

func AllowMissingProperties(options *bindOptions) {
	pcl.AllowMissingProperties(options)
}

func SkipResourceTypechecking(options *bindOptions) {
	pcl.SkipResourceTypechecking(options)
}

func SkipRangeTypechecking(options *bindOptions) {
	pcl.SkipRangeTypechecking(options)
}

func PreferOutputVersionedInvokes(options *bindOptions) {
	pcl.PreferOutputVersionedInvokes(options)
}

func SkipInvokeTypechecking(options *bindOptions) {
	pcl.SkipInvokeTypechecking(options)
}

func PluginHost(host plugin.Host) BindOption {
	return pcl.PluginHost(host)
}

func Loader(loader schema.Loader) BindOption {
	return pcl.Loader(loader)
}

func Cache(cache *PackageCache) BindOption {
	return pcl.Cache(cache)
}

func DirPath(path string) BindOption {
	return pcl.DirPath(path)
}

func ComponentBinder(binder ComponentProgramBinder) BindOption {
	return pcl.ComponentBinder(binder)
}

// NonStrictBindOptions returns a set of bind options that make the binder lenient about type checking.
// Changing errors into warnings when possible
func NonStrictBindOptions() []BindOption {
	return pcl.NonStrictBindOptions()
}

// BindProgram performs semantic analysis on the given set of HCL2 files that represent a single program. The given
// host, if any, is used for loading any resource plugins necessary to extract schema information.
func BindProgram(files []*syntax.File, opts ...BindOption) (*Program, hcl.Diagnostics, error) {
	return pcl.BindProgram(files, opts...)
}

// Used by language plugins to bind a PCL program in the given directory.
func BindDirectory(directory string, loader schema.ReferenceLoader, extraOptions ...BindOption) (*Program, hcl.Diagnostics, error) {
	return pcl.BindDirectory(directory, loader, extraOptions...)
}

func ParseFiles(parser *syntax.Parser, directory string, files []fs.DirEntry) (hcl.Diagnostics, error) {
	return pcl.ParseFiles(parser, directory, files)
}

// / ParseDirectory parses all of the PCL files in the given directory into the state of the parser.
func ParseDirectory(parser *syntax.Parser, directory string) (hcl.Diagnostics, error) {
	return pcl.ParseDirectory(parser, directory)
}

func ReadAllPackageDescriptors(files []*syntax.File) (map[string]*schema.PackageDescriptor, hcl.Diagnostics) {
	return pcl.ReadAllPackageDescriptors(files)
}

// ReadPackageDescriptors reads the package blocks in the given file and returns a map of package names to the package.
// The descriptors blocks are top-level program blocks have the following format:
// 
// 	package <name> {
// 	  baseProviderName = <name>
// 	  baseProviderVersion = <version>
// 	  baseProviderDownloadUrl = <url>
// 	  parameterization {
// 	      name = <name>
// 	      version = <version>
// 	      value = <base64 encoded string>
// 	  }
// 	}
// 
// Notes:
//   - parameterization block is optional.
//   - the value of the parameterization is base64-encoded string because we want to specify binary data in PCL form
// 
// These package descriptors allow the binder loader to load package schema information from a package source.
func ReadPackageDescriptors(file *syntax.File) (map[string]*schema.PackageDescriptor, hcl.Diagnostics) {
	return pcl.ReadPackageDescriptors(file)
}

