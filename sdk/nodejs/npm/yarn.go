package npm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	uuid "github.com/gofrs/uuid"
)

// yarnClassic is an implementation of PackageManager that uses Yarn Classic,
// which is the v1.x.y line, to install dependencies.
type yarnClassic struct {
	executable string
}

// Assert that YarnClassic is an instance of PackageManager.
var _ PackageManager = &yarnClassic{}

func newYarnClassic() (*yarnClassic, error) {
	yarnPath, err := exec.LookPath("yarn")
	yarn := &yarnClassic{
		executable: yarnPath,
	}
	return yarn, err
}

func (node *yarnClassic) Name() string {
	return "yarn"
}

func (yarn *yarnClassic) Install(ctx context.Context, dir string, production bool, stdout, stderr io.Writer) error {
	command := yarn.installCmd(ctx, production)
	command.Dir = dir
	command.Stdout = stdout
	return yarn.runCmd(command, stderr)
}

// Generates the installation command for a given installation of YarnClassic.
func (yarn *yarnClassic) installCmd(ctx context.Context, production bool) *exec.Cmd {
	args := []string{"install"}

	if production {
		args = append(args, "--production")
	}

	//nolint:gosec // False positive on tained command execution. We aren't accepting input from the user here.
	return exec.CommandContext(ctx, yarn.executable, args...)
}

func (yarn *yarnClassic) runCmd(command *exec.Cmd, stderr io.Writer) error {
	// `stderr` is ignored when `yarn` is used because it outputs warnings like "package.json: No license field"
	// to `stderr` that we don't need to show.
	var stderrBuffer bytes.Buffer
	command.Stderr = &stderrBuffer

	err := command.Run()
	// If we failed, and we're using `yarn`, write out any bytes that were written to `stderr`.
	if err != nil {
		stderr.Write(stderrBuffer.Bytes())
	}
	return err
}

// Pack runs `yarn pack` in the given directory, packaging the Node.js app located
// there into a tarball an returning it as `[]byte`. `stdout` is ignored for this command,
// as it does not produce useful data.
func (yarn *yarnClassic) Pack(ctx context.Context, dir string, stderr io.Writer) ([]byte, error) {
	// Note that `npm pack` doesn't have the ability to specify the resulting filename, since
	// it's meant to be uploaded directly to npm, which means we have to get that information
	// by parsing the output of the command. However, if we're using `yarn`, we can specify a
	// filename.
	// Since we're planning to read the name of the output from stdout, we create
	// a substitute buffer to intercept it.

	// generate a new uuid, to be used in the naming of the file.
	// TODO: We should probably prefer a os.tmpfile instead of recreating
	// the OS's functionality.
	uuid, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	packfile := fmt.Sprintf("%s.tgz", uuid.String())

	//nolint:gosec // False positive on tained command execution. We aren't accepting input from the user here.
	command := exec.CommandContext(ctx, yarn.executable, "pack", "--filename", packfile)
	command.Dir = dir

	err = yarn.runCmd(command, stderr)
	if err != nil {
		return nil, err
	}

	// Clean up the tarball after we've read it.
	defer os.Remove(packfile)
	// Read the tarball in as a byte slice.
	return os.ReadFile(packfile)
}

// checkYarnLock checks if there's a file named yarn.lock in pwd.
// This function is used to indicate whether to prefer Yarn over
// other package managers.
func checkYarnLock(pwd string) bool {
	yarnFile := filepath.Join(pwd, "yarn.lock")
	_, err := os.Stat(yarnFile)
	return err == nil
}