package npm

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
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

// This error occurs when the user has selected two package managers; only one can be selected at a time.
var errMutuallyExclusiveEnvVars = fmt.Errorf(
	"both PULUMI_PREFER_YARN and PULUMI_PREFER_PNPM are set; these env vars are mutually exclusive",
)

// ResolvePackageManager determines which package manager to use.
// First, we check if a package manager is explicitly selected via env var.
// Next, we look for the presence of a yarn lockfile and use YarnClassic if one is found.
// If there's no yarn lockfile, we look for a pnpm lockfile.
// If there's no pnpm lockfile, we default to Node.
// The argument pwd is the present working directory we're checking for the presence
// of a lockfile.
func ResolvePackageManager(pwd string) (PackageManager, error) {
	yarnEnvSet := preferYarn()
	pnpmEnvSet := preferPNPM()
	hasYarnLock := checkYarnLock(pwd)
	hasPNPMLock := checkPNPMLock(pwd)

	// • Error if both PULUMI_PREFER_PNPM and PULUMI_PREFER_YARN are set.
	if yarnEnvSet && pnpmEnvSet {
		return nil, errMutuallyExclusiveEnvVars
	}

	// • Now, check if either of these variables are set, since they take
	//   higher precedence than lockfiles.
	if yarnEnvSet {
		// • Try to use Yarn. If we fail, we can still try to use PNPM or NPM.
		yarn, err := newYarnClassic()
		// If we can't find the Yarn executable, so next we'll check for a PNPM lockfile.
		if err == nil {
			return yarn, nil
		}
		logging.Warningf("could not find yarn on the $PATH, falling back to pnpm or npm: %v", err)
		// • If the user has a pnpm lockfile, we try to load PNPM, falling back to NPM.
		if hasPNPMLock {
			return loadPNPMOrFallback()
		}
		// • …otherwise, we just use NPM.
		return newNPM()
	}

	// This block is the same behavior as above, except we start with PNPM,
	// then fallback to Yarn if there's a Yarn lockfile.
	if pnpmEnvSet {
		// • Now we try to find the PNPM executable, just like with Yarn.
		pnpmManager, err := newPNPM()
		if err == nil {
			return pnpmManager, nil
		}
		logging.Warningf("could not find pnpm on the $PATH, falling back to yarn or npm: %v", err)
		if hasYarnLock {
			return loadYarnClassicOrFallback()
		}
		// • …otherwise, we just use NPM.
		return newNPM()
	}

	// • By this point, we know that the user hasn't explicitly selected a package
	//   manager with an environment variable. We use the lockfiles as hints for
	//   which they prefer.
	// Case 1: No lockfiles present. Default to NPM.
	if !hasYarnLock && !hasPNPMLock {
		return newNPM()
	}

	// Case 2: Yarnlock found, no PNPM lock.
	if hasYarnLock && !hasPNPMLock {
		return loadYarnClassicOrFallback()
	}

	// Case 3: pnpm lock found, no yarn lock.
	if !hasYarnLock && hasPNPMLock {
		return loadPNPMOrFallback()
	}

	// TODO: These warning logs are inconsistent. i.e. they're not
	//       executed if we call `loadOrFallback`

	// Case 4: both lockfiles found.
	// Prefer Yarn, fallback to PNPM, fallback to NPM.
	// Even if there's also an PNPM lockfile, we prefer Yarn
	// for backward compatibility (since PNPM support was added to Pulumi later).
	yarn, err := newYarnClassic()
	if err == nil {
		return yarn, nil
	}
	logging.Warningf(
		"found lockfiles for PNPM and Yarn, but could not find yarn on the $PATH, trying pnpm instead: %v",
		err,
	)
	var manager PackageManager
	manager, err = newPNPM()
	if err != nil {
		logging.Warningf("could not find pnpm on the $PATH either, falling back to npm instead: %v", err)
		manager, err = newNPM()
	}

	return manager, err
}

// loadYarnClassicOrFallback attempts to load YarnClassic, falling back to NPM
// if yarn isn't on the $PATH.
func loadYarnClassicOrFallback() (PackageManager, error) {
	manager, err := newYarnClassic()
	if err != nil {
		return newNPM()
	}
	return manager, err
}

// loadPNPMOrFallback attempts to load PNPM, falling back to NPM
// if PNPM isn't on the $PATH.
func loadPNPMOrFallback() (PackageManager, error) {
	manager, err := newPNPM()
	if err != nil {
		return newNPM()
	}
	return manager, err
}

// preferYarn returns true if the `PULUMI_PREFER_YARN` environment variable is set.
func preferYarn() bool {
	return env.PreferYarn.Value()
}

// preferYarn returns true if the `PULUMI_PREFER_PNPM` environment variable is set.
func preferPNPM() bool {
	return env.PreferPNPM.Value()
}
