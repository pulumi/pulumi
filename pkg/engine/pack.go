// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/pack"
	"github.com/pulumi/pulumi/pkg/workspace"
)

type pkginfo struct {
	Pkg  *pack.Package
	Root string
}

// readPackageFromArg reads a package from an argument value.  It can be "-" to request reading from Stdin, and is
// interpreted as a path otherwise.  If an error occurs, it is printed to Stderr, and the returned value will be nil.
// In addition to the package, a root directory is returned that the compiler should be formed over, if any.
func (eng *Engine) readPackageFromArg(arg string) (*pkginfo, error) {
	// If the arg is "-", read from stdin.
	if arg == "-" {
		return eng.readPackageFromStdin()
	}

	// If the path is empty, we need to detect it based on the current working directory.
	var path string
	if arg == "" {
		pwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		path = pwd
		// Now that we got here, we have a path, so we will try to load it.
		pkgpath, err := workspace.DetectPackage(path)
		if err != nil {
			return nil, errors.Errorf("could not locate a package to load: %v", err)
		} else if pkgpath == "" {
			return nil, errors.Errorf("no package found by searching upwards from '%v'", path)
		}
		path = pkgpath
	} else {
		path = arg
	}

	// Finally, go ahead and load the package directly from the path that we ended up with.
	return eng.readPackage(path)
}

// readPackageFromStdin attempts to read a package from Stdin; if an error occurs, it will be printed to Stderr, and
// the returned value will be nil.
func (eng *Engine) readPackageFromStdin() (*pkginfo, error) {
	// If stdin, read the package from text, and then create a compiler using the working directory.
	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return nil, errors.Wrapf(err, "could not read package from stdin")
	}

	m := encoding.Marshalers[".json"]
	var pkg pack.Package
	err = m.Unmarshal(b, &pkg)
	if err != nil {
		return nil, errors.Wrap(err, "a problem occurred when unmarshaling stdin into a package")
	}
	if err = pkg.Validate(); err != nil {
		return nil, err
	}

	return &pkginfo{
		Pkg:  &pkg,
		Root: "",
	}, nil
}

// readPackage attempts to read a package from the given path; if an error occurs, it will be printed to Stderr, and
// the returned value will be nil.  If the path is a directory, nil is returned.
func (eng *Engine) readPackage(path string) (*pkginfo, error) {
	// If the path refers to a directory, bail early.
	info, err := os.Stat(path)
	if err != nil {
		return nil, errors.Wrapf(err, "could not read path '%v'", path)
	} else if info.IsDir() {
		return nil, errors.Wrapf(err, "path '%v' is a directory and not a path to package file", path)
	}

	pkg, err := pack.Load(path)
	if err != nil {
		return nil, err
	}

	return &pkginfo{
		Pkg:  pkg,
		Root: filepath.Dir(path),
	}, nil
}
