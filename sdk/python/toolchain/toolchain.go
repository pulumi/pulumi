package toolchain

import (
	"context"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
)

type typeChecker int

const (
	// TypeCheckerNone is the default typeChecker
	TypeCheckerNone typeChecker = iota
	// TypeCheckerMypy is the mypy typeChecker
	TypeCheckerMypy
	// TypeCheckerPyright is the pyright typeChecker
	TypeCheckerPyright
)

type packagemanager int

const (
	PackageManagerPip packagemanager = iota
	PackageManagerPoetry
)

type PythonOptions struct {
	// Virtual environment path to use.
	Virtualenv string
	// // The resolved virtual environment path.
	// // TODO: why do we have both here?
	// VirtualenvPath string
	// Use a typechecker to type check
	Typechecker typeChecker
	// The package manager to use for managing dependencies.
	PackageManager packagemanager
}

type PythonPackage struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Location string `json:"location"`
}

type Info struct {
	Executable string
	Version    string
}

// TODO: rename to Toolchain
type PackageManager interface {
	// TODO: rename to InstallDependencies
	InstallDependenciesWithWriters(ctx context.Context,
		root string, showOutput bool, infoWriter, errorWriter io.Writer,
	) error
	// TODO: should this take root?
	ListPackages(ctx context.Context, transitive bool) ([]PythonPackage, error)
	Command(ctx context.Context, args ...string) (*exec.Cmd, error)
	// TODO: something to get the python executable path and version for use in About

	About(ctx context.Context) (Info, error)

	// TODO: something to install the venv? setup ?
	// TODO: is this needed? looks like we tend to just call InstallDependenciesWithWriters
	Setup(ctx context.Context) error
}

func ResolveToolchain(root string, options PythonOptions) (PackageManager, error) {
	if options.PackageManager == PackageManagerPoetry {
		return newPoetry()
	}
	return newPip(root, options.Virtualenv)
}

func resolveVirtualEnvironmentPath(root, virtualenv string) string {
	if virtualenv == "" {
		return ""
	}
	if !filepath.IsAbs(virtualenv) {
		return filepath.Join(root, virtualenv)
	}
	return virtualenv
}

func virtualEnvBinDirName() string {
	if runtime.GOOS == windows {
		return "Scripts"
	}
	return "bin"
}
