package toolchain

import (
	"context"
	"errors"
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
	InstallDependencies(ctx context.Context, cwd string, showOutput bool, infoWriter, errorWriter io.Writer) error
	ValidateVenv(ctx context.Context) error
	ListPackages(ctx context.Context, transitive bool) ([]PythonPackage, error)
	Command(ctx context.Context, args ...string) (*exec.Cmd, error)
	About(ctx context.Context) (Info, error)
}

func Name(tc toolchain) string {
	switch tc {
	case Pip:
		return "Pip"
	default:
		return "Unknown"
	}
}

func ResolveToolchain(options PythonOptions) (Toolchain, error) {
	if options.Toolchain != Pip {
		return nil, errors.New("only pip toolchain is supported")
	}
	return newPip(options.Root, options.Virtualenv)
}

func virtualEnvBinDirName() string {
	if runtime.GOOS == windows {
		return "Scripts"
	}
	return "bin"
}
