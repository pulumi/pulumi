package npm

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
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
func Pack(ctx context.Context, dir string, stderr io.Writer) ([]byte, error) {
	pkgManager, err := ResolvePackageManager(dir)
	if err != nil {
		return nil, err
	}
	return pkgManager.Pack(ctx, dir, stderr)
}

// Install runs `npm install` in the given directory, installing the dependencies for the Node.js
// app located there. If the `PULUMI_PREFER_YARN` environment variable is set, `yarn install` is used
// instead of `npm install`.
// The returned string is the name of the package manager used during installation.
func Install(ctx context.Context, dir string, production bool, stdout, stderr io.Writer) (string, error) {
	pkgManager, err := ResolvePackageManager(dir)
	if err != nil {
		return "", err
	}
	name := pkgManager.Name()

	err = pkgManager.Install(ctx, dir, production, stdout, stderr)
	if err != nil {
		return name, err
	}

	// Ensure the "node_modules" directory exists.
	// NB: This is only approperate for certain package managers.
	//     Yarn with Plug'n'Play enabled won't produce a node_modules directory,
	//     either for Yarn Classic or Yarn Berry.
	nodeModulesPath := filepath.Join(dir, "node_modules")
	if _, err := os.Stat(nodeModulesPath); err != nil {
		if os.IsNotExist(err) {
			return name, fmt.Errorf("%s install reported success, but node_modules directory is missing", name)
		}
		// If the node_modules dir exists but we can't stat it, we might be able to proceed
		// without issue, but it's bizarre enough that we should warn.
		logging.Warningf("failed to read node_modules metadata: %v", err)
	}

	return name, nil
}

// ResolvePackageManager determines which package manager to use.
// It inspects the value of "PULUMI_PREFER_YARN" and checks for a yarn.lock file.
// If neither of those values are enabled or truthy, then it uses NPM over YarnClassic.
// The argument pwd is the present working directory we're checking for the presence
// of a lockfile.
func ResolvePackageManager(pwd string) (PackageManager, error) {
	// Prefer yarn if PULUMI_PREFER_YARN is truthy, or if yarn.lock exists.
	if preferYarn() || checkYarnLock(pwd) {
		yarn, err := newYarnClassic()
		// If we can't find the Yarn executable, then we should default to NPM.
		if err == nil {
			return yarn, nil
		}
		logging.Warningf("could not find yarn on the $PATH, trying npm instead: %v", err)
	}

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
