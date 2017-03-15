// Copyright 2017 Pulumi, Inc. All rights reserved.

package cmdutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pulumi/coconut/pkg/encoding"
	"github.com/pulumi/coconut/pkg/pack"
)

// ReadPackage attempts to read a package from the given path; if an error occurs, it will be printed to Stderr, and
// the returned value will be nil.
func ReadPackage(path string) *pack.Package {
	// Lookup the marshaler for this format.
	ext := filepath.Ext(path)
	m, has := encoding.Marshalers[ext]
	if !has {
		fmt.Fprintf(os.Stderr, "error: no marshaler found for file format '%v'\n", ext)
		return nil
	}

	// Read the contents.
	b, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: a problem occurred when reading file '%v'\n", path)
		fmt.Fprintf(os.Stderr, "       %v\n", err)
		return nil
	}

	return DecodePackage(m, b, path)
}

// ReadPackageFromArg reads a package from an argument value.  It can be "-" to request reading from Stdin, and is
// interpreted as a path otherwise.  If an error occurs, it is printed to Stderr, and the returned value will be nil.
func ReadPackageFromArg(arg string) *pack.Package {
	// If the arg is simply "-", read from stdin.
	if arg == "-" {
		return ReadPackageFromStdin()
	}

	// Read the package from a file.
	return ReadPackage(arg)
}

// ReadPackageFromStdin attempts to read a package from Stdin; if an error occurs, it will be printed to Stderr, and
// the returned value will be nil.
func ReadPackageFromStdin() *pack.Package {
	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not read from stdin\n")
		fmt.Fprintf(os.Stderr, "       %v\n", err)
		return nil
	}

	return DecodePackage(encoding.Marshalers[".json"], b, "stdin")
}

// DecodePackage turns a byte array into a package using the given marshaler.  If an error occurs, it is printed to
// Stderr, and the returned package value will be nil.
func DecodePackage(m encoding.Marshaler, b []byte, path string) *pack.Package {
	// Unmarshal the contents into a fresh package.
	pkg, err := encoding.Decode(m, b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: a problem occurred when unmarshaling file '%v'\n", path)
		fmt.Fprintf(os.Stderr, "       %v\n", err)
		return nil
	}
	return pkg
}
