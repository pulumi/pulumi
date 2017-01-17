// Copyright 2016 Marapongo, Inc. All rights reserved.

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/marapongo/mu/pkg/encoding"
	"github.com/marapongo/mu/pkg/pack"
	decode "github.com/marapongo/mu/pkg/pack/encoding"
)

// readPackage attempts to read a package from the given path; if an error occurs, it will be printed to Stderr, and
// the returned value will be nil.
func readPackage(path string) *pack.Package {
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

	return decodePackage(m, b, path)
}

// readPackageFromArg reads a package from an argument value.  It can be "-" to request reading from Stdin, and is
// interpreted as a path otherwise.  If an error occurs, it is printed to Stderr, and the returned value will be nil.
func readPackageFromArg(arg string) *pack.Package {
	if arg == "-" {
		// Read the package from stdin.
		return readPackageFromStdin()
	} else {
		// Read the package from a file.
		return readPackage(arg)
	}

}

// readPackageFromStdin attempts to read a package from Stdin; if an error occurs, it will be printed to Stderr, and
// the returned value will be nil.
func readPackageFromStdin() *pack.Package {
	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not read from stdin\n")
		fmt.Fprintf(os.Stderr, "       %v\n", err)
		return nil
	}

	return decodePackage(encoding.Marshalers[".json"], b, "stdin")
}

// decodePackage turns a byte array into a package using the given marshaler.  If an error occurs, it is printed to
// Stderr, and the returned package value will be nil.
func decodePackage(m encoding.Marshaler, b []byte, path string) *pack.Package {
	// Unmarshal the contents into a fresh package.
	pkg, err := decode.Decode(m, b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: a problem occurred when unmarshaling file '%v'\n", path)
		fmt.Fprintf(os.Stderr, "       %v\n", err)
		return nil
	}
	return pkg
}
