package nodejs

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	uuid "github.com/satori/go.uuid"

	"github.com/pkg/errors"
)

type yarn1 struct {
	path string
}

func (r yarn1) Install(dir string, stdout, stderr io.Writer) (string, error) {
	c := exec.Command(r.path, "install")
	c.Dir = dir

	// Run the command.
	if err := runCmd(c, false, stdout, stderr); err != nil {
		return "yarn", err
	}

	// Ensure the "node_modules" directory exists.
	// TODO check correct installation via `yarn why`
	nodeModulesPath := filepath.Join(dir, "node_modules")
	if _, err := os.Stat(nodeModulesPath); os.IsNotExist(err) {
		return "yarn", errors.Errorf("yarn install reported success, but node_modules directory is missing")
	}

	return "yarn", nil
}

func (r yarn1) Pack(dir string, stderr io.Writer) ([]byte, error) {
	c := exec.Command(r.path, "pack")
	c.Dir = dir

	// `yarn pack` has the ability to specify the resulting filename
	var packfile string
	packfile = fmt.Sprintf("%s.tgz", uuid.NewV4().String())
	c.Args = append(c.Args, "--filename", packfile)

	// Run the command.
	// `stdout` is ignored for the command, as it does not generate useful data.
	var stdout bytes.Buffer
	if err := runCmd(c, false, &stdout, stderr); err != nil {
		return nil, err
	}

	defer os.Remove(packfile)

	packTarball, err := ioutil.ReadFile(packfile)
	if err != nil {
		return nil, fmt.Errorf("yarn pack completed successfully but the packed .tgz file was not generated")
	}

	return packTarball, nil
}

func (r yarn1) Run(dir string, stdout, stderr io.Writer) (string, error) {
	return "", nil
}
