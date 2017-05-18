// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmdutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pulumi/lumi/pkg/encoding"
	"github.com/pulumi/lumi/pkg/pack"
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
