package npm

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
)

// NPM is the canonical "Node Package Manager".
type npm struct {
	executable string
}

// Assert that NPM is an instance of PackageManager.
var _ PackageManager = &npm{}

func newNPM() (*npm, error) {
	npmPath, err := exec.LookPath("npm")
	instance := &npm{
		executable: npmPath,
	}
	return instance, err
}

func (node *npm) Name() string {
	return "npm"
}

func (node *npm) Install(ctx context.Context, dir string, production bool, stdout, stderr io.Writer) error {
	command := node.installCmd(ctx, production)
	command.Dir = dir
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

func (node *npm) installCmd(ctx context.Context, production bool) *exec.Cmd {
	// We pass `--loglevel=error` to prevent `npm` from printing warnings about missing
	// `description`, `repository`, and `license` fields in the package.json file.
	args := []string{"install", "--loglevel=error"}

	if production {
		args = append(args, "--production")
	}

	//nolint:gosec // False positive on tained command execution. We aren't accepting input from the user here.
	return exec.CommandContext(ctx, node.executable, args...)
}

func (node *npm) Pack(ctx context.Context, dir string, stderr io.Writer) ([]byte, error) {
	//nolint:gosec // False positive on tained command execution. We aren't accepting input from the user here.
	command := exec.CommandContext(ctx, node.executable, "pack", "--loglevel=error")
	command.Dir = dir

	// We have to read the name of the file from stdout.
	var stdout bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = stderr
	err := command.Run()
	if err != nil {
		return nil, err
	}
	// Next, we try to read the name of the file from stdout.
	// packfile is the name of the file containing the tarball,
	// as produced by `npm pack`.
	packfile := strings.TrimSpace(stdout.String())
	defer os.Remove(packfile)

	packTarball, err := os.ReadFile(packfile)
	if err != nil {
		newErr := errors.New("`npm pack` completed successfully but the package .tgz file was not generated")
		return nil, newErr
	}

	return packTarball, nil
}
