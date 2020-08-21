package auto

import (
	"bytes"
	"os"
	"os/exec"
)

func runPulumiCommandSync(workdir string, additionalEnv []string, args ...string) (string, string, int, error) {
	// all commands should be run in non-interactive mode.
	// this causes commands to fail rather than prompting for input (and thus hanging indefinitely)
	args = append(args, "--non-interactive")
	cmd := exec.Command("pulumi", args...)
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(), additionalEnv...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	code := -2 // unknown
	err := cmd.Run()
	if exitError, ok := err.(*exec.ExitError); ok {
		code = exitError.ExitCode()
	}
	return stdout.String(), stderr.String(), code, err
}
