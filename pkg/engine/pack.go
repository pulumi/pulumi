package engine

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi-fabric/pkg/encoding"
	"github.com/pulumi/pulumi-fabric/pkg/pack"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
	"github.com/pulumi/pulumi-fabric/pkg/workspace"
)

// detectPackage returns a package given the path, or returns an error if one could not be located.
func (eng *Engine) detectPackage(path string) (*pack.Package, error) {
	pkgpath, err := workspace.DetectPackage(path, eng.Diag())
	if err != nil {
		return nil, errors.Errorf("could not locate a package to load: %v", err)
	} else if pkgpath == "" {
		return nil, errors.Errorf("no package found at: %v", err)
	}
	pkg, _ := eng.readPackage(pkgpath)
	contract.Assert(pkg != nil)
	return pkg, nil
}

// readPackageFromArg reads a package from an argument value.  It can be "-" to request reading from Stdin, and is
// interpreted as a path otherwise.  If an error occurs, it is printed to Stderr, and the returned value will be nil.
// In addition to the package, a root directory is returned that the compiler should be formed over, if any.
func (eng *Engine) readPackageFromArg(arg string) (*pack.Package, string) {
	// If the arg is simply "-", read from stdin.
	if arg == "-" {
		return eng.readPackageFromStdin(), ""
	}

	// Read the package from a file.
	return eng.readPackage(arg)
}

// readPackageFromStdin attempts to read a package from Stdin; if an error occurs, it will be printed to Stderr, and
// the returned value will be nil.
func (eng *Engine) readPackageFromStdin() *pack.Package {
	// If stdin, read the package from text, and then create a compiler using the working directory.
	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(eng.Stderr, "error: could not read from stdin\n")
		fmt.Fprintf(eng.Stderr, "       %v\n", err)
		return nil
	}

	return eng.DecodePackage(encoding.Marshalers[".json"], b, "stdin")
}

// readPackage attempts to read a package from the given path; if an error occurs, it will be printed to Stderr, and
// the returned value will be nil.  If the path is a directory, nil is returned.
func (eng *Engine) readPackage(path string) (*pack.Package, string) {
	// If it's a directory, bail early.
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(eng.Stderr, "error: could not read path '%v': %v\n", path, err)
		return nil, ""
	}
	if info.IsDir() {
		return nil, path
	}

	// Lookup the marshaler for this format.
	ext := filepath.Ext(path)
	m, has := encoding.Marshalers[ext]
	if !has {
		fmt.Fprintf(eng.Stderr, "error: no marshaler found for file format '%v'\n", ext)
		return nil, ""
	}

	// Read the contents.
	b, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Fprintf(eng.Stderr, "error: a problem occurred when reading file '%v'\n", path)
		fmt.Fprintf(eng.Stderr, "       %v\n", err)
		return nil, ""
	}

	return eng.DecodePackage(m, b, path), filepath.Dir(path)
}

// DecodePackage turns a byte array into a package using the given marshaler.  If an error occurs, it is printed to
// Stderr, and the returned package value will be nil.
func (eng *Engine) DecodePackage(m encoding.Marshaler, b []byte, path string) *pack.Package {
	// Unmarshal the contents into a fresh package.
	pkg, err := encoding.Decode(m, b)
	if err != nil {
		fmt.Fprintf(eng.Stderr, "error: a problem occurred when unmarshaling file '%v'\n", path)
		fmt.Fprintf(eng.Stderr, "       %v\n", err)
		return nil
	}
	return pkg
}
