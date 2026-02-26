// Copyright 2023-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package npm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

type PackageManagerType string

const (
	// AutoPackageManager automatically choses a packagemanager by looking for environment variables and lockfiles.
	AutoPackageManager PackageManagerType = "auto"
	NpmPackageManager  PackageManagerType = "npm"
	YarnPackageManager PackageManagerType = "yarn"
	PnpmPackageManager PackageManagerType = "pnpm"
	BunPackageManager  PackageManagerType = "bun"
)

// PackageDependency represents a single package dependency with its name and resolved version.
type PackageDependency struct {
	Name    string
	Version string
}

// A `PackageManager` is responsible for installing dependencies,
// packaging Pulumi programs, and executing Node in the context of
// installed packages. In practice, each implementation of this
// interface represents one of the package managers in the Node ecosystem
// e.g. Yarn, NPM, etc.
// The language host will dynamically dispatch to an instance of PackageManager
// in response to RPC requests.
type PackageManager interface {
	// Install will install dependencies with the given package manager.
	Install(ctx context.Context, dir string, production bool, stdout, stderr io.Writer) error
	// Link adds the package `packageName` which can be found at `path` to the package.json found in `dir`.
	Link(ctx context.Context, dir, packageName, path string) error
	Pack(ctx context.Context, dir string, stderr io.Writer) ([]byte, error)
	// Name is the name of the binary executable used to invoke this package manager.
	// e.g. yarn or npm
	Name() string
	// Version returns the version of the package manager.
	Version() (semver.Version, error)
	// ListPackages returns the list of packages and their versions in the given directory.
	// Each package manager uses its own mechanism to determine the packages (e.g. by querying
	// the package manager CLI or by reading lock/manifest files).
	ListPackages(ctx context.Context, dir string) ([]PackageDependency, error)
}

// Pack runs `npm pack` in the given directory, packaging the Node.js app located there into a
// tarball and returning it as `[]byte`. `stdout` is ignored for the command, as it does not
// generate useful data. If the `PULUMI_PREFER_YARN` environment variable is set, `yarn pack` is run
// instead of `npm pack`.
func Pack(ctx context.Context, packagemanager PackageManagerType, dir string, stderr io.Writer) ([]byte, error) {
	pkgManager, err := ResolvePackageManager(packagemanager, dir)
	if err != nil {
		return nil, err
	}
	return pkgManager.Pack(ctx, dir, stderr)
}

// Install runs `npm install` in the given directory, installing the dependencies for the Node.js
// app located there. If the `PULUMI_PREFER_YARN` environment variable is set, `yarn install` is used
// instead of `npm install`.
// The returned string is the name of the package manager used during installation.
func Install(ctx context.Context, packagemanager PackageManagerType, dir string, production bool,
	stdout, stderr io.Writer,
) (string, error) {
	pkgManager, err := ResolvePackageManager(packagemanager, dir)
	if err != nil {
		return "", err
	}
	name := pkgManager.Name()

	err = pkgManager.Install(ctx, dir, production, stdout, stderr)
	if err != nil {
		return name, err
	}

	// Ensure the "node_modules" directory exists.
	nodeModulesPath, err := searchup(dir, "node_modules")
	if nodeModulesPath == "" {
		if err != nil {
			return name, fmt.Errorf("error while looking for 'node_modules': %w", err)
		}
		return name, fmt.Errorf("%s install reported success, but node_modules directory is missing", name)
	}

	return name, nil
}

// ResolvePackageManager determines which package manager to use.
//
// If the packagemanager argument is set, and it is not `AutoPackageManager` then, that package
// manager is used. Otherwise, if the `PULUMI_PREFER_YARN` environment variable is set, or if
// a yarn.lock file exists, then YarnClassic is used. If a pnpm-lock.yaml file exists, then
// pnpm is used. If either bun.lockb or bun.lock (for newer versions of bun) then bun is used.
// Otherwise npm is used. The argument pwd is the directory  we're checking for
// the presence of a lockfile.
func ResolvePackageManager(packagemanager PackageManagerType, pwd string) (PackageManager, error) {
	// If a package manager is explicitly specified, use it.
	if packagemanager != "" && packagemanager != AutoPackageManager {
		switch packagemanager {
		case AutoPackageManager:
			// Make the linter for exhaustive switch cases happy, we never get here.
			break
		case NpmPackageManager:
			return newNPM()
		case YarnPackageManager:
			return newYarnClassic()
		case PnpmPackageManager:
			return newPnpm()
		case BunPackageManager:
			return newBun()
		default:
			return nil, fmt.Errorf("unknown package manager: %s", packagemanager)
		}
	}

	// No packagemanager specified, try to determine the best one to use.

	// Prefer yarn if PULUMI_PREFER_YARN is truthy, or if yarn.lock exists.
	if preferYarn() || checkYarnLock(pwd) {
		yarn, err := newYarnClassic()
		// If we can't find the Yarn executable, then we should default to NPM.
		if err == nil {
			return yarn, nil
		}
		logging.Warningf("could not find yarn on the $PATH, trying pnpm instead: %v", err)
	}

	// Prefer pnpm if pnpm-lock.yaml exists.
	if checkPnpmLock(pwd) {
		pnpm, err := newPnpm()
		if err == nil {
			return pnpm, nil
		}
		logging.Warningf("could not find pnpm on the $PATH, trying bun instead: %v", err)
	}

	// Prefer bun if bun.lock (bun >= v1.2) or bun.lockb (bun < 1.2) exists
	if checkBunLock(pwd) {
		bun, err := newBun()
		if err == nil {
			return bun, nil
		}
		logging.Warningf("could not find bun on the $PATH, trying npm instead: %v", err)
	}

	// Finally, fall back to npm.
	node, err := newNPM()
	if err != nil {
		return nil, fmt.Errorf("could not find npm on the $PATH; npm is installed with Node.js "+
			"available at https://nodejs.org/: %w", err)
	}

	return node, nil
}

// preferYarn returns true if the `PULUMI_PREFER_YARN` environment variable is set.
func preferYarn() bool {
	return cmdutil.IsTruthy(os.Getenv("PULUMI_PREFER_YARN"))
}

// ResolvePackageManagerForListing resolves the appropriate package manager for listing packages
// in the given directory. Unlike ResolvePackageManager, this function does not require the
// package manager executable to be installed. For package managers without dedicated lock file
// parsers (pnpm, bun), it creates a lightweight manager that reads from package.json.
func ResolvePackageManagerForListing(pwd string) PackageManager {
	if preferYarn() || checkYarnLock(pwd) {
		yarn, err := newYarnClassic()
		if err == nil {
			return yarn
		}
	}
	if checkPnpmLock(pwd) {
		return &pnpmManager{}
	}
	if checkBunLock(pwd) {
		return &bunManager{}
	}
	// Default to npm if available, otherwise use a stub that reads package.json.
	node, err := newNPM()
	if err == nil {
		return node
	}
	return &npmManager{}
}

// getLinkPackageProperty returns a string to use in `npm pkg set` to add the package to package.json dependencies.
func getLinkPackageProperty(packageName, path string) string {
	return fmt.Sprintf("dependencies.%s=file:%s", packageName, path)
}

// packageJSONFile represents the structure of a package.json file, used for reading dependency information.
type packageJSONFile struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// listPackagesFromPackageJSON reads dependency information directly from package.json in the given
// directory. This is used as a fallback for package managers that don't yet have dedicated lock
// file parsers. It returns version ranges rather than pinned versions.
func listPackagesFromPackageJSON(dir string) ([]PackageDependency, error) {
	packageFile := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(packageFile)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", packageFile, err)
	}
	var body packageJSONFile
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("could not parse %s: %w", packageFile, err)
	}
	var result []PackageDependency
	for name, version := range body.Dependencies {
		result = append(result, PackageDependency{
			Name:    name,
			Version: version,
		})
	}
	for name, version := range body.DevDependencies {
		result = append(result, PackageDependency{
			Name:    name,
			Version: version,
		})
	}
	return result, nil
}
