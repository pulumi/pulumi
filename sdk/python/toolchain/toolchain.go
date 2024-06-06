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
	// The root directory of the project.
	Root string
	// Virtual environment to use, relative to `Root`.
	Virtualenv string
	// Use a typechecker to type check.
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
	// InstallDependencies installs the dependencies of the project found in `cwd`.
	InstallDependencies(ctx context.Context, cwd string, showOutput bool, infoWriter, errorWriter io.Writer) error
	// EnsureVenv validates virtual environment of the toolchain and creates it if it doesn't exist.
	EnsureVenv(ctx context.Context, cwd string, showOutput bool, infoWriter, errorWriter io.Writer) error
	// ValidateVenv checks if the virtual environment of the toolchain is valid.
	ValidateVenv(ctx context.Context) error
	// ListPackages returns a list of Python packages installed in the toolchain.
	ListPackages(ctx context.Context, transitive bool) ([]PythonPackage, error)
	// Command returns an *exec.Cmd for running `python` using the configured toolchain.
	Command(ctx context.Context, args ...string) (*exec.Cmd, error)
	// ModuleCommand returns an *exec.Cmd for running an installed python module using the configured toolchain.
	// https://docs.python.org/3/using/cmdline.html#cmdoption-m
	ModuleCommand(ctx context.Context, module string, args ...string) (*exec.Cmd, error)
	// About returns information about the python executable of the toolchain.
	About(ctx context.Context) (Info, error)
}

func Name(tc toolchain) string {
	switch tc {
	case Pip:
		return "Pip"
	case Poetry:
		return "Poetry"
	default:
		return "Unknown"
	}
}

func ResolveToolchain(options PythonOptions) (Toolchain, error) {
	if options.Toolchain == Poetry {
		return newPoetry(options.Root)
	}
	return newPip(options.Root, options.Virtualenv)
}

func virtualEnvBinDirName() string {
	if runtime.GOOS == windows {
		return "Scripts"
	}
	return "bin"
}
