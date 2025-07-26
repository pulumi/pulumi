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
	"fmt"
	"io"
	"os"

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
	Pack(ctx context.Context, dir string, stderr io.Writer) ([]byte, error)
	// Name is the name of the binary executable used to invoke this package manager.
	// e.g. yarn or npm
	Name() string
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
