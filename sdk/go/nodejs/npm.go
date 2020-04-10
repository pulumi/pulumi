package nodejs

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

type npm struct {
	path string
	args string
}

func (r npm) Install(dir string, stdout, stderr io.Writer) (string, error) {
	c := exec.Command(r.path, "install", r.args)
	c.Dir = dir
	npm := true

	// Run the command.
	if err := runCmd(c, npm, stdout, stderr); err != nil {
		return "npm", err
	}

	// Ensure the "node_modules" directory exists.
	// TODO check correct installation via `npm list`
	nodeModulesPath := filepath.Join(dir, "node_modules")
	if _, err := os.Stat(nodeModulesPath); os.IsNotExist(err) {
		return "npm", errors.Errorf("npm install reported success, but node_modules directory is missing")
	}

	return "npm", nil
}

func (r npm) Pack(dir string, stderr io.Writer) ([]byte, error) {
	c := exec.Command(r.path, "pack", r.args)
	c.Dir = dir
	npm := true

	// Run the command.
	// `stdout` is ignored for the command, as it does not generate useful data.
	var stdout bytes.Buffer
	if err := runCmd(c, npm, &stdout, stderr); err != nil {
		return nil, err
	}

	packfile := strings.TrimSpace(stdout.String())

	defer os.Remove(packfile)

	packTarball, err := ioutil.ReadFile(packfile)
	if err != nil {
		return nil, fmt.Errorf("npm pack completed successfully but the packed .tgz file was not generated")
	}

	return packTarball, nil
}

func (r npm) Run(dir string, stdout, stderr io.Writer) (string, error) {
	return "", nil
}
