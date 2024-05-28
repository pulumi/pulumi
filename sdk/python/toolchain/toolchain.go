package toolchain

import (
	"context"
	"io"
	"os/exec"
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

type toolchain int

const (
	Pip toolchain = iota
	Poetry
)

type PythonOptions struct {
	// TODO: we could unify the two paths.
	// Virtual environment path to use. Must be an absolute path.
	Virtualenv string
	// The directory to use for poetry. Must be an absolute path.
	PoetryDirectory string
	// Use a typechecker to type check
	Typechecker typeChecker
	// The package manager to use for managing dependencies.
	Toolchain toolchain
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

type Toolchain interface {
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

func ResolveToolchain(options PythonOptions) (Toolchain, error) {
	if options.Toolchain == Poetry {
		return newPoetry(options.PoetryDirectory)
	}
	return newPip(options.Virtualenv)
}

func virtualEnvBinDirName() string {
	if runtime.GOOS == windows {
		return "Scripts"
	}
	return "bin"
}
