// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/pack"
	"github.com/pulumi/pulumi/pkg/workspace"
)

type Pkginfo struct {
	Pkg  *pack.Package
	Root string
}

// GetPwdMain returns the working directory and main entrypoint to use for this package.
func (pkginfo *Pkginfo) GetPwdMain() (string, string, error) {
	pwd := pkginfo.Root
	main := pkginfo.Pkg.Main
	if main != "" {
		// The path must be relative from the package root.
		if filepath.IsAbs(main) {
			return "", "", errors.New("project 'main' must be a relative path")
		}

		// Check that main is a *subdirectory* from the root.
		cleanPwd := filepath.Clean(pwd)
		main := filepath.Clean(path.Join(cleanPwd, main))
		if !strings.HasPrefix(main, cleanPwd) {
			return "", "", errors.New("project 'main' must be a subfolder")
		}

		// So that any relative paths inside of the program are correct, we still need to pass the pwd
		// of the main program's parent directory.  How we do this depends on if the target is a dir or not.
		maininfo, err := os.Stat(main)
		if err != nil {
			return "", "", errors.Wrapf(err, "project 'main' could not be read")
		}
		if maininfo.IsDir() {
			pwd = main
			main = ""
		} else {
			pwd = filepath.Dir(main)
			main = filepath.Base(main)
		}
	}
	return pwd, main, nil
}

// ReadPackageFromArg reads a package from an argument value.  It can be "-" to request reading from Stdin, and is
// interpreted as a path otherwise.  If an error occurs, it is printed to Stderr, and the returned value will be nil.
// In addition to the package, a root directory is returned that the compiler should be formed over, if any.
func ReadPackageFromArg(arg string) (*Pkginfo, error) {
	// If the arg is "-", read from stdin.
	if arg == "-" {
		return ReadPackageFromStdin()
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
	return ReadPackage(path)
}

// ReadPackageFromStdin attempts to read a package from Stdin; if an error occurs, it will be printed to Stderr, and
// the returned value will be nil.
func ReadPackageFromStdin() (*Pkginfo, error) {
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

	return &Pkginfo{
		Pkg:  &pkg,
		Root: "",
	}, nil
}

// ReadPackage attempts to read a package from the given path; if an error occurs, it will be printed to Stderr, and
// the returned value will be nil.  If the path is a directory, nil is returned.
func ReadPackage(path string) (*Pkginfo, error) {
	// If the path refers to a directory, bail early.
	info, err := os.Stat(path)
	if err != nil {
		return nil, errors.Wrapf(err, "could not read path '%v'", path)
	} else if info.IsDir() {
		return nil, errors.Errorf("path '%v' is a directory and not a path to package file", path)
	}

	pkg, err := pack.Load(path)
	if err != nil {
		return nil, err
	}

	return &Pkginfo{
		Pkg:  pkg,
		Root: filepath.Dir(path),
	}, nil
}
