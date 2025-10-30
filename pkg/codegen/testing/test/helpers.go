package test

import test "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/testing/test"

// GenPkgSignature corresponds to the shape of the codegen GeneratePackage functions.
type GenPkgSignature = test.GenPkgSignature

type SchemaVersion = test.SchemaVersion

// Schemas are downloaded in the makefile, and the versions specified here
// should be in sync with the makefile.
const AwsSchema = test.AwsSchema

// Schemas are downloaded in the makefile, and the versions specified here
// should be in sync with the makefile.
const AzureNativeSchema = test.AzureNativeSchema

// Schemas are downloaded in the makefile, and the versions specified here
// should be in sync with the makefile.
const AzureSchema = test.AzureSchema

// Schemas are downloaded in the makefile, and the versions specified here
// should be in sync with the makefile.
const KubernetesSchema = test.KubernetesSchema

// Schemas are downloaded in the makefile, and the versions specified here
// should be in sync with the makefile.
const RandomSchema = test.RandomSchema

// Schemas are downloaded in the makefile, and the versions specified here
// should be in sync with the makefile.
const EksSchema = test.EksSchema

// Schemas are downloaded in the makefile, and the versions specified here
// should be in sync with the makefile.
const AwsStaticWebsiteSchema = test.AwsStaticWebsiteSchema

// Schemas are downloaded in the makefile, and the versions specified here
// should be in sync with the makefile.
const AwsNativeSchema = test.AwsNativeSchema

// PulumiDotnetSDKVersion is the version of the Pulumi .NET SDK to use in program-gen tests
const PulumiDotnetSDKVersion = test.PulumiDotnetSDKVersion

// GeneratePackageFilesFromSchema loads a schema and generates files using the provided GeneratePackage function.
func GeneratePackageFilesFromSchema(schemaPath string, genPackageFunc GenPkgSignature) (map[string][]byte, error) {
	return test.GeneratePackageFilesFromSchema(schemaPath, genPackageFunc)
}

// LoadFiles loads the provided list of files from a directory.
func LoadFiles(dir, lang string, files []string) (map[string][]byte, error) {
	return test.LoadFiles(dir, lang, files)
}

func PathExists(path string) (bool, error) {
	return test.PathExists(path)
}

// `LoadBaseline` loads the contents of the given baseline directory,
// by inspecting its `codegen-manifest.json`.
func LoadBaseline(dir, lang string) (map[string][]byte, error) {
	return test.LoadBaseline(dir, lang)
}

// ValidateFileEquality compares maps of files for equality.
func ValidateFileEquality(t *testing.T, actual, expected map[string][]byte) bool {
	return test.ValidateFileEquality(t, actual, expected)
}

// If PULUMI_ACCEPT is set, writes out actual output to the expected
// file set, so we can continue enjoying golden tests without manually
// modifying the expected output.
func RewriteFilesWhenPulumiAccept(t *testing.T, dir, lang string, actual map[string][]byte) bool {
	return test.RewriteFilesWhenPulumiAccept(t, dir, lang, actual)
}

// Useful for populating code-generated destination
// `codeDir=$dir/$lang` with extra manually written files such as the
// unit test files. These files are copied from `$dir/$lang-extras`
// folder if present.
func CopyExtraFiles(t *testing.T, dir, lang string) {
	test.CopyExtraFiles(t, dir, lang)
}

// CheckAllFilesGenerated ensures that the set of expected and actual files generated
// are exactly equivalent.
func CheckAllFilesGenerated(t *testing.T, actual, expected map[string][]byte) {
	test.CheckAllFilesGenerated(t, actual, expected)
}

// Validates a transformer on a single file.
func ValidateFileTransformer(t *testing.T, inputFile string, expectedOutputFile string, transformer func(io.Reader, io.Writer) error) {
	test.ValidateFileTransformer(t, inputFile, expectedOutputFile, transformer)
}

func RunCommand(t *testing.T, name string, cwd string, exec string, args ...string) {
	test.RunCommand(t, name, cwd, exec, args...)
}

func RunCommandWithOptions(t *testing.T, opts *integration.ProgramTestOptions, name string, cwd string, exec string, args ...string) {
	test.RunCommandWithOptions(t, opts, name, cwd, exec, args...)
}

