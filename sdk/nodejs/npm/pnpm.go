package npm

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// pnpm is an implementation of PackageManager that uses PNPM,
// to install dependencies.
type pnpm struct {
	executable string
}

// Assert that YarnClassic is an instance of PackageManager.
var _ PackageManager = &pnpm{}

func newPNPM() (*pnpm, error) {
	path, err := exec.LookPath("pnpm")
	manager := &pnpm{
		executable: path,
	}
	return manager, err
}

func (manager *pnpm) Name() string {
	return "pnpm"
}

func (manager *pnpm) Install(ctx context.Context, dir string, production bool, stdout, stderr io.Writer) error {
	command := manager.installCmd(ctx, production)
	command.Dir = dir
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

// Generates the command to install packages with PNPM.
func (manager *pnpm) installCmd(ctx context.Context, production bool) *exec.Cmd {
	args := []string{"install"}

	if production {
		args = append(args, "--prod")
	}

	//nolint:gosec // False positive on tained command execution. We aren't accepting input from the user here.
	return exec.CommandContext(ctx, manager.executable, args...)
}

// Pack runs `pnpm pack` in the given directory, packaging the Node.js app located
// there into a tarball an returning it as `[]byte`. `stdout` is ignored for this command,
// as it does not produce useful data.
func (manager *pnpm) Pack(ctx context.Context, dir string, stderr io.Writer) ([]byte, error) {
	panic("TODO")
}

// checkPNPMLock checks if there's a file named pnpm-lock.yaml in pwd.
// This function is used to indicate whether to prefer PNPM over
// other package managers.
func checkPNPMLock(pwd string) bool {
	yarnFile := filepath.Join(pwd, "pnpm-lock.yaml")
	_, err := os.Stat(yarnFile)
	return err == nil
}
